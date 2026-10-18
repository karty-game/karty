// Execute ordinary SDK cartridges and the actual WASM/WebGL host. No test API
// is added to the game: observe existing command/event batches in the launcher.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { createServer } from "node:http";
import { mkdir, readFile, realpath, writeFile } from "node:fs/promises";
import { extname, relative, resolve } from "node:path";
import { chromium } from "playwright";

const root = await realpath(process.env.KARTY_STDLIB_GAMES || "testdata/local-packages/games");
const artifacts = resolve(process.env.KARTY_STDLIB_ARTIFACTS || "dist/validation/stdlib-spike");
await mkdir(artifacts, { recursive: true });
const mime = new Map([
  [".html", "text/html"],
  [".js", "text/javascript"],
  [".wasm", "application/wasm"],
  [".kart", "application/wasm"],
]);
const server = createServer(async (request, response) => {
  try {
    const name = decodeURIComponent(new URL(request.url, "http://localhost").pathname);
    const file = await realpath(resolve(root, "." + name));
    if (relative(root, file).startsWith("..")) {
      response.writeHead(403);
      response.end();
      return;
    }
    const data = await readFile(file);
    response.writeHead(200, {
      "Content-Type": mime.get(extname(file)) || "application/octet-stream",
      "Cache-Control": "no-store",
    });
    response.end(data);
  } catch {
    response.writeHead(404);
    response.end();
  }
});
await new Promise((done, reject) => {
  server.once("error", reject);
  server.listen(0, "127.0.0.1", done);
});
const reports = [];
try {
  for (const compiler of ["go", "tinygo"]) {
    for (const profile of ["walk", "precision"]) {
      reports.push(await check(profile, compiler));
    }
  }
  const hash = createHash("sha256");
  for (const name of ["go.mod", "fps/controller.go", "controls/input.go"]) {
    hash.update(name);
    hash.update(await readFile(resolve("testdata/local-packages/library", name)));
  }
  await writeFile(
    resolve(artifacts, "report.json"),
    JSON.stringify({ librarySourceSHA256: hash.digest("hex"), reports }, null, 2) + "\n",
  );
  console.log(JSON.stringify(reports, null, 2));
} finally {
  server.closeAllConnections();
  await new Promise((done) => server.close(done));
}

