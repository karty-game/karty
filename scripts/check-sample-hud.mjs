// Observe production command batches without changing gameplay or rendering.
// HUD labels must use KartUI and remain interactive at presentation resolution.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, readFile, realpath, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";
import { startVillageServer } from "./willowbrook-server.mjs";

const sample = process.argv[2];
assert.ok(
  ["terrain", "media-lab"].includes(sample),
  "usage: node scripts/check-sample-hud.mjs terrain|media-lab [web-directory] [--world-scale=0.25]",
);
const options = process.argv.slice(3);
const webDirectory = options[0] && !options[0].startsWith("--") ? options.shift() : `samples/${sample}/dist/web`;
assert.ok(
  options.length <= 1 && options.every((option) => option === "--world-scale=0.25" && sample === "terrain"),
  "only Terrain supports --world-scale=0.25",
);
const worldScale = options.length ? 0.25 : null;
const directory = await realpath(resolve(webDirectory));
const output = resolve(process.env.KARTY_SAMPLE_HUD_OUTPUT || `dist/validation/sample-hud/${sample}`);
await mkdir(output, { recursive: true });
async function capture(page, name) {
  const session = await page.context().newCDPSession(page);
  const viewport = page.viewportSize();
  const dpr = await page.evaluate(() => devicePixelRatio);
  let timer;
  try {
    const screenshot = await Promise.race([
      session.send("Page.captureScreenshot", {
        format: "png",
        fromSurface: true,
        captureBeyondViewport: false,
        optimizeForSpeed: true,
        clip: { x: 0, y: 0, width: viewport.width, height: viewport.height, scale: dpr },
      }),
      new Promise((_, reject) => {
        timer = setTimeout(() => reject(new Error("Composited screenshot exceeded 30s")), 30000);
      }),
    ]);
    const bytes = Buffer.from(screenshot.data, "base64");
    await writeFile(resolve(output, name), bytes);
    return { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) };
  } finally {
    clearTimeout(timer);
    await session.detach();
  }
}
const hash = async (name) =>
  createHash("sha256")
    .update(await readFile(resolve(directory, name)))
    .digest("hex");
