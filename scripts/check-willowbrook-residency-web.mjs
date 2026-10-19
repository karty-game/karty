// Exercise the click sequence that exhausted SDK entity references while culling.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";
import { startVillageServer } from "./willowbrook-server.mjs";

const directory = resolve(process.env.KARTY_WILLOWBROOK_WEB || "samples/willowbrook/dist/web");
const output = resolve(process.env.KARTY_WILLOWBROOK_OUTPUT || "dist/validation/willowbrook-residency");
await mkdir(output, { recursive: true });
const digest = async (name) =>
  createHash("sha256")
    .update(await readFile(resolve(directory, name)))
    .digest("hex");
const report = {
  passed: false,
  scope: "Actual WASM/WebGL2 with SwiftShader and synthesized mouse input; no device FPS qualification.",
  cartridgeSha256: await digest("game.kart"),
  hostSha256: await digest("karty-host.wasm"),
  cases: [],
};
const server = await startVillageServer(directory);
try {
  for (const viewport of [
    { width: 1280, height: 720 },
    { width: 660, height: 765 },
  ]) {
    const deadline = setTimeout(() => {
      console.error("Willowbrook residency fixture exceeded 120s");
      process.exit(1);
    }, 120000);
    const browser = await chromium.launch({
      headless: true,
      args: ["--no-sandbox", "--use-gl=angle", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
    });
    const evidence = { viewport, errors: [], clicks: [] };
    report.cases.push(evidence);
    try {
      const page = await browser.newPage({ viewport, hasTouch: true });
      page.on("pageerror", (error) => evidence.errors.push(error.message));
      page.on("console", (message) => {
        if (message.type() === "error" || /panic:|FIRST REJECT/.test(message.text()))
          evidence.errors.push(message.text());
      });
      await page.addInitScript(() => {
        const state = (globalThis.residencyEvidence = {
          frame: 0,
          ready: false,
          live: 0,
          peak: 0,
          created: 0,
          destroyed: 0,
          cameraUpdates: 0,
          maxRecords: 0,
          maxBytes: 0,
        });
        const live = new Set();
        let update, submit;
        Object.defineProperty(globalThis, "kartyClientUpdate", {
          configurable: true,
          get: () => update,
          set(original) {
            update = (frame, batch) => {
              state.frame = Number(frame);
              return original(frame, batch);
            };
          },
        });
        Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
          configurable: true,
          get: () => submit,
          set(original) {
            submit = (batch) => {
              const view = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
              const count = view.getUint32(16, true);
              state.maxRecords = Math.max(state.maxRecords, count);
              state.maxBytes = Math.max(state.maxBytes, batch.byteLength);
              let offset = 24;
              for (let index = 0; index < count; index++) {
                const tag = view.getUint8(offset),
                  length = view.getUint16(offset + 1, true),
                  payload = offset + 3;
                const id = length >= 4 ? view.getUint32(payload, true) : 0;
                if ([1, 8, 10, 16].includes(tag)) {
                  live.add(id);
                  state.created++;
                  state.peak = Math.max(state.peak, live.size);
                }
                if (tag === 2) {
                  live.delete(id);
                  state.destroyed++;
                }
                if (tag === 55) state.cameraUpdates++;
                if (tag === 8 || tag === 9) {
                  const size = view.getUint16(payload + 4, true);
                  const text = new TextDecoder().decode(batch.subarray(payload + 6, payload + 6 + size));
                  if (text === "WILLOWBROOK") state.ready = true;
                }
                offset += 3 + length;
              }
              state.live = live.size;
              return original(batch);
            };
          },
        });
      });
      await page.goto(`http://127.0.0.1:${server.address().port}/?debug=0`);
      await page.waitForFunction(() => residencyEvidence.ready, null, { timeout: 30000 });
      const frames = async (count) => {
        const start = await page.evaluate(() => residencyEvidence.frame);
        await page.waitForFunction((target) => residencyEvidence.frame >= target, start + count, { timeout: 15000 });
      };
      await frames(60);
      const initial = await page.evaluate(() => structuredClone(residencyEvidence));
      const canvas = await page.locator("canvas").first().boundingBox();
      assert.ok(canvas);
      let seed = 937;
      for (let index = 0; index < 32; index++) {
        seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
        const x = canvas.x + canvas.width * (0.05 + (0.9 * seed) / 4294967296);
        seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
        const y = canvas.y + canvas.height * (0.05 + (0.9 * seed) / 4294967296);
        evidence.clicks.push({ x, y });
        await page.mouse.move(x, y);
        await page.mouse.down();
        await frames(9);
        await page.mouse.up();
        await frames(15);
        assert.deepEqual(evidence.errors, []);
      }
      await frames(60);
      evidence.state = await page.evaluate(() => structuredClone(residencyEvidence));
      assert.ok(evidence.state.peak <= 384, "chunked sample must leave substantial SDK entity headroom");
      assert.ok(evidence.state.destroyed > initial.destroyed + 8, "click movement must retire map sprites");
      assert.ok(evidence.state.created > initial.created + 8, "click movement must admit replacement sprites");
      assert.ok(evidence.state.cameraUpdates > initial.cameraUpdates + 10, "input must move the camera");
      assert.ok(evidence.state.maxRecords <= 4096 && evidence.state.maxBytes <= 65536);
      assert.deepEqual(evidence.errors, []);
      await page.screenshot({ path: resolve(output, `clicks-${viewport.width}.png`), fullPage: true, timeout: 15000 });
      console.log(JSON.stringify({ viewport, state: evidence.state }));
    } finally {
      clearTimeout(deadline);
      await browser.close();
    }
  }
  report.passed = true;
} finally {
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
  await new Promise((done) => server.close(done));
}