async function check(profile, compiler) {
  const browser = await chromium.launch({
    headless: true,
    timeout: 9000,
    args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  try {
    const context = await browser.newContext({ viewport: { width: 960, height: 540 }, hasTouch: true });
    const page = await context.newPage();
    page.setDefaultTimeout(9000);
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.addInitScript(() => {
      globalThis.observed = {
        camera: 0,
        cameras: [],
        destroyed: [],
        poses: {},
        velocities: {},
        boxes: 0,
        frame: 0,
        touchActive: [],
      };
      document.addEventListener("touchend", (event) => {
        observed.touchActive = Array.from(event.touches, (touch) => touch.identifier);
      });
      function records(batch, visit) {
        if (batch.byteLength < 24) return;
        const view = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
        let offset = 24;
        for (let i = 0; i < view.getUint32(16, true); i++) {
          const tag = view.getUint8(offset),
            length = view.getUint16(offset + 1, true),
            start = offset + 3;
          visit(tag, view, start);
          offset = start + length;
        }
      }
      let submit, update;
      Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
        configurable: true,
        get: () => submit,
        set(original) {
          submit = (batch) => {
            records(batch, (tag, view, start) => {
              const id = view.getUint32(start, true);
              if (tag === 27) {
                observed.camera = id;
                observed.cameras.push(id);
              }
              if (tag === 2) observed.destroyed.push(id);
              if (tag === 42) observed.boxes++;
              if (tag === 45)
                observed.velocities[id] = [view.getFloat32(start + 4, true), view.getFloat32(start + 8, true)];
            });
            return original(batch);
          };
        },
      });
      Object.defineProperty(globalThis, "kartyClientUpdate", {
        configurable: true,
        get: () => update,
        set(original) {
          update = (frame, batch) => {
            observed.frame = Number(frame);
            records(batch, (tag, view, start) => {
              if (tag === 11 || tag === 19) {
                const id = view.getUint32(start + (tag === 19 ? 4 : 0), true);
                const pose = start + (tag === 19 ? 20 : 4);
                observed.poses[id] = Array.from({ length: 6 }, (_, i) => view.getFloat32(pose + i * 4, true));
              }
            });
            return original(frame, batch);
          };
        },
      });
    });
    await page.goto(
      `http://127.0.0.1:${server.address().port}/${profile}/dist/variants/${compiler}/index.html?debug=0&worldScale=0.5&aa=false`,
    );
    await page.waitForFunction(
      () => document.documentElement.dataset.kartyState === "running" && observed.poses[observed.camera],
    );
    await writeFile(resolve(artifacts, `${profile}-${compiler}-kit.png`), await page.screenshot({ fullPage: true }));
    // Give canvas keyboard focus; release capture so keyboard-only checks start cleanly.
    await page.mouse.click(480, 300);
    await page.waitForFunction(() => document.pointerLockElement !== null);
    await page.keyboard.press("Escape");
    await page.waitForFunction(() => document.pointerLockElement === null);
    const releasedAt = await page.evaluate(() => observed.frame);
    await page.waitForFunction((frame) => observed.frame >= frame + 4, releasedAt);
    // The focus/capture click can itself deliver mouse movement. Exercise the
    // game's explicit replacement before measuring movement from yaw zero.
    const focusCamera = await page.evaluate(() => observed.camera);
    await page.keyboard.press("v");
    await page.waitForFunction((old) => observed.camera !== old && observed.poses[observed.camera], focusCamera);
    const advance = async (count = 4) => {
      const frame = await page.evaluate(() => observed.frame);
      await page.waitForFunction(({ frame, count }) => observed.frame >= frame + count, { frame, count });
    };
    const state = () =>
      page.evaluate(() => ({
        id: observed.camera,
        pose: observed.poses[observed.camera],
        velocity: observed.velocities[observed.camera],
      }));
    const hold = async (key, count = 12) => {
      await page.keyboard.down(key);
      await advance(count);
      const value = await state();
      await page.keyboard.up(key);
      await advance();
      return value;
    };
    const stopped = async () => {
      const value = await state();
      assert.ok(Math.hypot(...value.velocity) < 0.0001, "release did not command zero horizontal velocity");
      await advance(6);
      const next = await state();
      assert.ok(
        Math.hypot(next.pose[0] - value.pose[0], next.pose[1] - value.pose[1]) < 0.015,
        "released controller kept moving",
      );
    };
    await advance(8);
    const initial = await state();
    const speed = profile === "walk" ? 3.3 : 1.65;
    const forwardKey = profile === "walk" ? "w" : "ArrowUp";
    const moved = await hold(forwardKey, 16);
    assert.ok(
      Math.abs(Math.hypot(...moved.velocity) - speed) < 0.001,
      `configured speed/binding not applied (${profile}/${compiler}): ${JSON.stringify({ initial, moved })}`,
    );
    const forwardTravel =
      (moved.pose[0] - initial.pose[0]) * Math.sin(initial.pose[3]) +
      (moved.pose[1] - initial.pose[1]) * Math.cos(initial.pose[3]);
    assert.ok(forwardTravel > 0.2, "actual host body did not move along its accepted heading");
    await stopped();
    const strafed = await hold("d");
    assert.ok(
      profile === "walk" ? Math.hypot(...strafed.velocity) > 3 : Math.hypot(...strafed.velocity) < 0.0001,
      "game-specific intent customization failed",
    );
    if (profile === "precision") {
      const unbound = await hold("w");
      assert.ok(Math.hypot(...unbound.velocity) < 0.0001, "second game retained first game's bindings");
    }
    // Two physical run keys must not cancel each other when only one is released.
    await page.keyboard.down("ShiftLeft");
    await page.keyboard.down("ShiftRight");
    await page.keyboard.up("ShiftLeft");
    const ran = await hold(forwardKey, 8);
    assert.ok(Math.abs(Math.hypot(...ran.velocity) - speed * 2) < 0.001, "independent held run keys lost state");
    await page.keyboard.up("ShiftRight");
    await page.keyboard.down(forwardKey);
    await advance();
    await page.keyboard.press("p");
    await advance();
    await stopped();
    await page.keyboard.up(forwardKey);
    const pausedInput = await hold(forwardKey, 6);
    assert.ok(Math.hypot(...pausedInput.velocity) < 0.0001, "game pause did not intercept input");
    await page.keyboard.press("p");
    await advance();
    const old = (await state()).id;
    await page.keyboard.press("v");
    await page.waitForFunction((old) => observed.camera !== old && observed.poses[observed.camera], old);
    assert.ok(
      await page.evaluate((old) => observed.destroyed.includes(old), old),
      "caller did not destroy replaced camera",
    );
    await advance();
    await page.mouse.click(480, 300);
    await page.waitForFunction(() => document.pointerLockElement !== null);
    await advance();
    const yaw = (await state()).pose[3];
    // Headless pointer-lock movement uses the normal DOM listener, as physical
    // movementX/Y can be zero in Chromium automation. Capture itself is real.
    await page
      .locator("canvas")
      .evaluate((canvas) =>
        canvas.dispatchEvent(new MouseEvent("mousemove", { bubbles: true, movementX: 30, movementY: -12 })),
      );
    await page.waitForFunction((yaw) => observed.poses[observed.camera][3] > yaw + 0.04, yaw);
    const turned = await state();
    const expectedYaw = profile === "walk" ? 0.075 : 0.15;
    assert.ok(Math.abs(turned.pose[3] - yaw - expectedYaw) < 0.01, "configured mouse sensitivity failed");
    await page.keyboard.press("Escape");
    await page.waitForFunction(() => document.pointerLockElement === null);
    await advance();

    const cdp = await context.newCDPSession(page);
    const left = { id: 11, x: 220, y: 400 },
      right = { id: 22, x: 760, y: 400 };
    await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [left] });
    await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [left, right] });
    const leftMoved = { ...left, y: 320 },
      rightMoved = { ...right, x: 840 };
    await cdp.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [leftMoved, rightMoved] });
    await advance(6);
    assert.ok(Math.hypot(...(await state()).velocity) > speed * 0.9, "actual touch did not move the host body");
    const touchYaw = (await state()).pose[3];
    // Chromium requires naming the ended finger to retain the other contact.
    await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [leftMoved] });
    await page.waitForFunction(() => observed.touchActive.length === 1 && observed.touchActive[0] === 22);
    await advance(6);
    const partial = await state();
    assert.ok(Math.hypot(...partial.velocity) < 0.0001, "ended left stick still moved");
    assert.ok(partial.pose[3] > touchYaw + 0.03, "ending left finger cancelled right-stick look");
    await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    await advance();
    await stopped();
    await writeFile(resolve(artifacts, `${profile}-${compiler}.png`), await page.screenshot({ fullPage: true }));
    assert.deepEqual(errors, [], "browser errors");
    const directory = resolve(root, profile, "dist/variants", compiler);
    return {
      profile,
      compiler,
      speed,
      initial: initial.pose,
      moved: moved.pose,
      checks: [
        "shared prefab and asset room loaded",
        "host movement",
        "release stop",
        "custom bindings",
        "intent customization",
        "multiple run keys",
        "pause interception",
        "camera replacement",
        "pointer capture/look/release",
        "independent touch release",
      ],
      hostSHA256: await digest(resolve(directory, "karty-host.wasm")),
      cartridgeSHA256: await digest(resolve(directory, "game.kart")),
      backend: "SwiftShader",
      worldScale: 0.5,
      antialiasing: false,
      input: "synthesized keyboard, real CDP touch events, synthesized DOM relative mouse movement",
    };
  } finally {
    await browser.close();
  }
}

async function digest(path) {
  return createHash("sha256")
    .update(await readFile(path))
    .digest("hex");
}
