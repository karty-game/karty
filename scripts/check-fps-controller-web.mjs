// Exercise an independent built FPS starter. Observe guest commands without
// replacing input, commands, cartridge sources or host rendering.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";
import { startVillageServer } from "./willowbrook-server.mjs";
import { observeController } from "./fps-browser-evidence.mjs";

assert.ok(process.argv[2], "Usage: node scripts/check-fps-controller-web.mjs <fps-starter/dist/web>");
const directory = resolve(process.argv[2]);
const output = resolve(process.env.KARTY_FPS_CONTROLLER_WEB_OUTPUT || "dist/validation/fps-controller-web");
await mkdir(output, { recursive: true });
const hash = async (path) =>
  createHash("sha256")
    .update(await readFile(path))
    .digest("hex");
const artifacts = ["karty-host.wasm", "game.kart"];
for (const name of await readdir(resolve(directory, "content"))) {
  if (name.endsWith(".kld")) artifacts.push(`content/${name}`);
}
assert.ok(artifacts.length > 2, "The built starter must include a room level");
const report = {
  passed: false,
  scope:
    "Actual independent FPS starter WASM and host physics in Chromium SwiftShader; synthetic keyboard and CDP multi-touch, observed commands and accepted host poses, plus screenshots. No physical-device or device-FPS qualification.",
  distribution: directory,
  artifacts: Object.fromEntries(
    await Promise.all(artifacts.sort().map(async (name) => [name, await hash(resolve(directory, name))])),
  ),
  checks: [],
  errors: [],
};
let server, browser, deadline;
try {
  server = await startVillageServer(directory);
  const origin = `http://127.0.0.1:${server.address().port}`;
  // Check provenance against the server's immutable snapshot, not live build files.
  for (const [name, sha256] of Object.entries(report.artifacts)) {
    const response = await fetch(`${origin}/${name}`, { method: "HEAD" });
    assert.equal(response.status, 200, `${name} is served`);
    assert.equal(response.headers.get("etag"), `"${sha256}"`, `${name} snapshot provenance`);
  }
  browser = await chromium.launch({
    headless: true,
    args: ["--no-sandbox", "--use-gl=angle", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  const run = async () => {
    for (const [name, width, height, deviceScaleFactor] of [
      ["desktop", 1280, 720, 1],
      ["landscape", 844, 390, 1],
      ["portrait-hidpi", 390, 844, 2],
    ]) {
      const context = await browser.newContext({ viewport: { width, height }, deviceScaleFactor, hasTouch: true });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      page.on("pageerror", (error) => report.errors.push(`${name}: ${error.message}`));
      page.on("console", (message) => {
        if (message.type() === "error") report.errors.push(`${name}: ${message.text()}`);
      });
      await page.addInitScript(observeController);
      const evidence = async () => page.evaluate(() => ({ ...globalThis.fpsEvidence }));
      const stop = async () => {
        await page.waitForFunction(
          () => fpsEvidence.velocity && Math.hypot(fpsEvidence.velocity.x, fpsEvidence.velocity.y) < 0.001,
        );
        const zero = await evidence();
        // Input is submitted before the next physics feedback; let that final
        // accepted step arrive before measuring a stationary body.
        await page.waitForFunction((frame) => fpsEvidence.frame >= frame + 3, zero.frame);
        const stopped = await evidence();
        await page.waitForFunction((frame) => fpsEvidence.frame >= frame + 20, stopped.frame);
        const after = await evidence();
        assert.equal(after.moving, zero.moving, `${name}: released controls remain stopped`);
        assert.equal(after.yaw, stopped.yaw, `${name}: released look remains stopped`);
        assert.ok(distance(after.pose, stopped.pose) < 0.001, `${name}: released host body remains stationary`);
        assert.ok(Math.abs(after.pose.z - stopped.pose.z) < 0.005, `${name}: released host body stays grounded`);
        return stopped;
      };
      try {
        await page.goto(`${origin}/index.html`);
        await page.waitForFunction(
          () => fpsEvidence.camera !== 0 && fpsEvidence.velocity !== null && fpsEvidence.pose !== null,
          null,
          {
            timeout: 45000,
          },
        );
        const initial = await evidence();
        assert.equal(initial.commandVersion, 16, "Current FPS uses wire16");
        assert.equal(initial.body?.flags, 3, "Default FPS attaches the shared capsule motor");
        assert.equal(initial.vectors.length, 0, "Default touch presentation starts on first touch");

        await page.keyboard.down("w");
        await page.waitForFunction(
          (before) =>
            fpsEvidence.moving >= before.moving + 3 &&
            Math.hypot(fpsEvidence.pose.x - before.pose.x, fpsEvidence.pose.y - before.pose.y) > 0.04,
          initial,
        );
        const keyboard = await evidence();
        assert.equal(keyboard.velocity.flags, 1, "Walking preserves vertical physics velocity");
        grounded(keyboard.pose, `${name}: keyboard host pose`);
        await page.keyboard.up("w");
        const keyboardStopped = await stop();

        const touch = await context.newCDPSession(page);
        // CSS screen coordinates let the host apply its own responsive mapping.
        // Floating sticks must work in portrait without authored edge assumptions.
        const points = [
          { id: 1, x: width * 0.25, y: height * 0.8 },
          { id: 2, x: width * 0.75, y: height * 0.8 },
        ];
        const beforeTouch = await evidence();
        await touch.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: points });
        const down = await evidence();
        await page.waitForFunction((frame) => fpsEvidence.frame >= frame + 2, down.frame);
        await touch.send("Input.dispatchTouchEvent", {
          type: "touchMove",
          touchPoints: [
            { ...points[0], y: points[0].y - 30 },
            { ...points[1], x: points[1].x + 30 },
          ],
        });
        await page.waitForFunction(
          (before) =>
            fpsEvidence.moving >= before.moving + 3 &&
            Math.abs(fpsEvidence.yaw - before.yaw) > 0.01 &&
            Math.hypot(fpsEvidence.pose.x - before.pose.x, fpsEvidence.pose.y - before.pose.y) > 0.015 &&
            fpsEvidence.vectors.length === 4,
          beforeTouch,
        );
        const active = await evidence();
        assert.equal(active.velocity.flags, 1, "Touch walking preserves vertical physics velocity");
        grounded(active.pose, `${name}: touch host pose`);
        const activeImage = `${name}-touch.png`;
        await page.screenshot({ path: resolve(output, activeImage) });
        await touch.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
        const released = await stop();
        const releasedImage = `${name}-released.png`;
        await page.screenshot({ path: resolve(output, releasedImage) });
        report.checks.push({
          viewport: name,
          width,
          height,
          deviceScaleFactor,
          keyboard: {
            movingCommands: keyboard.moving - initial.moving,
            startPose: initial.pose,
            acceptedDisplacement: distance(keyboard.pose, initial.pose),
            acceptedPose: keyboard.pose,
            stoppedPose: keyboardStopped.pose,
            stoppedFrame: keyboardStopped.frame,
          },
          touch: {
            movingCommands: active.moving - beforeTouch.moving,
            startPose: beforeTouch.pose,
            yawChange: active.yaw - beforeTouch.yaw,
            shapeEntities: active.vectors,
            acceptedDisplacement: distance(active.pose, beforeTouch.pose),
            acceptedPose: active.pose,
            stoppedPose: released.pose,
            stoppedFrame: released.frame,
          },
          screenshots: Object.fromEntries(
            await Promise.all(
              [activeImage, releasedImage].map(async (file) => [file, await hash(resolve(output, file))]),
            ),
          ),
        });
      } catch (error) {
        report.failure = { viewport: name, evidence: await evidence().catch(() => null) };
        await page.screenshot({ path: resolve(output, `${name}-failure.png`) }).catch(() => {});
        throw error;
      } finally {
        await context.close();
      }
    }
  };
  await Promise.race([
    run(),
    new Promise((_, reject) => {
      deadline = setTimeout(() => reject(new Error("FPS controller acceptance exceeded 120s")), 120000);
    }),
  ]);
  assert.deepEqual(report.errors, []);
  report.passed = true;
} catch (error) {
  report.errors.push(error.stack || String(error));
  throw error;
} finally {
  clearTimeout(deadline);
  if (browser) await browser.close();
  if (server) await new Promise((done) => server.close(done));
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
  console.log(JSON.stringify(report, null, 2));
}

function distance(first, second) {
  return Math.hypot(first.x - second.x, first.y - second.y);
}

function grounded(pose, message) {
  assert.ok([pose.x, pose.y, pose.z].every(Number.isFinite) && pose.z > 1.5 && pose.z < 1.9, message);
}
