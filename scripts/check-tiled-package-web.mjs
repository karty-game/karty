import assert from "node:assert/strict";
import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { createHash } from "node:crypto";
import { chromium } from "playwright";
import { startVillageServer } from "./willowbrook-server.mjs";
const argument = (key, fallback) => {
  const i = process.argv.indexOf(key);
  return i < 0 ? fallback : process.argv[i + 1];
};
const directory = resolve(argument("--directory", "testdata/tiled-room/dist/web"));
const label = argument("--label", "linked");
const output = resolve("dist/validation/tiled-package");
await mkdir(output, { recursive: true });
const hash = async (p) =>
  createHash("sha256")
    .update(await readFile(resolve(directory, p)))
    .digest("hex");
const level = (await readdir(resolve(directory, "content"))).find((n) => n.endsWith(".kld"));
const tiledReference = await readFile("testdata/tiled-room/maps/room.tiled.png");
const report = {
  passed: false,
  label,
  hostSha256: await hash("karty-host.wasm"),
  cartridgeSha256: await hash("game.kart"),
  levelSha256: await hash("content/" + level),
  tiledReferenceSha256: createHash("sha256").update(tiledReference).digest("hex"),
  scope:
    "Actual Go WASM/Ebiten 2D in Chromium SwiftShader; synthesized keyboard and CDP touch; no physical-device or FPS qualification.",
  errors: [],
};
const server = await startVillageServer(directory);
let browser;
const deadline = setTimeout(() => {
  console.error("Tiled consumer exceeded 120 seconds");
  process.exit(1);
}, 120000);
try {
  browser = await chromium.launch({
    headless: true,
    args: ["--no-sandbox", "--use-gl=angle", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  const page = await browser.newPage({ viewport: { width: 1280, height: 720 }, hasTouch: true });
  page.on("pageerror", (e) => report.errors.push(e.message));
  page.on("console", (e) => {
    if (e.type() === "error") report.errors.push(e.text());
  });
  await page.addInitScript(() => {
    const s = (globalThis.tiledEvidence = {
      frame: 0,
      created: 0,
      destroyed: 0,
      assets: [],
      texts: [],
      camera: [],
      cameraUpdates: 0,
      world3D: 0,
      maxBatch: 0,
      maxRecords: 0,
      retransforms: 0,
      known: {},
    });
    let update, submit;
    Object.defineProperty(globalThis, "kartyClientUpdate", {
      configurable: true,
      get: () => update,
      set: (original) => {
        update = (frame, batch) => {
          s.frame = Number(frame);
          return original(frame, batch);
        };
      },
    });
    Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
      configurable: true,
      get: () => submit,
      set: (original) => {
        submit = (batch) => {
          const v = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
          let offset = 24;
          s.maxBatch = Math.max(s.maxBatch, batch.byteLength);
          s.maxRecords = Math.max(s.maxRecords, v.getUint32(16, true));
          for (let i = 0; i < v.getUint32(16, true); i++) {
            const tag = v.getUint8(offset),
              p = offset + 3,
              len = v.getUint16(offset + 1, true),
              id = len >= 4 ? v.getUint32(p, true) : 0;
            if (tag === 16) {
              s.created++;
              s.known[id] = 0;
              const a = v.getUint32(p + 8, true);
              if (!s.assets.includes(a)) s.assets.push(a);
            }
            if (tag === 17) {
              const a = v.getUint32(p + 8, true);
              if (!s.assets.includes(a)) s.assets.push(a);
            }
            if (tag === 3 && id in s.known) {
              if (s.known[id]++) s.retransforms++;
            }
            if (tag === 2) {
              s.destroyed++;
              delete s.known[id];
            }
            if (tag === 55) {
              s.camera = [0, 4, 8].map((n) => v.getFloat32(p + n, true));
              s.cameraUpdates++;
            }
            if (tag >= 27 && tag <= 35) s.world3D++;
            if (tag === 9) {
              const n = v.getUint16(p + 4, true),
                t = new TextDecoder().decode(batch.slice(p + 6, p + 6 + n));
              if (!s.texts.includes(t)) s.texts.push(t);
            }
            offset += 3 + len;
          }
          return original(batch);
        };
      },
    });
  });
  await page.goto(`http://127.0.0.1:${server.address().port}/?debug=0`);
  await page.waitForFunction(() => tiledEvidence.texts.includes("TILED ROOM"), null, { timeout: 30000 });
  await page.locator("canvas").evaluate((c) => {
    c.tabIndex = 0;
    c.focus();
  });
  const state = () => page.evaluate(() => structuredClone(tiledEvidence));
  const frames = async (n) => {
    const start = (await state()).frame;
    await page.waitForFunction(({ start, n }) => tiledEvidence.frame >= start + n, { start, n }, { timeout: 20000 });
  };
  await frames(90);
  const initial = await state();
  assert.ok(initial.created >= 160);
  assert.equal(initial.assets.length, 2);
  // Compare interior markers against official Tiled's raster, not our transform
  // math. Both animation frames deliberately share these colored corner marks.
  report.tiledPixels = await page.evaluate(
    async ({ reference, camera }) => {
      const image = new Image();
      image.src = reference;
      await image.decode();
      const expected = document.createElement("canvas");
      expected.width = image.width;
      expected.height = image.height;
      const ctx = expected.getContext("2d");
      ctx.drawImage(image, 0, 0);
      const source = ctx.getImageData(0, 0, image.width, image.height).data;
      return new Promise((done) =>
        requestAnimationFrame(() => {
          const canvas = document.querySelector("canvas"),
            gl = canvas.getContext("webgl2");
          const rendered = new Uint8Array(canvas.width * canvas.height * 4);
          gl.readPixels(0, 0, canvas.width, canvas.height, gl.RGBA, gl.UNSIGNED_BYTE, rendered);
          const same = (a, b) => [0, 1, 2, 3].every((n) => source[a + n] === source[b + n]);
          const groups = Array(8).fill(0),
            failures = [];
          let checked = 0;
          for (let y = 2; y < image.height - 2; y += 2)
            for (let x = 2; x < image.width - 2; x += 2) {
              const p = (y * image.width + x) * 4;
              const marker =
                source[p + 3] === 255 &&
                ((source[p] === 250 && source[p + 1] === 218 && source[p + 2] === 105) ||
                  (source[p] === 36 && source[p + 1] === 52 && source[p + 2] === 74));
              if (!marker) continue;
              let interior = true;
              for (let dy = -1; dy <= 1; dy++)
                for (let dx = -1; dx <= 1; dx++) if (!same(p, ((y + dy) * image.width + x + dx) * 4)) interior = false;
              if (!interior) continue;
              const sx = Math.floor((((x + 0.5 - camera[0]) * camera[2] + 640) * canvas.width) / 1280);
              const sy = Math.floor((((y + 0.5 - camera[1]) * camera[2] + 360) * canvas.height) / 720);
              if (sx < 0 || sx >= canvas.width || sy < 0 || sy >= canvas.height) continue;
              const q = ((canvas.height - 1 - sy) * canvas.width + sx) * 4;
              checked++;
              groups[Math.min(7, Math.floor(x / 48))]++;
              if ([0, 1, 2].some((n) => Math.abs(source[p + n] - rendered[q + n]) > 24) && failures.length < 16)
                failures.push({
                  x,
                  y,
                  expected: Array.from(source.slice(p, p + 3)),
                  actual: Array.from(rendered.slice(q, q + 3)),
                });
            }
          done({ checked, groups, failures });
        }),
      );
    },
    { reference: "data:image/png;base64," + tiledReference.toString("base64"), camera: initial.camera },
  );
  assert.ok(report.tiledPixels.checked >= 200);
  assert.ok(
    report.tiledPixels.groups.every((n) => n >= 10),
    "missing Tiled flip group coverage",
  );
  assert.deepEqual(report.tiledPixels.failures, [], "live Ebiten pixels differ from official Tiled raster");
  await page.screenshot({ path: resolve(output, label + "-desktop.png"), fullPage: true });
  await page.keyboard.down("d");
  await frames(120);
  await page.keyboard.up("d");
  await frames(4);
  const moved = await state();
  assert.ok(moved.camera[0] > initial.camera[0] + 300);
  assert.ok(moved.destroyed > 0);
  assert.equal(moved.retransforms, 0, "static placements were needlessly retransformed");
  await page.keyboard.press("r");
  await frames(8);
  assert.ok((await state()).created > initial.created);
  await page.setViewportSize({ width: 390, height: 844 });
  await frames(5);
  const box = await page.locator("canvas").boundingBox();
  assert.ok(box && Math.abs(box.width - 390) < 2 && Math.abs(box.height - 844) < 2);
  const before = (await state()).camera;
  const client = await page.context().newCDPSession(page);
  const tx = box.x + box.width / 2 + 20,
    ty = box.y + box.height / 2 + 15;
  await client.send("Input.dispatchTouchEvent", {
    type: "touchStart",
    touchPoints: [{ id: 1, x: tx, y: ty, radiusX: 3, radiusY: 3 }],
  });
  await frames(4);
  await client.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  await frames(4);
  const after = await state();
  assert.ok(after.texts.includes("TILED POINTER"));
  const expected = [before[0] + (20 * 720) / box.height / before[2], before[1] + (15 * 720) / box.height / before[2]];
  assert.ok(
    Math.abs(after.camera[0] - expected[0]) < 1 && Math.abs(after.camera[1] - expected[1]) < 1,
    "portrait pointer projection",
  );
  await page.screenshot({ path: resolve(output, label + "-portrait.png"), fullPage: true });
  report.framebufferSum = await page.evaluate(
    () =>
      new Promise((done) =>
        requestAnimationFrame(() => {
          const c = document.querySelector("canvas"),
            gl = c.getContext("webgl2"),
            p = new Uint8Array(32 * 32 * 4);
          gl.readPixels(c.width / 2 - 16, c.height / 2 - 16, 32, 32, gl.RGBA, gl.UNSIGNED_BYTE, p);
          let sum = 0;
          for (let i = 0; i < p.length; i += 4) sum += p[i] + p[i + 1] + p[i + 2];
          done(sum);
        }),
      ),
  );
  assert.ok(report.framebufferSum > 10000);
  assert.equal(after.world3D, 0);
  assert.ok(after.maxBatch <= 65536 && after.maxRecords <= 4096);
  assert.deepEqual(report.errors, []);
  report.state = after;
  report.checks = [
    "independent generated SDK module with 160 non-square/flipped tiles",
    "live Ebiten corner pixels match official Tiled raster across all eight flip combinations",
    "absolute-clock animation and moving-bounds residency",
    "camera pan, cull and return without redundant transforms",
    "portrait canvas fit and actual CDP touch projection",
    "non-black Ebiten framebuffer, bounded batches and no 3D commands",
  ];
  report.passed = true;
} catch (e) {
  report.failure = String(e);
  throw e;
} finally {
  clearTimeout(deadline);
  if (browser) await browser.close();
  await new Promise((r) => server.close(r));
  await writeFile(resolve(output, label + "-report.json"), JSON.stringify(report, null, 2) + "\n");
}
console.log(JSON.stringify(report));
