// Observe the actual composited video, rather than treating its status label as
// playback evidence. Screenshot decoding does not alter the game canvas.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, readFile, realpath, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";
import { startVillageServer } from "./willowbrook-server.mjs";

const options = process.argv.slice(2);
const webDirectory = options[0] && !options[0].startsWith("--") ? options.shift() : "samples/media-lab/dist/web";
assert.ok(
  options.length <= 1 && options.every((option) => option.startsWith("--url=")),
  "usage: node scripts/check-media-lab-video-web.mjs [web-directory] [--url=http://127.0.0.1:4251/scene/index.html]",
);
const previewURL = options.length ? new URL(options[0].slice("--url=".length)) : null;
assert.ok(!previewURL || ["http:", "https:"].includes(previewURL.protocol), "preview URL must use HTTP or HTTPS");
const directory = await realpath(resolve(webDirectory));
const output = resolve(process.env.KARTY_MEDIA_VIDEO_OUTPUT || "dist/validation/media-lab/video");
await mkdir(output, { recursive: true });
const hash = async (name) =>
  createHash("sha256")
    .update(await readFile(resolve(directory, name)))
    .digest("hex");
const report = {
  passed: false,
  scope:
    "Production WASM in Chromium SwiftShader; real mouse/touch input and composited MPEG-1 frames. No physical audio or device FPS qualification.",
  hostSha256: await hash("karty-host.wasm"),
  cartridgeSha256: await hash("game.kart"),
  cases: [],
  errors: [],
};
const fixtures = [
  { name: "desktop", viewport: { width: 960, height: 540 }, dpr: 1 },
  { name: "fractional-dpr2", viewport: { width: 1208, height: 841 }, dpr: 2 },
  { name: "portrait", viewport: { width: 390, height: 844 }, dpr: 1 },
];
const server = previewURL ? null : await startVillageServer(directory);
const url = previewURL?.href || `http://127.0.0.1:${server.address().port}/index.html`;
report.url = url;
let browser;
try {
  if (previewURL) {
    report.servedArtifacts = {};
    for (const [name, expected] of [
      ["karty-host.wasm", report.hostSha256],
      ["game.kart", report.cartridgeSha256],
    ]) {
      const assetURL = new URL(name, url).href;
      const response = await fetch(assetURL);
      assert.equal(response.status, 200, `Preview did not serve ${name}`);
      const digest = createHash("sha256");
      let bytes = 0;
      for await (const chunk of response.body) {
        digest.update(chunk);
        bytes += chunk.length;
      }
      const sha256 = digest.digest("hex");
      report.servedArtifacts[name] = { url: assetURL, bytes, sha256 };
      assert.equal(sha256, expected, `Preview ${name} differs from selected distribution`);
    }
  }
  browser = await chromium.launch({
    headless: true,
    args: ["--no-sandbox", "--use-gl=angle", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  for (const fixture of fixtures) {
    const context = await browser.newContext({
      viewport: fixture.viewport,
      deviceScaleFactor: fixture.dpr,
      hasTouch: true,
    });
    const page = await context.newPage();
    page.setDefaultTimeout(30000);
    const deadline = setTimeout(() => void context.close(), 90000);
    const result = { ...fixture, frames: {}, videoResponses: [] };
    report.cases.push(result);
    page.on("response", (response) => {
      if (new URL(response.url()).pathname.endsWith(".kvid"))
        result.videoResponses.push({ url: response.url(), status: response.status() });
    });
    page.on("requestfailed", (request) => {
      if (new URL(request.url()).pathname.endsWith(".kvid"))
        report.errors.push(`${fixture.name}: video request failed: ${request.url()}: ${request.failure()?.errorText}`);
    });
    page.on("pageerror", (error) => report.errors.push(`${fixture.name}: ${error.message}`));
    page.on("console", (message) => {
      if (message.type() === "error" || /video playback failed/i.test(message.text()))
        report.errors.push(`${fixture.name}: ${message.text()}`);
    });
    try {
      await page.addInitScript(() => {
        const evidence = (globalThis.mediaVideo = { calls: [], texts: {}, frame: 0 });
        let play, stop, update, submit;
        Object.defineProperty(globalThis, "kartyHostPlayVideo", {
          configurable: true,
          get: () => play,
          set(original) {
            play = (...args) => {
              evidence.calls.push(args);
              return original(...args);
            };
          },
        });
        Object.defineProperty(globalThis, "kartyHostStopVideo", {
          configurable: true,
          get: () => stop,
          set(original) {
            stop = (...args) => {
              evidence.calls.push([0, 0, 0, 0, 0]);
              return original(...args);
            };
          },
        });
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
        Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
          configurable: true,
          get: () => submit,
          set(original) {
            submit = (batch) => {
              const view = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
              let offset = 24;
              for (let i = 0; i < view.getUint32(16, true); i++) {
                const tag = view.getUint8(offset),
                  p = offset + 3,
                  size = view.getUint16(offset + 1, true);
                if (tag === 20) {
                  const length = view.getUint16(p + 9, true);
                  evidence.texts[`${view.getUint32(p, true)}:${view.getUint32(p + 4, true)}`] =
                    new TextDecoder().decode(batch.subarray(p + 11, p + 11 + length));
                }
                offset += 3 + size;
              }
              return original(batch);
            };
          },
        });
      });
      await page.goto(url);
      const waitText = (text) =>
        page.waitForFunction((text) => Object.values(mediaVideo.texts).some((value) => value.includes(text)), text);
      await waitText("Play a sound, start a burst, or replay the video.");
      const box = await page.locator("canvas").boundingBox();
      assert.ok(box, "Media Lab canvas is missing");
      const scale = Math.min(box.width / 960, box.height / 540);
      const clip = {
        x: box.x + (box.width - 960 * scale) / 2 + 307 * scale,
        y: box.y + (box.height - 540 * scale) / 2 + 137 * scale,
        width: 346 * scale,
        height: 166 * scale,
      };
      const capture = async (name) => {
        const png = await page.screenshot({ clip, timeout: 30000 });
        await writeFile(resolve(output, `${fixture.name}-${name}.png`), png);
        const pixels = await page.evaluate(async (base64) => {
          const bytes = Uint8Array.from(atob(base64), (value) => value.charCodeAt(0));
          const image = await createImageBitmap(new Blob([bytes], { type: "image/png" }));
          const canvas = new OffscreenCanvas(90, 45),
            context = canvas.getContext("2d");
          context.drawImage(image, 0, 0, 90, 45);
          image.close();
          return Array.from(context.getImageData(0, 0, 90, 45).data);
        }, png.toString("base64"));
        let brightColor = 0;
        for (let i = 0; i < pixels.length; i += 4) {
          const max = Math.max(...pixels.slice(i, i + 3)),
            min = Math.min(...pixels.slice(i, i + 3));
          if (max > 180 && max - min > 110) brightColor++;
        }
        result.frames[name] = { brightColorFraction: brightColor / (pixels.length / 4) };
        return pixels;
      };
      const difference = (a, b) => {
        let sum = 0;
        for (let i = 0; i < a.length; i++) if (i % 4 !== 3) sum += Math.abs(a[i] - b[i]);
        return sum / ((a.length * 3) / 4);
      };
      const narrow = box.width <= 700,
        inset = narrow ? 12 : 20;
      const buttonWidth = (box.width - 2 * inset - 8) / 2;
      const y = box.y + box.height - (narrow ? 262 : 178);
      const touch = narrow ? await context.newCDPSession(page) : null;
      const click = async (stop = false) => {
        const x = box.x + (stop ? box.width - inset - buttonWidth / 2 : inset + buttonWidth / 2);
        if (touch) {
          const frame = await page.evaluate(() => mediaVideo.frame);
          await touch.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ id: 1, x, y }] });
          await page.waitForFunction((frame) => mediaVideo.frame >= frame + 4, frame);
          await touch.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
        } else await page.mouse.click(x, y);
      };
      const baseline = await capture("before");
      await click();
      await waitText("Playing MPEG-1 video with MP2 sound");
      await page.waitForTimeout(500);
      const first = await capture("playing-first");
      assert.ok(
        result.frames["playing-first"].brightColorFraction > 0.5,
        "Play updated its label but no decoded video was presented",
      );
      await page.waitForTimeout(600);
      const second = await capture("playing-second");
      result.frameDifference = difference(first, second);
      assert.ok(result.frameDifference > 3, "Decoded video did not advance between captures");
      await click(true);
      await waitText("Video stopped; press Play video to replay");
      await page.waitForTimeout(100);
      const stopped = await capture("stopped");
      result.stoppedDifference = difference(baseline, stopped);
      assert.ok(result.stoppedDifference < 2, "Stop did not remove the decoded video");
      await click();
      await waitText("Playing MPEG-1 video with MP2 sound");
      await page.waitForTimeout(500);
      await capture("replayed");
      assert.ok(result.frames.replayed.brightColorFraction > 0.5, "Replay did not present decoded video");
      result.evidence = await page.evaluate(() => structuredClone(mediaVideo));
      assert.deepEqual(
        result.evidence.calls,
        [
          [1, 300, 130, 360, 180],
          [0, 0, 0, 0, 0],
          [1, 300, 130, 360, 180],
        ],
        "Unexpected video lifecycle calls",
      );
      assert.ok(result.videoResponses.length > 0, "No cold video asset request was observed");
      assert.ok(
        result.videoResponses.every((response) => response.status === 200),
        "Preview did not serve every video asset request successfully",
      );
      result.passed = true;
      console.log(`Media Lab video: ${fixture.name} decoded, advanced, stopped and replayed`);
    } catch (error) {
      result.failure = String(error);
      await page.screenshot({ path: resolve(output, `${fixture.name}-failed.png`), timeout: 5000 }).catch(() => {});
      throw error;
    } finally {
      clearTimeout(deadline);
      await context.close();
    }
  }
  assert.deepEqual(report.errors, [], "browser errors or video decoder failures");
  report.passed = true;
} finally {
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
  await browser?.close();
  if (server) {
    server.closeAllConnections();
    await new Promise((done) => server.close(done));
  }
}