const report = {
  passed: false,
  sample,
  scope: `Production WASM in Chromium SwiftShader ${worldScale ? "with worldScale=0.25 for focused presentation UI validation" : "at default rendering quality"}; passive command observation and synthetic keyboard/mouse/touch. No physical-device FPS or audio qualification.`,
  worldScale,
  hostSha256: await hash("karty-host.wasm"),
  cartridgeSha256: await hash("game.kart"),
  cases: [],
  errors: [],
};
const desktop = sample === "terrain" ? { width: 1280, height: 720 } : { width: 960, height: 540 };
const cases = [
  { name: "desktop", viewport: desktop, dpr: 1 },
  { name: "fractional-dpr1", viewport: { width: 1208, height: 841 }, dpr: 1 },
  { name: "fractional-dpr2", viewport: { width: 1208, height: 841 }, dpr: 2 },
  { name: "portrait", viewport: { width: 390, height: 844 }, dpr: 1 },
];
const server = await startVillageServer(directory);
let browser;
try {
  browser = await chromium.launch({
    headless: true,
    args: ["--no-sandbox", "--use-gl=angle", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  for (const fixture of cases) {
    const context = await browser.newContext({
      viewport: fixture.viewport,
      deviceScaleFactor: fixture.dpr,
      hasTouch: true,
    });
    const page = await context.newPage();
    page.setDefaultTimeout(30000);
    const timer = setTimeout(() => void context.close(), 120000);
    page.on("pageerror", (error) => report.errors.push(`${fixture.name}: ${error.message}`));
    page.on("console", (message) => {
      if (message.type() === "error") report.errors.push(`${fixture.name}: ${message.text()}`);
    });
    const checks = [];
    try {
      await page.addInitScript(() => {
        const evidence = (globalThis.sampleHUD = { legacyText2D: 0, texts: {}, seen: [], batches: 0, frame: 0 });
        let update;
        Object.defineProperty(globalThis, "kartyClientUpdate", {
          configurable: true,
          get: () => update,
          set(original) {
            update = (frame, batch) => {
              evidence.frame = Number(frame);
              return original(frame, batch);
            };
          },
        });
        let submit;
        Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
          configurable: true,
          get: () => submit,
          set(original) {
            submit = (batch) => {
              const view = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
              let offset = 24;
              evidence.batches++;
              for (let i = 0; i < view.getUint32(16, true); i++) {
                const tag = view.getUint8(offset),
                  p = offset + 3,
                  size = view.getUint16(offset + 1, true);
                if (tag === 8) evidence.legacyText2D++;
                if (tag === 20) {
                  const instance = view.getUint32(p, true),
                    element = view.getUint32(p + 4, true);
                  const length = view.getUint16(p + 9, true);
                  const text = new TextDecoder().decode(batch.subarray(p + 11, p + 11 + length));
                  evidence.texts[`${instance}:${element}`] = text;
                  if (!evidence.seen.includes(text)) evidence.seen.push(text);
                }
                if (tag === 19) {
                  const instance = view.getUint32(p, true);
                  for (const key of Object.keys(evidence.texts))
                    if (key.startsWith(`${instance}:`)) delete evidence.texts[key];
                }
                offset += 3 + size;
              }
              return original(batch);
            };
          },
        });
      });
      const url = new URL(`http://127.0.0.1:${server.address().port}/index.html`);
      if (worldScale) url.searchParams.set("worldScale", String(worldScale));
      await page.goto(url.href);
      const waitText = (text) =>
        page.waitForFunction((text) => Object.values(sampleHUD.texts).some((value) => value.includes(text)), text);
      const canvas = page.locator("canvas");
      if (sample === "terrain") {
        await waitText("Cove beach · Terrain study");
        const waitWorld = (frames) =>
          page.waitForFunction(
            (frames) =>
              globalThis.kartyWorldRenderer?.stats().cameras.some((camera) => camera.device?.frames >= frames),
            frames,
          );
        await waitWorld(3);
        await canvas.evaluate((element) => {
          element.tabIndex = 0;
          element.focus();
        });
        await page.keyboard.press("v");
        await waitText("Cove cliff foot · Terrain study");
        await waitWorld(2);
        await page.keyboard.press("r");
        await waitText("Cove beach · Terrain study");
        await waitWorld(2);
        checks.push("viewpoint label is KartUI; real V cycles and R resets the viewpoint label");
        checks.push("3D renderer presents at least three initial frames and two frames after each viewpoint switch");
      } else {
        await waitText("Play a sound, start a burst, or replay the video.");
        await waitText("BGM REPLAY: OFF");
        const box = await canvas.boundingBox();
        assert.ok(box, "media canvas is not visible");
        const { width, height } = box;
        const narrow = width <= 700;
        const inset = narrow ? 12 : 20;
        const touch = narrow ? await context.newCDPSession(page) : null;
        const frames = async () => {
          const before = await page.evaluate(() => sampleHUD.frame);
          await page.waitForFunction((before) => sampleHUD.frame >= before + 4, before);
        };
        const click = async (x, y, text) => {
          assert.ok(x > 0 && y > 0 && x < width && y < height, "button target falls outside canvas");
          if (narrow) {
            await touch.send("Input.dispatchTouchEvent", {
              type: "touchStart",
              touchPoints: [{ id: 1, x: box.x + x, y: box.y + y }],
            });
            await frames();
            await touch.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
            await frames();
          } else await page.mouse.click(box.x + x, box.y + y);
          await waitText(text);
        };
        const videoY = height - (narrow ? 262 : 178);
        const videoWidth = (width - 2 * inset - 8) / 2;
        await click(inset + videoWidth / 2, videoY, "Playing MPEG-1 video with MP2 sound");
        await click(width - inset - videoWidth / 2, videoY, "Video stopped; press Play video to replay");
        const audioWidth = (width - 2 * inset - 24) / 3;
        const audio = narrow
          ? [
              [width / 2, height - 208],
              [width / 2, height - 152],
              [width / 2, height - 96],
            ]
          : [
              [inset + audioWidth / 2, height - 110],
              [width / 2, height - 110],
              [width - inset - audioWidth / 2, height - 110],
            ];
        await click(...audio[0], "Played one short 8-bit source through QOA");
        await click(...audio[1], "Burst started: overlapping the same QOA sample");
        await click(...audio[2], "BGM started with a host-owned streaming QOA loop");
        await waitText("BGM REPLAY: ON");
        await click(...audio[2], "BGM loop stopped");
        await waitText("BGM REPLAY: OFF");
        checks.push(
          `${narrow ? "touch" : "mouse"} activates video play/stop, one-shot, burst and music on/off through KartUI`,
        );
      }
      await page.waitForFunction(() => document.querySelector("canvas")?.width > 0);
      const evidence = await page.evaluate(() => structuredClone(sampleHUD));
      const presentation = await page.evaluate(() => ({
        dpr: devicePixelRatio,
        canvasWidth: document.querySelector("canvas").width,
        canvasHeight: document.querySelector("canvas").height,
      }));
      assert.equal(presentation.dpr, fixture.dpr);
      assert.equal(evidence.legacyText2D, 0, "sample created legacy world Text2D instead of presentation UI");
      assert.ok(
        Object.values(evidence.texts).length >= (sample === "terrain" ? 1 : 2),
        "presentation labels are missing",
      );
      checks.push("no legacy Text2D commands; current UI text retained by instance and element");
      const screenshot = await capture(page, `${fixture.name}.png`);
      assert.deepEqual(screenshot, {
        width: fixture.viewport.width * fixture.dpr,
        height: fixture.viewport.height * fixture.dpr,
      });
      report.cases.push({ ...fixture, url: url.href, checks, evidence, presentation, screenshot });
      console.log(`${sample}: ${fixture.name} passed`);
    } catch (error) {
      const evidence = await page.evaluate(() => structuredClone(globalThis.sampleHUD)).catch(() => null);
      const renderer =
        sample === "terrain"
          ? await page.evaluate(() => globalThis.kartyWorldRenderer?.stats()).catch(() => null)
          : null;
      report.cases.push({ ...fixture, checks, failure: String(error), evidence, renderer });
      await page.screenshot({ path: resolve(output, `${fixture.name}-failed.png`), timeout: 5000 }).catch(() => {});
      throw error;
    } finally {
      clearTimeout(timer);
      await context.close();
    }
  }
  assert.deepEqual(report.errors, [], "browser errors");
  report.passed = true;
} finally {
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
  await browser?.close();
  server.closeAllConnections();
  await new Promise((done) => server.close(done));
}
