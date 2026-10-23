// Walk the real sample colliders using ordinary keys. Both guest commands and
// accepted host poses are observed; no source, input or output is replaced.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";
import { observeController } from "./fps-browser-evidence.mjs";
import { startVillageServer } from "./willowbrook-server.mjs";

assert.ok(
  process.argv[2] && process.argv[3],
  "Usage: node scripts/check-fps-traversal-web.mjs <crate-room/dist/web> <terrain/dist/web>",
);
const output = resolve(process.env.KARTY_FPS_TRAVERSAL_WEB_OUTPUT || "dist/validation/fps-traversal-web");
await mkdir(output, { recursive: true });
const hash = async (path) =>
  createHash("sha256")
    .update(await readFile(path))
    .digest("hex");
const evidence = (page) => page.evaluate(() => ({ ...globalThis.fpsEvidence }));
const distance = (a, b) => Math.hypot(a.x - b.x, a.y - b.y);
const report = {
  passed: false,
  scope:
    "Actual sample WASM and host physics in Chromium SwiftShader; ordinary synthesized keyboard/V/arrow input (Shift running on Terrain), observed accepted poses and commands, screenshots. Viewport 640x360, render scale 0.25, antialiasing disabled, vegetation draws bypassed; terrain mesh and all physics unchanged. No physical-device or device-FPS qualification.",
  viewport: { width: 640, height: 360 },
  budgets: { totalWallSeconds: 360, stageWallSeconds: 120, crateFrames: 800, terrainFrames: 1800 },
  checks: [],
  errors: [],
};
let browser, deadline;
try {
  browser = await chromium.launch({
    headless: true,
    args: ["--no-sandbox", "--use-gl=angle", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  await Promise.race([
    run(),
    new Promise((_, reject) => {
      deadline = setTimeout(() => reject(new Error("FPS traversal exceeded 360 seconds")), 360000);
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
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
  console.log(JSON.stringify(report, null, 2));
}

async function run() {
  for (const [name, directory, route, frameBudget] of [
    ["crate-room", resolve(process.argv[2]), walkCrateBank, 800],
    ["terrain", resolve(process.argv[3]), walkTerrainSlope, 1800],
  ]) {
    const server = await startVillageServer(directory);
    const context = await browser.newContext({ viewport: report.viewport });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const check = { name, directory, artifacts: {}, stages: [], screenshots: {} };
    report.checks.push(check);
    page.on("pageerror", (error) => report.errors.push(`${name}: ${error.message}`));
    page.on("console", (message) => {
      if (message.type() === "error") report.errors.push(`${name}: ${message.text()}`);
    });
    try {
      const origin = `http://127.0.0.1:${server.address().port}`;
      for (const file of [
        "karty-host.wasm",
        "game.kart",
        ...(await readdir(resolve(directory, "content")))
          .filter((file) => file.endsWith(".kld"))
          .map((file) => `content/${file}`),
      ].sort()) {
        check.artifacts[file] = await hash(resolve(directory, file));
        const response = await fetch(`${origin}/${file}`, { method: "HEAD" });
        assert.equal(response.status, 200);
        assert.equal(response.headers.get("etag"), `"${check.artifacts[file]}"`, "immutable snapshot provenance");
      }
      await page.addInitScript(observeController);
      await page.goto(`${origin}/index.html?debug=0&worldScale=0.25&aa=false&bypassVegetation=true`);
      await page.waitForFunction(
        () => fpsEvidence.pose !== null && fpsEvidence.velocity !== null && fpsEvidence.body !== null,
        null,
        { timeout: 45000 },
      );
      const initial = await stopped(page);
      assert.equal(initial.commandVersion, 16, "current sample uses wire 16");
      assert.equal(initial.body.flags, 3, "sample uses shared capsule motor");
      await recordStage(check, "initial", initial);
      await route(page, check);
      assert.ok((await evidence(page)).frame - initial.frame < frameBudget, `${name}: overall frame budget`);
    } catch (error) {
      check.failure = await evidence(page).catch(() => null);
      await page.screenshot({ path: resolve(output, `${name}-failure.png`) }).catch(() => {});
      throw error;
    } finally {
      await context.close();
      await new Promise((done) => server.close(done));
    }
  }
}

// Release is measured after the final submitted input has traversed host physics.
async function stopped(page) {
  await page.waitForFunction(
    () => fpsEvidence.velocity && Math.hypot(fpsEvidence.velocity.x, fpsEvidence.velocity.y) < 0.001,
  );
  const zero = await evidence(page);
  await page.waitForFunction((frame) => fpsEvidence.frame >= frame + 3, zero.frame);
  const before = await evidence(page);
  await page.waitForFunction((frame) => fpsEvidence.frame >= frame + 20, before.frame);
  const after = await evidence(page);
  assert.equal(after.moving, zero.moving, "released key remains stopped");
  assert.equal(after.yaw, before.yaw, "released look remains stopped");
  assert.ok(distance(after.pose, before.pose) < 0.001, "accepted host XY remains stopped");
  assert.ok(Math.abs(after.pose.z - before.pose.z) < 0.005, "accepted grounded height remains stopped");
  return after;
}

// The only action here is a held platform key; success comes from host feedback.
async function holdUntil(page, key, axis, value, direction, frames) {
  const initial = await evidence(page);
  await page.keyboard.down(key);
  try {
    await page.waitForFunction(
      ({ initial, axis, value, direction, frames }) => {
        if (fpsEvidence.frame - initial > frames)
          throw new Error(`Held key exceeded ${frames} frames before reaching ${axis}=${value}`);
        const coordinate = axis === "yaw" ? fpsEvidence.yaw : fpsEvidence.pose?.[axis];
        return Number.isFinite(coordinate) && direction * (coordinate - value) >= 0;
      },
      { initial: initial.frame, axis, value, direction, frames },
      { timeout: 120000 },
    );
  } finally {
    await page.keyboard.up(key);
  }
  return stopped(page);
}

async function photograph(page, check, stage) {
  const file = `${check.name}-${stage}.png`;
  await page.screenshot({ path: resolve(output, file) });
  check.screenshots[file] = await hash(resolve(output, file));
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
}

// Keep completed evidence visible even while a software-rendered route runs.
async function recordStage(check, name, state) {
  check.stages.push({ name, ...state });
  console.log(
    JSON.stringify({ sample: check.name, stage: name, frame: state.frame, pose: state.pose, yaw: state.yaw }),
  );
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
}

// Several fixed ticks can run before the browser observes a held arrow release.
// Coarse facing is enough: the run verifies its actual corridor continuously.
async function faceTerrainCorridor(page) {
  const facing = await holdUntil(page, "ArrowRight", "yaw", 0, 1, 100);
  assert.ok(facing.yaw >= 0 && facing.yaw < 0.2, "ordinary arrow turn faces the terrain corridor");
  return facing;
}

// Cross-slope contacts deflect a capsule sideways. Ordinary strafe corrections
// keep the run inside the source-verified corridor. Shift uses the sample's
// existing run multiplier to fit this long route within software-render budgets.
async function walkTerrainLeg(page, key, targetY, direction) {
  const initial = await evidence(page);
  const started = Date.now();
  let state = initial;
  let strafe = null;
  let maxDeviation = 0;
  await page.keyboard.down("Shift");
  await page.keyboard.down(key);
  try {
    while (direction * (state.pose.y - targetY) < 0) {
      const remaining = 120000 - (Date.now() - started);
      assert.ok(remaining > 0, "terrain leg exceeded 120 seconds");
      assert.ok(state.frame - initial.frame <= 650, "terrain leg exceeded 650 frames");
      maxDeviation = Math.max(maxDeviation, Math.abs(state.pose.x + 16));
      assert.ok(maxDeviation < 0.6, "observed terrain walk stayed inside the sampled corridor");
      const x = state.pose.x;
      const correction =
        x > -15.85 || (strafe === "a" && x > -16) ? "a" : x < -16.15 || (strafe === "d" && x < -16) ? "d" : null;
      if (correction !== strafe) {
        if (strafe) await page.keyboard.up(strafe);
        if (correction) await page.keyboard.down(correction);
        strafe = correction;
      }
      await page.waitForFunction((frame) => fpsEvidence.frame > frame, state.frame, {
        polling: 5,
        timeout: Math.min(10000, remaining),
      });
      state = await evidence(page);
    }
  } finally {
    await page.keyboard.up(key);
    if (strafe) await page.keyboard.up(strafe);
    await page.keyboard.up("Shift");
  }
  const settled = await stopped(page);
  maxDeviation = Math.max(maxDeviation, Math.abs(state.pose.x + 16), Math.abs(settled.pose.x + 16));
  assert.ok(maxDeviation < 0.6, "completed terrain walk stayed inside the sampled corridor");
  assert.ok(state.frame - initial.frame <= 650, "completed terrain leg stayed within 650 frames");
  return { ...settled, corridorMaxDeviation: maxDeviation, input: { move: key, run: true, steering: "A/D" } };
}

async function walkCrateBank(page, check) {
  const approach = await holdUntil(page, "w", "y", -7.5, 1, 120);
  await recordStage(check, "clear-approach", approach);
  // The east bank starts at X=8 with a .15m lip; top=.7X-5.45.
  // Y=-7.5 stays clear of the authored bumpers and initial crate stacks.
  const ascent = await holdUntil(page, "d", "x", 9.4, 1, 260);
  assert.ok(ascent.pose.z > 2.6 && ascent.pose.z - approach.pose.z > 0.8, "host climbed the bank lip and slope");
  assert.ok(Math.abs(ascent.pose.y - approach.pose.y) < 0.2, "bank route stayed on its clear approach line");
  await recordStage(check, "bank-ascent", ascent);
  await photograph(page, check, "ascent");
  const descent = await holdUntil(page, "a", "x", 7.2, -1, 180);
  assert.ok(Math.abs(descent.pose.z - 1.67) < 0.08, "host descended back to crate-room floor");
  assert.ok(ascent.pose.z - descent.pose.z > 0.8, "bank descent has accepted vertical travel");
  await recordStage(check, "bank-descent", descent);
  await photograph(page, check, "descent");
}

async function walkTerrainSlope(page, check) {
  const previousCamera = (await evidence(page)).camera;
  // V uses the existing cliff-foot landmark; it does not inject a test pose.
  await page.keyboard.press("v", { delay: 100 });
  await page.waitForFunction(
    (camera) => fpsEvidence.camera !== camera && fpsEvidence.pose !== null && fpsEvidence.velocity !== null,
    previousCamera,
  );
  const start = await stopped(page);
  assert.ok(
    Math.abs(start.pose.x + 16) < 0.1 && Math.abs(start.pose.y + 6) < 0.1,
    "ordinary V selected cliff-foot landmark",
  );
  assert.equal(start.body?.flags, 3, "replacement landmark retains shared capsule motor");
  await recordStage(check, "cliff-foot", start);
  const facing = await faceTerrainCorridor(page);
  await recordStage(check, "corridor-heading", facing);
  // Including the .3m capsule footprint, X[-16.9,-15.1], Y[-6.4,14.8]
  // contains no holes and has a steepest triangle of 44.54 degrees.
  const ascent = await walkTerrainLeg(page, "w", 14, 1);
  assert.ok(ascent.pose.z > 6 && ascent.pose.z - start.pose.z > 4, "host climbed the actual steep terrain");
  assert.ok(Math.abs(ascent.pose.x + 16) < 0.6, "terrain route stayed within the sampled corridor");
  await recordStage(check, "terrain-ascent", ascent);
  const descent = await walkTerrainLeg(page, "s", -5.5, -1);
  assert.ok(
    descent.pose.z < 2 && ascent.pose.z - descent.pose.z > 4,
    "host descended the steep terrain back to cliff foot",
  );
  await recordStage(check, "terrain-descent", descent);
  // Preserve both physical results before asking the software renderer for a PNG.
  await photograph(page, check, "descent");
}
