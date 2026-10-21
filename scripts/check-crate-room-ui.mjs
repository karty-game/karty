// Observe the real cartridge command stream; never replace its input or output.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";
import { startVillageServer } from "./willowbrook-server.mjs";

const directory = resolve(process.argv[2] || "samples/crate-room/dist/web");
const output = resolve(process.env.KARTY_CRATE_ROOM_UI_OUTPUT || "dist/validation/crate-room-ui");
await mkdir(output, { recursive: true });
const hash = async (name) =>
  createHash("sha256")
    .update(await readFile(resolve(directory, name)))
    .digest("hex");
const report = {
  passed: false,
  scope:
    "Actual WASM in Chromium SwiftShader; synthetic multi-touch, mouse and keyboard. No physical device or FPS qualification.",
  hostSha256: await hash("karty-host.wasm"),
  cartridgeSha256: await hash("game.kart"),
  checks: [],
  errors: [],
};
const server = await startVillageServer(directory);
const browser = await chromium.launch({
  headless: true,
  args: ["--no-sandbox", "--use-gl=angle", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
});
const deadline = setTimeout(() => {
  throw new Error("Crate room UI exceeded 120s");
}, 120000);
try {
  for (const [name, width, height, deviceScaleFactor] of [
    ["desktop", 1280, 720, 1],
    ["landscape", 844, 390, 1],
    ["portrait-hidpi", 390, 844, 2],
  ]) {
    const context = await browser.newContext({ viewport: { width, height }, deviceScaleFactor, hasTouch: true });
    const page = await context.newPage();
    page.on("pageerror", (error) => report.errors.push(`${name}: ${error.message}`));
    await page.addInitScript(() => {
      const state = (globalThis.crateUIEvidence = {
        frame: 0,
        legacyText: 0,
        status: "",
        shots: 0,
        blasts: 0,
        yaw: 0,
        moving: 0,
      });
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
            let offset = 24;
            for (let i = 0; i < view.getUint32(16, true); i++) {
              const tag = view.getUint8(offset),
                p = offset + 3;
              if (tag === 8) state.legacyText++;
              if (tag === 20) {
                const length = view.getUint16(p + 9, true);
                const text = new TextDecoder().decode(batch.slice(p + 11, p + 11 + length));
                const counts = /CRATE ROOM.*Shots (\d+).*Blasts (\d+)/.exec(text);
                if (counts) {
                  state.status = text;
                  state.shots = Number(counts[1]);
                  state.blasts = Number(counts[2]);
                }
              }
              if (tag === 28) state.yaw = view.getFloat32(p + 20, true);
              if (tag === 45 && Math.hypot(view.getFloat32(p + 4, true), view.getFloat32(p + 8, true)) > 0.01)
                state.moving++;
              offset += 3 + view.getUint16(offset + 1, true);
            }
            return original(batch);
          };
        },
      });
    });
    await page.goto(`http://127.0.0.1:${server.address().port}/index.html`);
    await page.waitForFunction(() => globalThis.crateUIEvidence?.status.includes("CRATE ROOM"), null, {
      timeout: 45000,
    });
    assert.equal(
      await page.evaluate(() => crateUIEvidence.legacyText),
      0,
      "All sample text belongs to presentation UI",
    );
    const touch = await context.newCDPSession(page);
    const point = (id, x, y) => ({ id, x: width / 2 + ((x - 640) * height) / 720, y: (y * height) / 720 });
    const shoot = point(3, 640, 585);
    await touch.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [shoot] });
    await page.waitForFunction(() => crateUIEvidence.shots >= 3, null, { timeout: 10000 });
    await touch.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    const afterRelease = await page.evaluate(() => ({ frame: crateUIEvidence.frame, shots: crateUIEvidence.shots }));
    await page.waitForFunction((frame) => crateUIEvidence.frame >= frame + 20, afterRelease.frame);
    assert.equal(
      await page.evaluate(() => crateUIEvidence.shots),
      afterRelease.shots,
      "Releasing SHOOT stops repeat fire",
    );
    const beforeMulti = await page.evaluate(() => ({ ...crateUIEvidence }));
    await touch.send("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: [point(1, 610, 660), point(2, 670, 660), shoot],
    });
    // Let the host observe each finger's down position before moving it.
    const touchFrame = await page.evaluate(() => crateUIEvidence.frame);
    await page.waitForFunction((frame) => crateUIEvidence.frame >= frame + 2, touchFrame);
    await touch.send("Input.dispatchTouchEvent", {
      type: "touchMove",
      touchPoints: [point(1, 600, 650), point(2, 690, 650), shoot],
    });
    await page
      .waitForFunction(
        (before) =>
          crateUIEvidence.shots >= before.shots + 2 &&
          crateUIEvidence.moving > before.moving &&
          Math.abs(crateUIEvidence.yaw - before.yaw) > 0.01,
        beforeMulti,
        { timeout: 10000 },
      )
      .catch(async (error) => {
        report.failure = {
          viewport: name,
          before: beforeMulti,
          after: await page.evaluate(() => ({ ...crateUIEvidence })),
        };
        await page.screenshot({ path: resolve(output, `${name}-failure.png`) });
        throw error;
      });
    await touch.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    const narrow = width <= 600;
    const actionX = width - (narrow ? 56 : 92);
    const resetY = narrow ? 34 : 44;
    await page.screenshot({ path: resolve(output, `${name}-before-actions.png`) });
    await page.mouse.click(actionX, resetY + 52, { delay: 100 });
    await page.waitForFunction(() => crateUIEvidence.blasts === 1);
    await page.mouse.click(actionX, resetY, { delay: 100 });
    await page.waitForFunction(() => crateUIEvidence.shots === 0 && crateUIEvidence.blasts === 0);
    await page.keyboard.press("b", { delay: 100 });
    await page.waitForFunction(() => crateUIEvidence.blasts === 1);
    await page.keyboard.press("r", { delay: 100 });
    await page.waitForFunction(() => crateUIEvidence.shots === 0 && crateUIEvidence.blasts === 0);
    await page.screenshot({ path: resolve(output, `${name}.png`) });
    report.checks.push(
      `${name}: presentation HUD, held-fire release, simultaneous move/look/shoot, BLAST/RESET UI and keyboard`,
    );
    await context.close();
  }
  assert.deepEqual(report.errors, []);
  report.passed = true;
} finally {
  clearTimeout(deadline);
  await browser.close();
  await new Promise((done) => server.close(done));
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
}
console.log(JSON.stringify(report, null, 2));
