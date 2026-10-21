// Runs the real Vigil cartridge and browser host. Observers never alter commands.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { createServer } from "node:http";
import { mkdir, readFile, realpath, writeFile } from "node:fs/promises";
import { extname, resolve, sep } from "node:path";
import { chromium } from "playwright";

const projectDirectory = await realpath(resolve(process.env.KARTY_VIGIL_PROJECT || "samples/vigil"));
const animation = JSON.parse(await readFile(resolve(projectDirectory, "levels/crypt/walk.generated.json"), "utf8"));
assert.equal(animation.version, 3, "whole-body animation metadata must be regenerated");
assert.equal(animation.walkNames.length, animation.frames * animation.directions);
assert.equal(animation.idleNames.length, animation.directions);
assert.equal(animation.firstIdleTexture, animation.firstWalkTexture + animation.walkNames.length);
assert.equal(animation.rig.length, animation.walkNames.length + animation.directions);
assert.equal(animation.moveSpeed, 4, "regenerate metadata for the brisk default movement");
assert.equal(animation.hastenSpeed, 5.2, "regenerate metadata for hastening");

const materialsOnly = process.argv.includes("--materials-only");
const aggregateOnly = process.argv.includes("--aggregate-only");
const fixture = process.argv.find((arg) => arg.startsWith("--fixture="))?.slice(10);
if (
  fixture &&
  ![
    "input",
    "route",
    "route-gallery",
    "portrait",
    "props",
    "fireball",
    "aim",
    "occlusion",
    "occlusion-wall",
    "occlusion-portrait",
  ].includes(fixture)
)
  throw new Error("invalid Vigil fixture");
if (!materialsOnly && !fixture) {
  // Software-rendered captures and traversal have separate browser lifetimes.
  // Keep every assertion and the existing 120-second deadline per fixture.
  const output = resolve("dist/validation/vigil");
  await mkdir(output, { recursive: true });
  const source = await realpath(
    resolve(process.argv.slice(2).find((arg) => !arg.startsWith("--")) || resolve(projectDirectory, "dist/web")),
  );
  const identity = async (name) =>
    createHash("sha256")
      .update(await readFile(resolve(source, name)))
      .digest("hex");
  const pending = {
    fixture: "combined",
    passed: false,
    cartridgeSha256: await identity("game.kart"),
    hostSha256: await identity("karty-host.wasm"),
    checks: [],
    errors: [],
  };
  await writeFile(resolve(output, "report.json"), JSON.stringify(pending, null, 2) + "\n");
  for (const name of aggregateOnly
    ? []
    : [
        "occlusion",
        "occlusion-wall",
        "occlusion-portrait",
        "fireball",
        "aim",
        "input",
        "route",
        "route-gallery",
        "portrait",
        "props",
      ]) {
    const result = spawnSync(
      process.execPath,
      [fileURLToPath(import.meta.url), ...process.argv.slice(2), "--fixture=" + name],
      { stdio: "inherit" },
    );
    if (result.error) throw result.error;
    if (result.status !== 0) process.exit(result.status ?? 1);
  }
  const input = JSON.parse(await readFile(resolve(output, "input/report.json"), "utf8"));
  const route = JSON.parse(await readFile(resolve(output, "route-report.json"), "utf8"));
  const gallery = JSON.parse(await readFile(resolve(output, "route-gallery/report.json"), "utf8"));
  const portrait = JSON.parse(await readFile(resolve(output, "portrait/report.json"), "utf8"));
  const props = JSON.parse(await readFile(resolve(output, "props/report.json"), "utf8"));
  const aim = JSON.parse(await readFile(resolve(output, "aim/report.json"), "utf8"));
  const fireball = JSON.parse(await readFile(resolve(output, "fireball/report.json"), "utf8"));
  const occlusion = JSON.parse(await readFile(resolve(output, "occlusion/report.json"), "utf8"));
  const occlusionWall = JSON.parse(await readFile(resolve(output, "occlusion-wall/report.json"), "utf8"));
  const occlusionPortrait = JSON.parse(await readFile(resolve(output, "occlusion-portrait/report.json"), "utf8"));
  for (const extra of [aim, gallery, occlusion, occlusionWall, occlusionPortrait]) {
    assert.equal(extra.cartridgeSha256, route.cartridgeSha256);
    assert.equal(extra.hostSha256, route.hostSha256);
  }
  assert.equal(input.cartridgeSha256, route.cartridgeSha256);
  assert.equal(input.hostSha256, route.hostSha256);
  assert.equal(portrait.cartridgeSha256, route.cartridgeSha256);
  assert.equal(portrait.hostSha256, route.hostSha256);
  assert.equal(props.cartridgeSha256, route.cartridgeSha256);
  assert.equal(props.hostSha256, route.hostSha256);
  assert.equal(fireball.cartridgeSha256, route.cartridgeSha256);
  assert.equal(fireball.hostSha256, route.hostSha256);
  assert.equal(route.cartridgeSha256, pending.cartridgeSha256);
  assert.equal(route.hostSha256, pending.hostSha256);
  const report = {
    ...route,
    fixture: "combined",
    deadlineMsPerFixture: route.deadlineMs,
    checks: [
      ...new Set([
        ...input.checks,
        ...route.checks,
        ...gallery.checks,
        ...portrait.checks,
        ...props.checks,
        ...fireball.checks,
        ...aim.checks,
        ...occlusion.checks,
        ...occlusionWall.checks,
        ...occlusionPortrait.checks,
      ]),
    ],
    captures: [
      ...route.captures,
      ...gallery.captures.map((name) => "route-gallery/" + name),
      ...portrait.captures.map((name) => "portrait/" + name),
      ...props.captures.map((name) => "props/" + name),
      ...fireball.captures.map((name) => "fireball/" + name),
      ...occlusion.captures.map((name) => "occlusion/" + name),
      ...occlusionWall.captures.map((name) => "occlusion-wall/" + name),
      ...occlusionPortrait.captures.map((name) => "occlusion-portrait/" + name),
    ],
    fixtures: { input, route, gallery, portrait, props, fireball, aim, occlusion, occlusionPortrait },
    passed:
      input.passed &&
      route.passed &&
      gallery.passed &&
      portrait.passed &&
      props.passed &&
      fireball.passed &&
      aim.passed &&
      occlusion.passed &&
      occlusionWall.passed &&
      occlusionPortrait.passed,
  };
  delete report.deadlineMs;
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
  console.log(JSON.stringify({ passed: report.passed, checks: report.checks }, null, 2));
  process.exit(report.passed ? 0 : 1);
}
const inputOnly = fixture === "input";
const routeGalleryOnly = fixture === "route-gallery";
const routesOnly = fixture === "route" || routeGalleryOnly;
const portraitOnly = fixture === "portrait";
const propsOnly = fixture === "props";
const fireballOnly = fixture === "fireball";
const aimOnly = fixture === "aim";
const occlusionOnly = ["occlusion", "occlusion-wall", "occlusion-portrait"].includes(fixture);
const occlusionWallOnly = fixture === "occlusion-wall";
const occlusionPortraitOnly = fixture === "occlusion-portrait";
// SwiftShader traversal is qualified separately from full-resolution art captures.
const movementScale = 0.25;
const directory = await realpath(
  resolve(process.argv.slice(2).find((arg) => !arg.startsWith("--")) || resolve(projectDirectory, "dist/web")),
);
const phase = process.argv.find((arg) => arg.startsWith("--phase="))?.slice(8) || "current";
if (!/^[a-z0-9-]+$/.test(phase)) throw new Error("invalid material review phase");
const output = resolve(
  "dist/validation/vigil",
  materialsOnly
    ? "materials-" + phase
    : routeGalleryOnly
      ? "route-gallery"
      : inputOnly
        ? "input"
        : portraitOnly
          ? "portrait"
          : propsOnly
            ? "props"
            : fireballOnly || aimOnly
              ? fixture
              : occlusionOnly
                ? fixture
                : "",
);
await mkdir(output, { recursive: true });
const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");
const mime = new Map([
  [".html", "text/html"],
  [".js", "text/javascript"],
  [".wasm", "application/wasm"],
  [".kart", "application/wasm"],
  [".kld", "application/wasm"],
]);
const server = createServer(async (request, response) => {
  try {
    const path = decodeURIComponent(new URL(request.url, "http://localhost").pathname);
    const file = await realpath(resolve(directory, path === "/" ? "index.html" : path.slice(1)));
    if (!file.startsWith(directory + sep)) throw new Error("path outside distribution");
    response.writeHead(200, {
      "Content-Type": mime.get(extname(file)) || "application/octet-stream",
      "Cache-Control": "no-store",
    });
    response.end(await readFile(file));
  } catch {
    response.writeHead(404).end();
  }
});
await new Promise((done, reject) => {
  server.once("error", reject);
  server.listen(0, "127.0.0.1", done);
});
const browser = await chromium.launch({
  headless: true,
  args: ["--no-sandbox", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
});
const deadline = setTimeout(() => {
  void browser.close();
  server.closeAllConnections();
  server.close();
}, 120000);
const report = {
  fixture: materialsOnly ? "materials" : fixture,
  deadlineMs: 120000,
  scope: portraitOnly
    ? `Actual cartridge/host WASM and WebGL on Chromium SwiftShader, 390x844 portrait resized to 1280x720 landscape and 2560x1080 ultrawide. Movement and framebuffer captures use worldScale=${movementScale}; aa=false. Synthesized touch; no physical-device FPS qualification.`
    : `Actual cartridge/host WASM and WebGL on Chromium SwiftShader. Movement uses worldScale=${movementScale}; art captures use worldScale=1; aa=false. Synthesized keyboard, mouse and landscape touch; no physical-device FPS qualification.`,
  cartridgeSha256: hash(await readFile(resolve(directory, "game.kart"))),
  hostSha256: hash(await readFile(resolve(directory, "karty-host.wasm"))),
  errors: [],
  checks: [],
  captures: [],
  passed: false,
};

// Check the opaque interior of the red hood where the nearby column used to
// slice through a camera-facing card. Source texels choose material regions;
// rendered colors only need to retain red dominance, not exact lighting values.
async function hoodOcclusion({ source, screenshot, state, animation }) {
  const decode = async (base64) => {
    const bitmap = await createImageBitmap(
      new Blob([Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))], { type: "image/png" }),
    );
    const image = new OffscreenCanvas(bitmap.width, bitmap.height),
      context = image.getContext("2d");
    context.drawImage(bitmap, 0, 0);
    return {
      width: bitmap.width,
      height: bitmap.height,
      pixels: context.getImageData(0, 0, bitmap.width, bitmap.height).data,
    };
  };
  const texture = await decode(source),
    image = await decode(screenshot),
    scale = 720 / state.height,
    dx = state.position[0] - state.camera[0] - 18,
    dy = state.position[1] - state.camera[1] - 18,
    baseX = 640 + ((dx - dy) / Math.SQRT2) * scale,
    baseY = 360 - (((dx + dy) * 0.5) / Math.SQRT2 + (0.03 * Math.sqrt(3)) / 2) * scale,
    shaftCenterX = 640 + ((7.125 - state.camera[0] - 18 - (9.925 - state.camera[1] - 18)) / Math.SQRT2) * scale,
    samples = new Map();
  let top = texture.height,
    bottom = -1;
  for (let y = 0; y < texture.height; y++)
    for (let x = 0; x < texture.width; x++) {
      if (texture.pixels[(y * texture.width + x) * 4 + 3] >= 128) {
        top = Math.min(top, y);
        bottom = Math.max(bottom, y);
      }
    }
  for (let y = top; y < top + (bottom - top + 1) * 0.35; y++) {
    for (let x = 0; x < texture.width; x++) {
      const i = (y * texture.width + x) * 4,
        [r, g, b, a] = texture.pixels.slice(i, i + 4);
      if (a < 250 || r < 35 || r < g * 1.7 || r < b * 1.7) continue;
      const px = Math.round(baseX + ((x + 0.5) / texture.width - animation.originX) * animation.projectedSize * scale),
        py = Math.round(baseY + ((y + 0.5) / texture.height - animation.originY) * animation.projectedSize * scale),
        at = (py * image.width + px) * 4,
        color = Array.from(image.pixels.slice(at, at + 3));
      // Select the hood interior that actually overlaps the column shaft,
      // independently of atlas canvas size, padding and body registration.
      if (px < shaftCenterX - 0.22 * scale || px > shaftCenterX - 0.1 * scale) continue;
      samples.set(px + ":" + py, {
        pixel: [px, py],
        color,
        retained: color[0] > 8 && color[0] > color[1] * 1.65 && color[0] > color[2] * 1.65,
      });
    }
  }
  const points = [...samples.values()],
    retained = points.filter((point) => point.retained).length;
  return { samples: points.length, retained, fraction: retained / points.length, points };
}

// Project opaque texels from the currently submitted whole-body PNG into the
// actual framebuffer. This works with any facing, pivot and physical viewport.
async function sourceSpriteComparison({ source, disabled, enabled, state }) {
  const decode = async (base64) => {
    const bitmap = await createImageBitmap(
      new Blob([Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))], { type: "image/png" }),
    );
    const canvas = new OffscreenCanvas(bitmap.width, bitmap.height),
      context = canvas.getContext("2d");
    context.drawImage(bitmap, 0, 0);
    const result = {
      width: bitmap.width,
      height: bitmap.height,
      pixels: context.getImageData(0, 0, bitmap.width, bitmap.height).data,
    };
    bitmap.close();
    return result;
  };
  const texture = await decode(source),
    off = await decode(disabled),
    on = await decode(enabled);
  if (off.width !== on.width || off.height !== on.height) throw new Error("occlusion comparison viewport changed");
  const scale = on.height / state.height,
    [x, y, z] = state.heroPosition;
  const dx = x - state.camera[0] - 18,
    dy = y - state.camera[1] - 18;
  const baseX = on.width / 2 + ((dx - dy) / Math.SQRT2) * scale;
  const baseY = on.height / 2 - (((dx + dy) * 0.5) / Math.SQRT2 + z * Math.cos(Math.PI / 6)) * scale;
  const red = (color) => color[0] > 8 && color[0] > color[1] * 1.65 && color[0] > color[2] * 1.65;
  const points = new Map();
  for (let sy = 1; sy < texture.height - 1; sy++)
    for (let sx = 1; sx < texture.width - 1; sx++) {
      const at = (sy * texture.width + sx) * 4;
      if (texture.pixels[at + 3] < 250) continue;
      // Reject antialiased edges; the fixture tests opaque anatomical material.
      if ([-1, 1, -texture.width, texture.width].some((offset) => texture.pixels[at + offset * 4 + 3] < 250)) continue;
      const sourceColor = Array.from(texture.pixels.slice(at, at + 3));
      const px = Math.round(baseX + ((sx + 0.5) / texture.width - state.heroOrigin[0]) * state.heroSize[0] * scale);
      const py = Math.round(
        baseY + ((sy + 0.5) / texture.height - state.heroOrigin[1]) * state.heroSize[1] * Math.cos(Math.PI / 6) * scale,
      );
      if (px < 0 || py < 0 || px >= on.width || py >= on.height) continue;
      const pixel = (py * on.width + px) * 4;
      const a = Array.from(off.pixels.slice(pixel, pixel + 3)),
        b = Array.from(on.pixels.slice(pixel, pixel + 3));
      points.set(px + ":" + py, {
        pixel: [px, py],
        sourceRed: red(sourceColor) && sourceColor[0] > 35 && sy < texture.height * 0.55,
        offRed: red(a),
        onRed: red(b),
        changed: Math.max(...a.map((value, axis) => Math.abs(value - b[axis]))) > 8,
      });
    }
  const values = [...points.values()],
    hood = values.filter((point) => point.sourceRed);
  let changed = 0,
    outsideChanged = 0,
    outsideSamples = 0;
  const left = baseX - state.heroOrigin[0] * state.heroSize[0] * scale - 3;
  const right = left + state.heroSize[0] * scale + 6;
  const top = baseY - state.heroOrigin[1] * state.heroSize[1] * Math.cos(Math.PI / 6) * scale - 3;
  const bottom = top + state.heroSize[1] * Math.cos(Math.PI / 6) * scale + 6;
  for (let py = 0; py < on.height; py++)
    for (let px = 0; px < on.width; px++) {
      const i = (py * on.width + px) * 4;
      const differs =
        Math.abs(off.pixels[i] - on.pixels[i]) > 8 ||
        Math.abs(off.pixels[i + 1] - on.pixels[i + 1]) > 8 ||
        Math.abs(off.pixels[i + 2] - on.pixels[i + 2]) > 8;
      if (differs) changed++;
      if (px < left || px > right || py < top || py > bottom) {
        outsideSamples++;
        if (differs) outsideChanged++;
      }
    }
  return {
    width: on.width,
    height: on.height,
    samples: values.length,
    silhouetteChanged: values.filter((point) => point.changed).length,
    silhouetteUnchanged: values.filter((point) => !point.changed).length,
    hoodSamples: hood.length,
    hoodOff: hood.filter((point) => point.offRed).length,
    hoodOn: hood.filter((point) => point.onRed).length,
    hoodRestored: hood.filter((point) => !point.offRed && point.onRed).length,
    hoodStillOccluded: hood.filter((point) => !point.offRed && !point.onRed).length,
    changed,
    outsideChanged,
    outsideSamples,
    outsideUnchangedFraction: 1 - outsideChanged / outsideSamples,
  };
}

function observe(animation) {
  globalThis.vigil = {
    animation,
    hero: 0,
    root: 0,
    position: null,
    heroPosition: null,
    heroOrigin: null,
    heroFacing: 0,
    rootFacing: 0,
    camera: null,
    height: 14,
    transforms: 0,
    positions: [],
    movementFrames: [],
    heroTexture: 0,
    pose: null,
    spriteFrames: 0,
    spriteViolations: [],
    spriteFacingCounts: {},
    spriteFacingViolations: [],
    textures: [],
    textureFrames: [],
    masks: {},
    curtainReady: false,
    maximumBatch: 0,
    pathRequests: [],
    pathResults: [],
    lights: {},
    fireballs: {},
    fireballCasts: 0,
    maximumFireballs: 0,
    projectileSprites: {},
    maximumProjectileSprites: 0,
    fireballImpacts: [],
    success: false,
  };
  let placementCheckPending = false;
  // A guest update may flush several complete protocol batches. Check the
  // cross-actor root/painting invariant after its synchronous submissions,
  // when both transforms have reached the state that can actually render.
  const checkPlacement = () => {
    placementCheckPending = false;
    const state = globalThis.vigil;
    if (!state.hero || !state.root) return;
    const pose = state.pose;
    const data = animation.rig[pose],
      expected = [state.position[0] + data.offsetX, state.position[1] + data.offsetY, 0.03 + data.offsetZ];
    state.spriteFrames++;
    if (
      (state.heroFacing !== 2 ||
        state.rootFacing !== 4 ||
        Math.hypot(...expected.map((value, axis) => value - state.heroPosition[axis])) > 0.0001 ||
        Math.abs(state.heroOrigin[0] - data.originX) > 0.00001 ||
        Math.abs(state.heroOrigin[1] - data.originY) > 0.00001) &&
      state.spriteViolations.length < 16
    )
      state.spriteViolations.push({
        pose,
        expected,
        actual: state.heroPosition.slice(),
        origin: state.heroOrigin.slice(),
        data,
      });
  };
  let submit;
  Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
    configurable: true,
    get: () => submit,
    set(original) {
      submit = (batch) => {
        const state = globalThis.vigil,
          b = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
        state.maximumBatch = Math.max(state.maximumBatch, batch.byteLength);
        let o = 24;
        for (let i = 0; i < b.getUint32(16, true); i++) {
          const tag = b.getUint8(o),
            length = b.getUint16(o + 1, true),
            p = o + 3;
          if (
            tag === 20 &&
            new TextDecoder().decode(batch.subarray(p + 11, p + 11 + b.getUint16(p + 9, true))) === "3 / 3"
          )
            state.success = true;
          if (tag === 57) {
            if (length !== 56) throw new Error("invalid FindPath request length");
            state.pathRequests.push({
              frame: Number(b.getBigUint64(8, true)),
              request: b.getUint32(p, true),
              handle: b.getUint32(p + 4, true),
              from: [b.getFloat32(p + 16, true), b.getFloat32(p + 20, true)],
              to: [b.getFloat32(p + 24, true), b.getFloat32(p + 28, true)],
            });
          }
          if (tag === 27 || tag === 28) {
            const s = p + (tag === 27 ? 8 : 4);
            state.camera = [b.getFloat32(s + 4, true), b.getFloat32(s + 8, true)];
            state.height = b.getFloat32(s + 28, true);
          }
          if (tag === 29 || tag === 31) {
            const sprite = p + (tag === 29 ? 44 : 4),
              facing = b.getUint8(sprite + 4),
              asset = b.getUint32(sprite, true),
              entity = b.getUint32(p, true);
            state.spriteFacingCounts[facing] = (state.spriteFacingCounts[facing] || 0) + 1;
            if (facing !== 2 && facing !== 4) state.spriteFacingViolations.push({ tag, entity, asset, facing });
            if (asset >= animation.firstWalkTexture && asset < animation.firstIdleTexture + animation.directions) {
              state.hero = entity;
              state.heroTexture = asset;
              state.heroFacing = facing;
              state.heroOrigin = [b.getFloat32(sprite + 16, true), b.getFloat32(sprite + 20, true)];
              state.heroSize = [b.getFloat32(sprite + 8, true), b.getFloat32(sprite + 12, true)];
              if (tag === 29)
                state.heroPosition = [
                  b.getFloat32(p + 8, true),
                  b.getFloat32(p + 12, true),
                  b.getFloat32(p + 16, true),
                ];
            }
            if (asset === animation.rootTexture) {
              state.root = entity;
              state.rootFacing = facing;
              if (tag === 29) state.position = [b.getFloat32(p + 8, true), b.getFloat32(p + 12, true)];
            }
            const firstFlame = animation.fireTextures.first,
              lastFlame = animation.fireTextures.last;
            if (
              asset >= firstFlame &&
              asset <= lastFlame &&
              (state.projectileSprites[entity] || (tag === 29 && Math.abs(b.getFloat32(p + 16, true) - 0.95) < 0.001))
            ) {
              const effect = (state.projectileSprites[entity] ||= {});
              effect.asset = asset;
              effect.size = [b.getFloat32(sprite + 8, true), b.getFloat32(sprite + 12, true)];
              if (tag === 29)
                effect.position = [b.getFloat32(p + 8, true), b.getFloat32(p + 12, true), b.getFloat32(p + 16, true)];
              state.maximumProjectileSprites = Math.max(
                state.maximumProjectileSprites,
                Object.keys(state.projectileSprites).length,
              );
            }
          }
          if (tag === 30 && state.projectileSprites[b.getUint32(p, true)])
            state.projectileSprites[b.getUint32(p, true)].position = [4, 8, 12].map((offset) =>
              b.getFloat32(p + offset, true),
            );
          if (tag === 30 && b.getUint32(p, true) === state.hero)
            state.heroPosition = [b.getFloat32(p + 4, true), b.getFloat32(p + 8, true), b.getFloat32(p + 12, true)];
          if (tag === 30 && b.getUint32(p, true) === state.root) {
            state.position = [b.getFloat32(p + 4, true), b.getFloat32(p + 8, true)];
            state.transforms++;
            state.positions.push(state.position);
          }
          const id = b.getUint32(p, true);
          if (tag === 10 && [1280, 7424].includes(b.getFloat32(p + 5, true))) state.curtain = id;
          if (tag === 7 && b.getInt32(p + 4, true) === -9) state.masks[id] = { alpha: 0 };
          if (tag === 6) {
            if (state.masks[id]) state.masks[id].alpha = b.getFloat32(p + 4, true);
            if (id === state.curtain) state.curtainReady = b.getFloat32(p + 4, true) === 0;
          }
          if (tag === 3 && state.masks[id])
            Object.assign(state.masks[id], {
              x: b.getFloat32(p + 4, true),
              y: b.getFloat32(p + 8, true),
              w: b.getFloat32(p + 16, true) * 2,
              h: b.getFloat32(p + 20, true) * 2,
            });
          if (tag === 37 || tag === 38) {
            const s = p + (tag === 37 ? 8 : 4);
            state.lights[b.getUint32(p, true)] = b.getUint8(s + 1) === 1;
            if (tag === 37 && Math.abs(b.getFloat32(s + 40, true) - 3.8) < 0.001 && b.getFloat32(s + 28, true) > 3) {
              state.fireballs[id] = { positions: [] };
              state.fireballCasts++;
              state.maximumFireballs = Math.max(state.maximumFireballs, Object.keys(state.fireballs).length);
            }
            if (state.fireballs[id]) {
              const bolt = state.fireballs[id];
              bolt.position = [8, 12, 16].map((offset) => b.getFloat32(s + offset, true));
              bolt.color = [28, 32, 36].map((offset) => b.getFloat32(s + offset, true));
              bolt.radius = b.getFloat32(s + 40, true);
              bolt.positions.push(bolt.position.slice());
              if (bolt.radius > 4.21 && !bolt.impacted) {
                bolt.impacted = true;
                state.fireballImpacts.push({
                  position: bolt.position.slice(),
                  color: bolt.color.slice(),
                  radius: bolt.radius,
                });
              }
            }
          }
          if (tag === 2 || tag === 34) {
            delete state.fireballs[id];
            delete state.lights[id];
            delete state.projectileSprites[id];
          }
          o += 3 + length;
        }
        if (state.hero && state.root) {
          const pose =
            state.heroTexture >= animation.firstIdleTexture
              ? animation.walkNames.length + state.heroTexture - animation.firstIdleTexture
              : state.heroTexture - animation.firstWalkTexture;
          if (pose !== state.pose) {
            state.pose = pose;
            state.textures.push(pose);
            state.textureFrames.push({
              texture: pose,
              asset: state.heroTexture,
              frame: Number(globalThis.kartyFrame ?? document.documentElement.dataset.kartyFrame),
            });
          }
          if (!placementCheckPending) {
            placementCheckPending = true;
            queueMicrotask(checkPlacement);
          }
        }
        if (state.position)
          state.movementFrames.push({
            frame: Number(globalThis.kartyFrame ?? document.documentElement.dataset.kartyFrame),
            position: state.position.slice(),
            texture: state.pose,
            heroPosition: state.heroPosition?.slice(),
            heroOrigin: state.heroOrigin?.slice(),
          });
        return original(batch);
      };
    },
  });
  let update;
  Object.defineProperty(globalThis, "kartyClientUpdate", {
    configurable: true,
    get: () => update,
    set(original) {
      update = (frame, batch) => {
        const b = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
        let offset = 24;
        for (let i = 0; i < b.getUint32(16, true); i++) {
          const tag = b.getUint8(offset),
            length = b.getUint16(offset + 1, true),
            p = offset + 3;
          if (tag === 17) {
            vigil.pathResults.push({
              request: b.getUint32(p, true),
              handle: b.getUint32(p + 4, true),
              status: b.getUint8(p + 8),
              count: b.getUint16(p + 10, true),
              expanded: b.getUint16(p + 12, true),
            });
          }
          offset += 3 + length;
        }
        return original(frame, batch);
      };
    },
  });
}

try {
  const page = await browser.newPage({
    viewport: portraitOnly ? { width: 390, height: 844 } : { width: 1280, height: 720 },
  });
  page.on("pageerror", (error) => report.errors.push(String(error)));
  page.on("console", (message) => {
    if (message.type() === "error") report.errors.push(message.text());
  });
  // Inspect source PNGs before starting the active software-rendered game.
  // Their alpha readback should not contend with WebGL frame advancement.
  if (inputOnly) {
    const started = Date.now();
    const poseImages = [];
    for (const name of [...animation.walkNames, ...animation.idleNames])
      poseImages.push(
        (await readFile(resolve(projectDirectory, animation.textureDirectory, name + ".png"))).toString("base64"),
      );
    report.groundAlpha = await page.evaluate(
      async ({ images, animation }) => {
        const results = [];
        for (let pose = 0; pose < images.length; pose++) {
          const bitmap = await createImageBitmap(
            new Blob([Uint8Array.from(atob(images[pose]), (c) => c.charCodeAt(0))], { type: "image/png" }),
          );
          const canvas = new OffscreenCanvas(bitmap.width, bitmap.height),
            context = canvas.getContext("2d", { willReadFrequently: true });
          context.drawImage(bitmap, 0, 0);
          const pixels = context.getImageData(0, 0, bitmap.width, bitmap.height).data;
          let bottom = -1;
          for (let y = 0; y < bitmap.height; y++)
            for (let x = 0; x < bitmap.width; x++)
              if (pixels[(y * bitmap.width + x) * 4 + 3] >= 128) bottom = Math.max(bottom, y + 1);
          const data = animation.rig[pose],
            size = animation.projectedSize,
            minWorldZ = 0.03 + data.offsetZ + ((data.originY - bottom / bitmap.height) * size) / Math.cos(Math.PI / 6),
            shiftX = (data.offsetX - data.offsetY) / Math.SQRT2 / size + animation.originX - data.originX,
            shiftY =
              ((-0.5 * (data.offsetX + data.offsetY)) / Math.SQRT2 - data.offsetZ * Math.cos(Math.PI / 6)) / size +
              animation.originY -
              data.originY;
          results.push({ pose, bottom, minWorldZ, shiftX, shiftY });
          bitmap.close();
        }
        return results;
      },
      { images: poseImages, animation },
    );
    for (const ground of report.groundAlpha) {
      assert.ok(
        ground.bottom > 0 && Math.abs(ground.minWorldZ - 0.03) < 0.001,
        `whole-body opaque sole does not retain its ground plane: ${JSON.stringify(ground)}`,
      );
      assert.ok(
        Math.abs(ground.shiftX) < 0.00001 && Math.abs(ground.shiftY) < 0.00001,
        `whole-body sole anchor changed the registered image: ${JSON.stringify(ground)}`,
      );
    }
    report.checks.push(
      "all whole-body PNG alpha bounds stay above the floor with a grounded sole and preserved root projection",
    );
    report.sourceAlphaMs = Date.now() - started;
  }
  await page.addInitScript(observe, animation);
  await page.goto(`http://127.0.0.1:${server.address().port}/?worldScale=${movementScale}&aa=false&debug=0`);
  await page.waitForFunction(
    () =>
      globalThis.vigil?.hero &&
      vigil.curtainReady &&
      globalThis.kartyWorldRenderer?.stats().cameras.some((c) => c.device?.frames >= 5),
    null,
    { timeout: 30000 },
  );
  const canvas = page.locator("canvas");
  await canvas.evaluate((c) => {
    c.tabIndex = 0;
    c.focus();
  });
  const frame = () => page.evaluate(() => Number(globalThis.kartyFrame ?? document.documentElement.dataset.kartyFrame));
  const frames = async (count) => {
    const start = await frame();
    await page.waitForFunction(
      ({ start, count }) =>
        Number(globalThis.kartyFrame ?? document.documentElement.dataset.kartyFrame) >= start + count,
      { start, count },
      { timeout: 15000 },
    );
  };
  const press = async (key, count = 6) => {
    await page.keyboard.down(key);
    await frames(count);
    await page.keyboard.up(key);
    await frames(4);
  };
  if (aimOnly) {
    const cases = [["w"], ["w", "d"], ["d"], ["s", "d"], ["s"], ["s", "a"], ["a"], ["w", "a"]];
    report.aimDirections = [];
    const project = ([x, y]) => [(x - y) / Math.SQRT2, (-0.5 * (x + y)) / Math.SQRT2];
    for (const [index, keys] of cases.entries()) {
      for (const key of keys) await page.keyboard.down(key);
      await frames(6);
      const movement = await page.evaluate(() => {
        const last = vigil.positions.at(-1),
          previous = vigil.positions.at(-2);
        return last.map((value, axis) => value - previous[axis]);
      });
      const standing = index % 2 === 0;
      if (standing) {
        for (const key of keys) await page.keyboard.up(key);
        await frames(4);
      }
      const previousBolts = await page.evaluate(() => Object.keys(vigil.fireballs));
      await page.keyboard.down("f");
      await frames(4);
      await page.keyboard.up("f");
      if (!standing) for (const key of keys) await page.keyboard.up(key);
      await frames(4);
      const bolt = await page.evaluate((previous) => {
        const entry = Object.entries(vigil.fireballs).find(([id]) => !previous.includes(id));
        return entry?.[1];
      }, previousBolts);
      assert.ok(bolt && !bolt.impacted, `no travelling fireball for ${keys}`);
      const start = bolt.positions[0];
      const end = bolt.positions.find((position) => Math.hypot(position[0] - start[0], position[1] - start[1]) > 0.05);
      assert.ok(end, `no projectile travel observed for ${keys}`);
      const projectile = end.slice(0, 2).map((value, axis) => value - start[axis]);
      const a = project(movement),
        b = project(projectile);
      assert.ok(Math.hypot(...a) > 0.001 && Math.hypot(...b) > 0.001, `empty direction sample for ${keys}`);
      const errorDegrees = (Math.abs(Math.atan2(a[0] * b[1] - a[1] * b[0], a[0] * b[0] + a[1] * b[1])) * 180) / Math.PI;
      assert.ok(errorDegrees < 0.1, `${keys} projectile deviated ${errorDegrees} degrees from accepted movement`);
      report.aimDirections.push({ keys, standing, movement, projectile, errorDegrees });
      // The next cast must follow the ordinary eighteen-update spell cooldown.
      await frames(6);
    }
    report.checks.push(
      "actual WASM fireball trajectories match all eight accepted keyboard headings, both moving and after release",
    );
  } else if (occlusionOnly) {
    const project = (goal) =>
      page.evaluate(([x, y]) => {
        const rect = document.querySelector("canvas").getBoundingClientRect(),
          [cx, cy] = vigil.camera;
        const dx = x - cx - 18,
          dy = y - cy - 18,
          scale = rect.height / vigil.height;
        return [
          rect.left + rect.width / 2 + ((dx - dy) / Math.SQRT2) * scale,
          rect.top + rect.height / 2 - (((dx + dy) * 0.5) / Math.SQRT2) * scale,
        ];
      }, goal);
    const walk = async (goal) => {
      const pixel = await project(goal);
      const viewport = page.viewportSize();
      assert.ok(
        pixel[0] > 5 && pixel[0] < viewport.width - 5 && pixel[1] > 110 && pixel[1] < viewport.height - 120,
        `occlusion route covered/offscreen: ${goal} -> ${pixel}`,
      );
      await page.keyboard.down("Shift");
      await page.mouse.move(...pixel);
      await page.mouse.down();
      await frames(3);
      await page.mouse.up();
      try {
        await page.waitForFunction(([x, y]) => Math.hypot(vigil.position[0] - x, vigil.position[1] - y) < 0.1, goal, {
          timeout: 15000,
        });
      } catch (error) {
        const state = await page.evaluate(() => ({
          position: vigil.position,
          camera: vigil.camera,
          lastMovement: vigil.movementFrames.slice(-5),
        }));
        console.log("Occlusion route failure", JSON.stringify({ goal, pixel, state }));
        await page.screenshot({ path: resolve(output, "route-failure.png") });
        throw error;
      }
      await page.keyboard.up("Shift");
      await frames(5);
    };
    const pair = async (name, requireHood) => {
      await page.waitForFunction(
        () => {
          // Framing now settles at a dead-zone boundary rather than forcing
          // exact centering. Retain a stable view for the paired pixel capture.
          const dx = vigil.position[0] + 1.6 - vigil.camera[0] - 18;
          const dy = vigil.position[1] + 1.6 - vigil.camera[1] - 18;
          const right = (dx - dy) / Math.SQRT2;
          const down = -(dx + dy) / (2 * Math.SQRT2);
          const inside =
            Math.abs(right) <= (0.025 * vigil.height * 1280) / 720 / 2 + 0.0003 &&
            Math.abs(down) <= (0.04 * vigil.height) / 2 + 0.0003;
          const previous = vigil.framingPrevious;
          const stable =
            previous && Math.abs(previous[0] - vigil.camera[0]) + Math.abs(previous[1] - vigil.camera[1]) < 0.00001;
          vigil.framingPrevious = vigil.camera.slice();
          vigil.framingStable = inside && stable ? (vigil.framingStable || 0) + 1 : 0;
          return vigil.framingStable >= 3;
        },
        null,
        { timeout: 15000 },
      );
      const reducedSize = occlusionPortraitOnly
        ? await page.evaluate(() => {
            const camera = kartyWorldRenderer.stats().cameras[0];
            return [camera.worldWidth, camera.worldHeight];
          })
        : null;
      assert.equal(
        await page.evaluate(() => kartyWorldRenderer.setOptions({ worldScale: 1, bypassOcclusion: true })),
        null,
      );
      // The snapshot waits for the completed draw in requestAnimationFrame;
      // another simulation update at full software-rendered scale is redundant.
      await frames(1);
      const state = await page.evaluate(() => ({
        position: vigil.position.slice(),
        heroPosition: vigil.heroPosition.slice(),
        heroOrigin: vigil.heroOrigin.slice(),
        heroSize: vigil.heroSize.slice(),
        camera: vigil.camera.slice(),
        height: vigil.height,
        pose: vigil.pose,
      }));
      const snapshot = () =>
        page.evaluate(
          () =>
            new Promise((resolve) =>
              requestAnimationFrame(() =>
                resolve({
                  pixels: document.querySelector("canvas").toDataURL("image/png"),
                  camera: kartyWorldRenderer.stats().cameras[0],
                  root: vigil.position.slice(),
                  hero: vigil.heroPosition.slice(),
                }),
              ),
            ),
        );
      const off = await snapshot();
      // Portrait's CSS 390x844 viewport fits a logical 332x720 world target.
      // Verify full scale against its actual quarter-scale target, avoiding
      // an incorrect equality between CSS pixels and logical render pixels.
      const expectedSize = reducedSize
        ? reducedSize.map((value) => value * 4)
        : [page.viewportSize().width, page.viewportSize().height];
      const rounding = reducedSize ? 3 : 0;
      assert.ok(Math.abs(off.camera.worldWidth - expectedSize[0]) <= rounding, "disabled capture lost full scale");
      assert.ok(Math.abs(off.camera.worldHeight - expectedSize[1]) <= rounding);
      assert.equal(off.camera.occlusionFocus[3], 0, "occlusion bypass did not disable CPU focus selection");
      assert.equal(await page.evaluate(() => kartyWorldRenderer.setOptions({ bypassOcclusion: false })), null);
      await frames(1);
      const on = await snapshot();
      assert.ok(Math.abs(on.camera.worldWidth - expectedSize[0]) <= rounding, "enabled capture lost full scale");
      assert.ok(Math.abs(on.camera.worldHeight - expectedSize[1]) <= rounding);
      assert.deepEqual(on.root, off.root, "occlusion diagnostic changed the physical player transform");
      assert.deepEqual(on.hero, off.hero, "occlusion diagnostic changed the submitted hero painting");
      const focus = on.camera.occlusionFocus,
        shape = on.camera.occlusionShape;
      const expectedFocus = [
        state.heroPosition[0],
        state.heroPosition[1],
        state.heroPosition[2] + (state.heroOrigin[1] - 0.5) * state.heroSize[1],
        1,
      ];
      assert.ok(
        focus.every((value, axis) => Math.abs(value - expectedFocus[axis]) < 0.001),
        `occlusion focus detached from actual sprite model: ${focus} vs ${expectedFocus}`,
      );
      const expectedShape = [
        Math.max(0.35, Math.min(2, state.heroSize[0] * 0.4)),
        Math.max(0.45, Math.min(2.5, state.heroSize[1] * Math.cos(Math.PI / 6) * 0.52)),
        state.heroPosition[2] + 0.04,
        0.25,
      ];
      assert.ok(
        shape.every((value, axis) => Math.abs(value - expectedShape[axis]) < 0.001),
        `occlusion ellipse detached from current sprite: ${shape} vs ${expectedShape}`,
      );
      const sourceName =
        state.pose < animation.walkNames.length
          ? animation.walkNames[state.pose]
          : animation.idleNames[state.pose - animation.walkNames.length];
      const values = await page.evaluate(sourceSpriteComparison, {
        source: (await readFile(resolve(projectDirectory, animation.textureDirectory, sourceName + ".png"))).toString(
          "base64",
        ),
        disabled: off.pixels.split(",")[1],
        enabled: on.pixels.split(",")[1],
        state,
      });
      for (const [suffix, capture] of [
        ["disabled", off],
        ["enabled", on],
      ]) {
        const file = name + "-" + suffix + ".png";
        await writeFile(resolve(output, file), Buffer.from(capture.pixels.split(",")[1], "base64"));
        report.captures.push(file);
      }
      (report.occluders ??= []).push({ name, state, focus, shape, ...values });
      assert.ok(
        values.samples >= 40 && values.silhouetteChanged >= 8,
        `${name}: reveal did not expose source-silhouette pixels: ${JSON.stringify(values)}`,
      );
      assert.ok(values.silhouetteUnchanged >= 8, `${name}: reveal removed every sampled foreground/hero pixel`);
      assert.ok(values.outsideUnchangedFraction > 0.99, `${name}: geometry outside the local focus changed`);
      if (requireHood) {
        assert.ok(
          values.hoodSamples >= 8 && values.hoodRestored >= 3 && values.hoodOn > values.hoodOff,
          `${name}: reveal did not restore opaque red source hood pixels: ${JSON.stringify(values)}`,
        );
        assert.ok(values.hoodStillOccluded >= 1, `${name}: foreground geometry lost all partial hood coverage`);
      }
      assert.equal(await page.evaluate(() => kartyWorldRenderer.setOptions({ worldScale: 0.25 })), null);
      await frames(1);
    };
    if (!occlusionWallOnly) {
      if (!occlusionPortraitOnly) {
        await walk([10.75, 5.75]);
        await walk([8.75, 6.75]);
        await pair("pew", false);
      }
      // Use authored navigation cell centers for the precise endpoint check.
      await walk([11.75, 8.25]);
      await walk([10.75, 8.75]);
      await walk([8.25, 10.75]);
      if (occlusionPortraitOnly) {
        await page.setViewportSize({ width: 390, height: 844 });
        await frames(5);
      }
      await pair(occlusionPortraitOnly ? "portrait-column" : "column", true);
    }
    if (occlusionWallOnly) {
      await press("r");
      await walk([6.75, 4.25]);
      await page.keyboard.down("a");
      await page.keyboard.down("s");
      await frames(24);
      await page.keyboard.up("a");
      await page.keyboard.up("s");
      await frames(4);
      assert.ok(
        Math.abs((await page.evaluate(() => vigil.position))[0] - 6.28) < 0.04,
        "low-wall fixture did not reach wall clearance",
      );
      await pair("low-wall", false);
    }
    report.checks.push(
      occlusionPortraitOnly
        ? "390x844 portrait occluder fade follows the same submitted upright sprite focus and restores source hood pixels without moving the player"
        : occlusionWallOnly
          ? "ordinary movement near a low wall reveals source-silhouette pixels while retaining partial local geometry and unchanged physical transforms"
          : "ordinary routes behind pew and column reveal source-silhouette pixels while retaining partial local geometry and unchanged physical transforms",
    );
    assert.deepEqual(await page.evaluate(() => vigil.spriteViolations), []);
  } else if (fireballOnly) {
    const active = () => page.evaluate(() => Object.keys(vigil.fireballs).length);
    const capture = async (name) => {
      const data = await page.evaluate(
        () =>
          new Promise((done) =>
            requestAnimationFrame(() => {
              const canvas = document.querySelector("canvas");
              done(canvas.toDataURL("image/png"));
            }),
          ),
      );
      await writeFile(resolve(output, name + ".png"), Buffer.from(data.split(",")[1], "base64"));
      report.captures.push(name + ".png");
      return data;
    };
    assert.equal(await page.evaluate(() => kartyWorldRenderer.setOptions({ worldScale: 1 })), null);
    await frames(2);
    const before = await capture("before-cast");
    const root = await page.evaluate(() => vigil.position);
    // Observe the first collision while normal updates continue. Capturing the
    // canvas and state together avoids missing this short effect during PNG IO.
    await page.evaluate(() => {
      globalThis.vigilImpactReview = { samples: [], until: performance.now() + 30000 };
      const collect = () => {
        const review = globalThis.vigilImpactReview;
        const travelling = Object.values(vigil.fireballs).find(
          (value) => !value.impacted && value.positions.length >= 8,
        );
        if (!review.moving && travelling) {
          // Preserve the travelling core with its pixels. Reading live sprites
          // after asynchronous PNG IO can instead observe the later explosion.
          review.moving = {
            bolt: structuredClone(travelling),
            sprites: structuredClone(vigil.projectileSprites),
            pixels: document.querySelector("canvas").toDataURL("image/png"),
          };
        }
        const bolt = Object.values(vigil.fireballs).find((value) => value.impacted);
        const tick = bolt?.positions.length;
        if (
          bolt &&
          (review.samples.length === 0 || tick >= review.samples[0].tick + [0, 3, 12][review.samples.length])
        ) {
          const canvas = document.querySelector("canvas"),
            [cx, cy] = vigil.camera,
            [x, y, z] = bolt.position,
            scale = 720 / vigil.height,
            dx = x - cx - 18,
            dy = y - cy - 18,
            px = 640 + ((dx - dy) / Math.SQRT2) * scale,
            py = 360 - (((dx + dy) * 0.5) / Math.SQRT2 + z * Math.cos(Math.PI / 6)) * scale;
          const crop = document.createElement("canvas");
          crop.width = crop.height = 200;
          crop.getContext("2d").drawImage(canvas, Math.round(px - 100), Math.round(py - 100), 200, 200, 0, 0, 200, 200);
          review.samples.push({
            tick,
            bolt: structuredClone(bolt),
            sprites: structuredClone(vigil.projectileSprites),
            pixels: crop.toDataURL("image/png"),
          });
        }
        if (review.samples.length < 3 && performance.now() < review.until) requestAnimationFrame(collect);
      };
      requestAnimationFrame(collect);
    });
    await page.keyboard.down("f");
    await frames(10);
    assert.equal(await active(), 1);
    const moving = await page.evaluate(() => globalThis.vigilImpactReview.moving);
    assert.ok(moving && !moving.bolt.impacted, "observer missed the travelling fireball phase");
    const lit = moving.pixels;
    await writeFile(resolve(output, "moving-fireball.png"), Buffer.from(lit.split(",")[1], "base64"));
    report.captures.push("moving-fireball.png");
    report.moving = moving.bolt;
    report.movingEffects = Object.values(moving.sprites);
    assert.ok(report.moving.positions.length >= 8);
    assert.ok(Math.hypot(report.moving.position[0] - root[0], report.moving.position[1] - root[1]) > 0.8);
    const floorLight = await page.evaluate(
      async ({ before, lit }) => {
        const points = [
          [12, 6.3],
          [12.5, 6],
          [13.5, 5.5],
          [11, 5.5],
          [14, 4.5],
        ];
        const sample = async (data) => {
          const blob = await (await fetch(data)).blob(),
            bitmap = await createImageBitmap(blob);
          const image = new OffscreenCanvas(bitmap.width, bitmap.height),
            context = image.getContext("2d");
          context.drawImage(bitmap, 0, 0);
          bitmap.close();
          const [cx, cy] = vigil.camera,
            scale = 720 / vigil.height;
          return points.map(([x, y]) => {
            const px = 640 + ((x - cx - 18 - y + cy + 18) / Math.SQRT2) * scale;
            const py = 360 - ((x - cx - 18 + y - cy - 18) / Math.SQRT2) * 0.5 * scale;
            const patch = context.getImageData(Math.round(px) - 2, Math.round(py) - 2, 5, 5).data;
            let red = 0;
            for (let i = 0; i < patch.length; i += 4) red += patch[i];
            return red / 25;
          });
        };
        return { points, before: await sample(before), lit: await sample(lit) };
      },
      { before, lit },
    );
    report.floorLight = floorLight;
    assert.ok(
      floorLight.lit.some((value, i) => value > floorLight.before[i] + 8),
      "moving point light did not brighten floor away from its sprite",
    );
    await page.evaluate((scale) => kartyWorldRenderer.setOptions({ worldScale: scale }), movementScale);
    await frames(30);
    assert.equal(await page.evaluate(() => vigil.fireballCasts), 1, "held F repeated casts");
    await page.keyboard.up("f");
    await frames(65);
    assert.equal(await active(), 0, "expired projectile light leaked");
    assert.equal(
      await page.evaluate(() => Object.keys(vigil.projectileSprites).length),
      0,
      "expired projectile sprites leaked",
    );
    const impact = await page.evaluate(() => ({
      impacts: vigil.fireballImpacts,
      samples: globalThis.vigilImpactReview.samples,
    }));
    assert.equal(impact.impacts.length, 1, "obstacle collision did not explode exactly once");
    assert.equal(impact.samples.length, 3, "contact, flash and smoke phases were not captured");
    assert.ok(
      impact.impacts[0].color[0] > 6 && impact.impacts[0].radius > 5,
      "impact light did not pulse brighter and wider",
    );
    for (const [i, sample] of impact.samples.entries()) {
      const sprites = Object.values(sample.sprites);
      assert.equal(sprites.length, 9, "impact did not retain three burst cards and six bounded sparks");
      assert.ok(
        sprites.every((sprite) => sprite.position[2] - sprite.size[1] / 2 >= 0.039),
        "impact sprite crossed the floor",
      );
      const name = ["impact-contact.png", "impact-burst.png", "impact-sparks.png"][i];
      await writeFile(resolve(output, name), Buffer.from(sample.pixels.split(",")[1], "base64"));
      delete sample.pixels;
      report.captures.push(name);
    }
    const spread = (sample) =>
      Math.max(
        ...Object.values(sample.sprites).map((sprite) =>
          Math.hypot(sprite.position[0] - sample.bolt.position[0], sprite.position[1] - sample.bolt.position[1]),
        ),
      );
    assert.ok(spread(impact.samples[2]) > spread(impact.samples[0]) + 0.2, "impact sparks did not travel outward");
    const core = report.movingEffects.find((sprite) => sprite.asset === animation.fireTextures.core);
    const flashSizes = impact.samples.flatMap((sample) =>
      Object.values(sample.sprites)
        .filter((sprite) => sprite.asset === animation.fireTextures.flash)
        .map((sprite) => sprite.size[0]),
    );
    assert.ok(
      core && Math.max(...flashSizes) > core.size[0] * 1.5,
      "impact flash did not expand beyond the compact travelling core",
    );
    const smoke = Object.values(impact.samples[2].sprites).filter(
      (sprite) => sprite.asset === animation.fireTextures.smoke || sprite.asset === animation.fireTextures.smoke + 1,
    );
    assert.equal(smoke.length, 2, "late impact did not transition to two rising smoke wisps");
    assert.ok(
      smoke.every((sprite) => sprite.position[2] > impact.samples[0].bolt.position[2]),
      "smoke did not rise above the impact",
    );
    assert.ok(
      Math.hypot(
        ...impact.samples[0].bolt.position.map((value, axis) => value - impact.samples[1].bolt.position[axis]),
      ) < 0.0001,
      "collision burst moved through the obstruction",
    );
    report.impact = impact;
    assert.deepEqual(await page.evaluate(() => vigil.position), root, "casting moved the player");
    report.checks.push(
      "F launches a moving upright fireball and its point light visibly illuminates nearby floor",
      "held F casts once and expired fireballs release their lights",
      "obstacle impact expands a short flash, six outward embers and two rising smoke wisps with a brighter light pulse, then releases every effect",
    );

    const clickButton = async (touch) => {
      const point = await page.evaluate(() => {
        // UI uses viewport pixels, independently of world input projection.
        const r = document.querySelector("canvas").getBoundingClientRect();
        const narrow = r.width <= 700,
          padding = narrow ? 16 : 24;
        return { x: r.right - padding - 2 * (44 + 8) - 64, y: r.top + padding + (narrow ? 172 : 120) + 22 };
      });
      if (touch) {
        const cdp = await page.context().newCDPSession(page);
        await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ ...point, id: 1 }] });
        await frames(8);
        await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
        await cdp.detach();
      } else {
        await page.mouse.move(point.x, point.y);
        await page.mouse.down();
        await frames(8);
        await page.mouse.up();
      }
      await frames(4);
    };
    // An away-facing cast can be correctly occluded by the player/pew. Use a
    // side-facing cast to make the flame trail visible in the art capture.
    await press("a", 2);
    const desktopRoot = await page.evaluate(() => vigil.position);
    await page.evaluate(() => kartyWorldRenderer.setOptions({ worldScale: 1 }));
    await frames(2);
    await clickButton(false);
    assert.equal(await active(), 1, "desktop Fireball button did not cast");
    assert.deepEqual(await page.evaluate(() => vigil.position), desktopRoot, "HUD click leaked to walking");
    await capture("side-fireball");
    await page.evaluate((scale) => kartyWorldRenderer.setOptions({ worldScale: scale }), movementScale);
    await frames(20);
    for (let i = 0; i < 5; i++) await press("f", 18);
    report.maximumFireballs = await page.evaluate(() => vigil.maximumFireballs);
    assert.ok(report.maximumFireballs >= 2 && report.maximumFireballs <= 4);
    await press("r");
    assert.equal(await active(), 0, "reset did not release projectile lights");
    assert.equal(
      await page.evaluate(() => Object.keys(vigil.projectileSprites).length),
      0,
      "reset did not release projectile sparks",
    );
    report.maximumProjectileSprites = await page.evaluate(() => vigil.maximumProjectileSprites);
    assert.ok(report.maximumProjectileSprites <= 36, "four-slot projectile sprite bound exceeded");
    report.checks.push(
      "desktop Fireball button casts without walking; repeated casts are bounded and reset clears all projectiles",
    );

    await page.setViewportSize({ width: 390, height: 844 });
    await frames(5);
    await capture("portrait-before-cast");
    const portraitRoot = await page.evaluate(() => vigil.position);
    await clickButton(true);
    assert.equal(await active(), 1, "portrait touch Fireball button did not cast");
    assert.deepEqual(await page.evaluate(() => vigil.position), portraitRoot, "touch HUD cast leaked to walking");
    await capture("portrait-fireball-button");
    report.checks.push("390x844 portrait touch Fireball button casts without walking");
  } else if (propsOnly) {
    const obstacles = [
      { name: "pew", x: 7.1, y: 5.5, width: 2.8, depth: 0.75 },
      { name: "column", x: 6.7, y: 9.5, width: 0.85, depth: 0.85 },
    ];
    const project = (point) =>
      page.evaluate(([x, y]) => {
        const rect = document.querySelector("canvas").getBoundingClientRect(),
          [cx, cy] = vigil.camera,
          scale = rect.height / vigil.height,
          dx = x - cx - 18,
          dy = y - cy - 18;
        return [
          rect.left + rect.width / 2 + ((dx - dy) / Math.SQRT2) * scale,
          rect.top + rect.height / 2 - ((dx + dy) / Math.SQRT2) * 0.5 * scale,
        ];
      }, point);
    const walk = async (goal) => {
      const pixel = await project(goal);
      assert.ok(
        pixel[0] > 5 && pixel[0] < 1275 && pixel[1] > 120 && pixel[1] < 590,
        `prop route target covered or offscreen: ${goal} -> ${pixel}`,
      );
      await page.keyboard.down("Shift");
      await page.mouse.move(...pixel);
      await page.mouse.down();
      await frames(3);
      await page.mouse.up();
      await page.waitForFunction(([x, y]) => Math.hypot(vigil.position[0] - x, vigil.position[1] - y) < 0.1, goal, {
        timeout: 15000,
      });
      await page.keyboard.up("Shift");
      await frames(10);
    };
    const capture = async (name) => {
      assert.equal(await page.evaluate(() => kartyWorldRenderer.setOptions({ worldScale: 1 })), null);
      await frames(2);
      assert.equal(await page.evaluate(() => kartyWorldRenderer.stats().cameras[0].worldWidth), 1280);
      const screenshot = await page.screenshot({ path: resolve(output, name + ".png") });
      report.captures.push(name + ".png");
      (report.propViews ??= []).push({ name, position: await page.evaluate(() => vigil.position) });
      if (name === "column-near") {
        const state = await page.evaluate(() => ({
          position: vigil.position,
          camera: vigil.camera,
          height: vigil.height,
          texture: vigil.heroTexture,
        }));
        assert.equal(
          state.texture,
          animation.firstIdleTexture + 5,
          "column hood regression requires the idle northeast-facing pose",
        );
        assert.ok(
          Math.abs(state.position[0] - 6.39) < 0.06 && Math.abs(state.position[1] - 9.75) < 0.06,
          "column hood regression requires the near-side collision fixture",
        );
        const pixels = await page.evaluate(hoodOcclusion, {
          source: (
            await readFile(resolve(projectDirectory, animation.textureDirectory, animation.idleNames[5] + ".png"))
          ).toString("base64"),
          screenshot: screenshot.toString("base64"),
          state,
          animation,
        });
        report.columnHood = { ...pixels, state };
        assert.ok(pixels.samples >= 8, "column hood fixture did not select enough opaque red source texels");
        assert.ok(pixels.fraction >= 0.8, `near column cut through the pilgrim hood: ${JSON.stringify(pixels)}`);
        report.checks.push("near column retains opaque red hood pixels instead of slicing the upright pilgrim");
      }
      assert.equal(await page.evaluate(() => kartyWorldRenderer.setOptions({ worldScale: 0.25 })), null);
      await frames(2);
    };
    const push = async (name, keys, axis, boundary) => {
      const before = await page.evaluate(() => vigil.position);
      const first = await page.evaluate(() => vigil.positions.length);
      for (const key of keys) await page.keyboard.down(key);
      await frames(45);
      for (const key of keys) await page.keyboard.up(key);
      await frames(5);
      const after = await page.evaluate(() => vigil.position);
      const positions = await page.evaluate((first) => vigil.positions.slice(first), first);
      assert.ok(positions.length >= 2, `${name} collision fixture never approached its obstacle`);
      assert.ok(after[axis] > before[axis] + 0.1, `${name} collision fixture did not move toward its obstacle`);
      assert.ok(
        after[axis] <= boundary + 0.0001 && after[axis] > boundary - 0.06,
        `${name} did not stop at its radius clearance: ${before} -> ${after}, boundary ${boundary}`,
      );
      const stopped = await page.evaluate(() => ({ position: vigil.position, transforms: vigil.transforms }));
      await frames(8);
      assert.deepEqual(
        await page.evaluate(() => ({ position: vigil.position, transforms: vigil.transforms })),
        stopped,
        `${name} push kept moving after key release`,
      );
      report.checks.push(`actual keyboard movement stops at ${name} footprint with player-radius clearance`);
    };
    await walk([8.75, 4.75]);
    await push("pew", ["w", "a"], 1, 5.5 - 0.28);
    await capture("pew-near");
    await walk([10.75, 5.75]);
    await walk([8.75, 6.75]);
    await capture("pew-far");
    await walk([10.75, 8.75]);
    await walk([6.25, 9.75]);
    await push("column", ["w", "d"], 0, 6.7 - 0.28);
    await capture("column-near");
    await walk([6.25, 10.75]);
    await walk([8.25, 10.75]);
    await capture("column-far");
    const positions = await page.evaluate(() => vigil.positions);
    for (const obstacle of obstacles)
      for (const [x, y] of positions) {
        const dx = Math.max(obstacle.x - x, 0, x - obstacle.x - obstacle.width),
          dy = Math.max(obstacle.y - y, 0, y - obstacle.y - obstacle.depth);
        assert.ok(
          Math.hypot(dx, dy) >= 0.28 - 0.0001,
          `submitted player transform intruded into ${obstacle.name} clearance: ${[x, y]}`,
        );
      }
    report.checks.push(
      "all observed player transforms preserve pew and column collision clearance",
      "actual routes around pew and column reach their opposite sides; full-resolution depth-occlusion review captures",
    );
  } else if (portraitOnly) {
    // The full-frame camera follows the physical viewport. Global sprites and
    // input use the camera's height-scaled, centered 1280x720 logical canvas.
    // Project independently in physical pixels, then compare the fog commands
    // after applying that separate presentation transform.
    const project = async (point) =>
      page.evaluate(([x, y]) => {
        const canvas = document.querySelector("canvas"),
          rect = canvas.getBoundingClientRect(),
          [cx, cy] = vigil.camera,
          scale = rect.height / vigil.height,
          dx = x - cx - 18,
          dy = y - cy - 18,
          logicalScale = rect.height / 720,
          offsetX = (rect.width - 1280 * logicalScale) / 2,
          offsetY = (rect.height - 720 * logicalScale) / 2,
          px = rect.width / 2 + ((dx - dy) / Math.SQRT2) * scale,
          py = rect.height / 2 - ((dx + dy) / Math.SQRT2) * 0.5 * scale,
          lx = (px - offsetX) / logicalScale,
          ly = (py - offsetY) / logicalScale;
        return {
          pixel: [rect.left + px, rect.top + py],
          logical: [lx, ly],
          viewport: [rect.width, rect.height],
          masked: Object.values(vigil.masks).some(
            (m) =>
              m.alpha > 0 && lx >= m.x - m.w / 2 && lx < m.x + m.w / 2 && ly >= m.y - m.h / 2 && ly < m.y + m.h / 2,
          ),
        };
      }, point);
    const tap = async (goal) => {
      const { pixel, viewport } = await project(goal);
      assert.ok(
        pixel[0] > 5 && pixel[0] < viewport[0] - 5 && pixel[1] > 110 && pixel[1] < viewport[1] - 120,
        `portrait floor target covered or offscreen: ${goal} -> ${pixel}`,
      );
      const cdp = await page.context().newCDPSession(page);
      await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true });
      await cdp.send("Input.dispatchTouchEvent", {
        type: "touchStart",
        touchPoints: [{ x: pixel[0], y: pixel[1], id: 1 }],
      });
      await frames(3);
      await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
      await cdp.detach();
      await page.waitForFunction(([x, y]) => Math.hypot(vigil.position[0] - x, vigil.position[1] - y) < 0.45, goal, {
        timeout: 15000,
      });
      await frames(8);
    };
    const pixels = async (name, hidden, open) => {
      const a = await project(hidden),
        b = await project(open);
      assert.equal(a.masked, true, "responsive fog failed to conceal a wall-hidden floor point");
      assert.equal(b.masked, false, "responsive fog concealed open floor");
      for (const point of [a, b])
        assert.ok(
          point.pixel[0] > 0 &&
            point.pixel[0] < point.viewport[0] &&
            point.pixel[1] > 110 &&
            point.pixel[1] < point.viewport[1] - 120,
          `responsive pixel sample is covered or offscreen: ${JSON.stringify(point)}`,
        );
      const screenshot = await page.screenshot({ path: resolve(output, name + ".png") });
      report.captures.push(name + ".png");
      const values = await page.evaluate(
        async ({ base64, points }) => {
          const bitmap = await createImageBitmap(
            new Blob([Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))], { type: "image/png" }),
          );
          const image = new OffscreenCanvas(bitmap.width, bitmap.height),
            context = image.getContext("2d");
          context.drawImage(bitmap, 0, 0);
          return {
            width: bitmap.width,
            height: bitmap.height,
            pixels: points.map(([x, y]) => Array.from(context.getImageData(Math.round(x), Math.round(y), 1, 1).data)),
          };
        },
        { base64: screenshot.toString("base64"), points: [a.pixel, b.pixel] },
      );
      const viewport = page.viewportSize();
      assert.deepEqual([values.width, values.height], [viewport.width, viewport.height]);
      assert.ok(
        values.pixels[0].slice(0, 3).every((value) => value <= 2),
        "responsive concealed floor leaked into framebuffer",
      );
      assert.ok(
        values.pixels[1].slice(0, 3).some((value) => value > 10),
        "responsive open floor was blacked out",
      );
    };
    for (let i = 0; i < 5; i++) await press("-");
    await tap([9, 4]);
    await tap([7, 4]);
    await pixels("portrait-sight", [5.5, 8.7], [8, 4]);
    const hiddenGoal = [5.25, 8.75];
    const hidden = await project(hiddenGoal);
    assert.equal(hidden.masked, true, "touch escape target must start behind the wall");
    const cdp = await page.context().newCDPSession(page);
    await cdp.send("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: [{ x: hidden.pixel[0], y: hidden.pixel[1], id: 1 }],
    });
    await frames(3);
    await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    await cdp.detach();
    await page.waitForFunction(
      ([x, y]) => Math.hypot(vigil.position[0] - x, vigil.position[1] - y) < 0.15,
      hiddenGoal,
      { timeout: 15000 },
    );
    report.checks.push(
      "390x844 portrait touch reaches wall-hidden ground by navigating around the masonry corner",
      "portrait framebuffer and fog use actual viewport projection",
    );
    await page.setViewportSize({ width: 1280, height: 720 });
    await frames(12);
    await press("r");
    await pixels("resized-landscape-sight", [2, 9], [12, 8]);
    await tap([12, 7]);
    report.checks.push("portrait-to-landscape resize preserves sight, framebuffer alignment and touch projection");
    await page.setViewportSize({ width: 2560, height: 1080 });
    await frames(12);
    await press("r");
    for (let i = 0; i < 8; i++) await press("=");
    const overscan = await project([2, 9]);
    assert.ok(overscan.logical[0] < 0, "ultrawide test must sample beyond the authored logical left edge");
    await pixels("ultrawide-sight", [2, 9], [12, 8]);
    await tap([12, 7]);
    report.checks.push(
      "2560x1080 ultrawide resize conceals floor beyond authored logical bounds and preserves touch projection",
    );
  } else {
    const capture = async (name) => {
      assert.equal(await page.evaluate(() => kartyWorldRenderer.setOptions({ worldScale: 1 })), null);
      await frames(2);
      assert.equal(
        await page.evaluate(() => kartyWorldRenderer.stats().cameras[0].worldWidth),
        1280,
        "art review must render at native width",
      );
      await page.screenshot({ path: resolve(output, name + ".png") });
      report.captures.push(name + ".png");
      assert.equal(
        await page.evaluate((scale) => kartyWorldRenderer.setOptions({ worldScale: scale }), movementScale),
        null,
      );
      await frames(2);
    };
    const sight = async (hidden) => {
      const state = await page.evaluate(() => ({
        camera: vigil.camera,
        height: vigil.height,
        masks: Object.values(vigil.masks).filter((m) => m.alpha > 0),
      }));
      const [cx, cy] = state.camera;
      const s = 720 / state.height;
      const x = 640 + ((2 - (cx + 18) - (9 - (cy + 18))) / Math.SQRT2) * s;
      const y = 360 - ((2 - (cx + 18) + 9 - (cy + 18)) / Math.SQRT2) * 0.5 * s;
      assert.equal(
        state.masks.some((m) => x >= m.x - m.w / 2 && x < m.x + m.w / 2 && y >= m.y - m.h / 2 && y < m.y + m.h / 2),
        hidden,
        "wall-hidden transept must follow current player sight",
      );
      assert.ok(state.masks.length > 0 && state.masks.length < 720);
    };
    await sight(true);
    await capture("entrance");
    const entrancePixels = await page.evaluate(
      async (base64) => {
        const png = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0));
        const bitmap = await createImageBitmap(new Blob([png], { type: "image/png" }));
        const image = new OffscreenCanvas(bitmap.width, bitmap.height),
          context = image.getContext("2d");
        context.drawImage(bitmap, 0, 0);
        const pixel = (x, y) => Array.from(context.getImageData(Math.round(x), Math.round(y), 1, 1).data);
        const [cx, cy] = vigil.camera,
          s = 720 / vigil.height;
        const at = (x, y) =>
          pixel(
            640 + ((x - cx - 18 - (y - cy - 18)) / Math.SQRT2) * s,
            360 - ((x - cx - 18 + y - cy - 18) / Math.SQRT2) * 0.5 * s,
          );
        return { hidden: at(2, 9), open: at(12, 8) };
      },
      (await readFile(resolve(output, "entrance.png"))).toString("base64"),
    );
    assert.ok(
      entrancePixels.hidden.slice(0, 3).every((value) => value <= 2),
      "concealed floor leaked into actual framebuffer",
    );
    assert.ok(
      entrancePixels.open.slice(0, 3).some((value) => value > 10),
      "open nave was blacked out",
    );
    report.checks.push("actual framebuffer conceals floor beyond wall and retains open nave");

    if (materialsOnly) {
      for (const view of ["normals", "height", "ao", "smoothness"]) {
        assert.equal(await page.evaluate((view) => kartyWorldRenderer.setOptions({ materialView: view }), view), null);
        await capture(view);
      }
      assert.equal(
        await page.evaluate(() => kartyWorldRenderer.setOptions({ materialView: "shaded", bypassLights: true })),
        null,
      );
      await capture("without-direct-light");
      assert.equal(await page.evaluate(() => kartyWorldRenderer.setOptions({ bypassLights: false })), null);
      report.checks.push("full-resolution normal, height, AO and smoothness views", "direct-light comparison");
    } else {
      if (!routesOnly) {
        for (const keys of [["w"], ["w", "a"], ["a"], ["a", "s"], ["s"], ["s", "d"], ["d"], ["d", "w"]]) {
          // Each direction starts on clear floor: a faster default pace made
          // the former cumulative circle run into a pew before its eighth row.
          await press("r");
          for (const key of keys) await page.keyboard.down(key);
          await frames(12);
          for (const key of keys) await page.keyboard.up(key);
          await frames(4);
        }
        const directions = await page.evaluate(() => [
          ...new Set(
            vigil.textures.map((t) =>
              t >= vigil.animation.walkNames.length
                ? t - vigil.animation.walkNames.length
                : Math.floor(t / vigil.animation.frames),
            ),
          ),
        ]);
        report.facingRows = directions;
        assert.equal(directions.length, animation.directions, "all facing rows must execute");
        assert.ok(
          (await page.evaluate(
            () => new Set(vigil.textures.filter((t) => t < vigil.animation.walkNames.length)).size,
          )) >
            animation.directions * 2,
          "walk cycles must advance beyond idle",
        );
        const stopped = await page.evaluate(() => ({ position: vigil.position, transforms: vigil.transforms }));
        await frames(10);
        assert.deepEqual(
          await page.evaluate(() => ({ position: vigil.position, transforms: vigil.transforms })),
          stopped,
          "released input kept moving",
        );
        report.checks.push("eight facing rows and animated walking frames", "key release stops motion");
        await press("r");

        // A complete straight walk cycle must wrap continuously at its travelled
        // cadence. Inspect real submitted texture changes, without driving state.
        await page.keyboard.down("w");
        await page.keyboard.down("a");
        await frames(3);
        await page.evaluate(() => {
          vigil.textureFrames = [];
        });
        await frames(60);
        const animationFrames = await page.evaluate(() =>
          vigil.textureFrames.filter((frame) => frame.texture < vigil.animation.walkNames.length),
        );
        await page.keyboard.up("w");
        await page.keyboard.up("a");
        assert.ok(animationFrames.length >= animation.frames + 1, "ordinary walking must execute a complete cycle");
        let wrapped = false;
        for (let i = 1; i < animationFrames.length; i++) {
          const previous = animationFrames[i - 1].texture % animation.frames,
            current = animationFrames[i].texture % animation.frames;
          const elapsed = Math.max(1, animationFrames[i].frame - animationFrames[i - 1].frame),
            advance = (current - previous + animation.frames) % animation.frames,
            maxAdvance = Math.ceil(
              (elapsed * animation.moveSpeed * animation.frames) / (60 * animation.stride) + 0.000001,
            );
          assert.ok(advance >= 1 && advance <= maxAdvance, "walk phase jumped beyond submitted travel");
          wrapped ||= current < previous;
          assert.ok(
            animationFrames[i].frame - animationFrames[i - 1].frame <=
              Math.ceil((60 * animation.stride) / (animation.moveSpeed * animation.frames)) + 1,
            "walk pose held too long",
          );
        }
        assert.equal(wrapped, true, "actual walking never crossed the final-to-first pose seam");
        await frames(4);
        assert.ok(
          await page.evaluate(
            () =>
              vigil.pose >= vigil.animation.walkNames.length &&
              vigil.pose < vigil.animation.walkNames.length + vigil.animation.directions,
          ),
          "released movement must show a dedicated idle pose",
        );
        report.checks.push("distance-driven walk cadence and continuous cycle wrap");
        await press("r");
        // Inspect actual submitted movement, including quiet updates, rather
        // than inferring smooth motion from animation frame order alone.
        assert.equal(await page.evaluate(() => kartyWorldRenderer.setOptions({ worldScale: 0.25 })), null);
        await frames(2);
        const probe = async (goal) => {
          const start = await page.evaluate(() => ({ first: vigil.movementFrames.length, position: vigil.position }));
          const pixel = await page.evaluate(([x, y]) => {
            const [cx, cy] = vigil.camera,
              scale = 720 / vigil.height,
              dx = x - cx - 18,
              dy = y - cy - 18;
            return [640 + ((dx - dy) / Math.SQRT2) * scale, 360 - ((dx + dy) / Math.SQRT2) * 0.5 * scale];
          }, goal);
          assert.ok(
            pixel[0] > 0 && pixel[0] < 1280 && pixel[1] > 120 && pixel[1] < 590,
            `movement probe target covered or offscreen: ${goal} -> ${pixel}`,
          );
          await page.mouse.move(...pixel);
          await page.mouse.down();
          await frames(3);
          await page.mouse.up();
          await page.waitForFunction(
            ([x, y]) => Math.hypot(vigil.position[0] - x, vigil.position[1] - y) < 0.001,
            goal,
            { timeout: 15000 },
          );
          await frames(4);
          const trace = await page.evaluate((first) => vigil.movementFrames.slice(first), start.first);
          let previous = start.position;
          const moves = [];
          for (const state of trace) {
            const dx = state.position[0] - previous[0],
              dy = state.position[1] - previous[1],
              distance = Math.hypot(dx, dy);
            if (distance > 0.00001)
              moves.push({
                ...state,
                dx,
                dy,
                distance,
                row:
                  state.texture >= animation.walkNames.length
                    ? state.texture - animation.walkNames.length
                    : Math.floor(state.texture / animation.frames),
              });
            previous = state.position;
          }
          for (let i = 1; i < moves.length; i++)
            assert.equal(moves[i].frame, moves[i - 1].frame + 1, "click movement paused between waypoints");
          const sameDirection = (a, b) => (a.dx * b.dx + a.dy * b.dy) / (a.distance * b.distance) > 0.999998;
          const cruise = moves.filter(
            (move, i) =>
              i > 0 && i + 1 < moves.length && sameDirection(moves[i - 1], move) && sameDirection(move, moves[i + 1]),
          );
          assert.ok(cruise.length >= 20, "movement probe did not observe a sustained clear segment");
          for (const move of cruise)
            assert.ok(
              Math.abs(move.distance - animation.moveSpeed / 60) < 0.0001,
              `click movement discarded its speed budget at a grid waypoint: ${JSON.stringify(move)}`,
            );
          return {
            start: start.position,
            goal,
            moves,
            cruiseUpdates: cruise.length,
            rows: [...new Set(moves.slice(1, -1).map((move) => move.row))],
          };
        };
        const open = await probe([12.25, 8.25]);
        for (const move of open.moves)
          assert.ok(
            move.dy > 0 && move.dx >= -0.00001,
            `open click reversed toward a grid centre: ${JSON.stringify(move)}`,
          );
        assert.equal(open.rows.length, 1, "straight click movement flickered between facing rows");
        let traveled = 0;
        for (const move of open.moves) {
          traveled += move.distance;
          const expected = Math.floor((traveled / animation.stride) * animation.frames + 0.00001) % animation.frames;
          assert.equal(move.texture % animation.frames, expected, "whole-body pose drifted from submitted root travel");
        }
        await press("r");
        await probe([8.75, 4.75]);
        const aroundPew = await probe([8.75, 6.75]);
        assert.ok(aroundPew.rows.length >= 2, "pew route must exercise multiple directions and waypoints");
        report.clickMovement = {
          worldScale: 0.25,
          openCruiseUpdates: open.cruiseUpdates,
          openRows: open.rows,
          pewCruiseUpdates: aroundPew.cruiseUpdates,
          pewRows: aroundPew.rows,
          evidence: "click-movement.json",
        };
        await writeFile(resolve(output, "click-movement.json"), JSON.stringify({ open, aroundPew }, null, 2) + "\n");
        report.checks.push(
          "actual click starts toward its goal without backward grid recentering",
          "actual straight and multiwaypoint click routes retain movement speed without idle waypoint updates",
          "straight click route preserves one facing row",
          "whole-body assets follow the submitted physical root phase",
        );
        await press("r");

        // The last held-drag event must reach navigation even between the old
        // six-tick input gate and pointer release. Use ordinary browser input.
        const dragPixel = (goal) =>
          page.evaluate(([x, y]) => {
            const [cx, cy] = vigil.camera,
              scale = 720 / vigil.height;
            return [
              640 + ((x - cx - 18 - y + cy + 18) / Math.SQRT2) * scale,
              360 - ((x - cx - 18 + y - cy - 18) / Math.SQRT2) * 0.5 * scale,
            ];
          }, goal);
        await page.mouse.move(...(await dragPixel([12.25, 6.25])));
        await page.mouse.down();
        await frames(2);
        const dragGoal = [11.75, 8.25];
        await page.mouse.move(...(await dragPixel(dragGoal)));
        await page.mouse.up();
        await page.waitForFunction(
          ([x, y]) => Math.hypot(vigil.position[0] - x, vigil.position[1] - y) < 0.15,
          dragGoal,
          { timeout: 15000 },
        );
        report.checks.push("final held-drag target survives pointer release without waiting for a six-tick gate");
        await press("r");
        assert.equal(
          await page.evaluate((scale) => kartyWorldRenderer.setOptions({ worldScale: scale }), movementScale),
          null,
        );
        await frames(2);
      }

      if (inputOnly) {
        // A wall-hidden destination must reach the navigator, which routes
        // around the masonry rather than leaving the player stuck behind it.
        const hiddenGoal = [2.25, 8.75];
        const concealed = await page.evaluate(() => {
          const [cx, cy] = vigil.camera,
            s = 720 / vigil.height;
          return [
            640 + ((2.25 - cx - 18 - (8.75 - cy - 18)) / Math.SQRT2) * s,
            360 - ((2.25 - cx - 18 + 8.75 - cy - 18) / Math.SQRT2) * 0.5 * s,
          ];
        });
        await page.keyboard.down("Shift");
        await page.mouse.move(...concealed);
        await page.mouse.down();
        await frames(3);
        await page.mouse.up();
        await page.waitForFunction(
          ([x, y]) => Math.hypot(vigil.position[0] - x, vigil.position[1] - y) < 0.15,
          hiddenGoal,
          { timeout: 15000 },
        );
        await page.keyboard.up("Shift");
        report.checks.push("mouse click through foreground occlusion reaches hidden ground around the wall");
        await press("r");
      }

      if (!inputOnly) {
        // Project actual world targets using the observed, unmodified camera pose.
        // Intermediate floor clicks keep every target within the visible viewport.
        const walk = async (goal, touch = false) => {
          const pixel = await page.evaluate(([x, y]) => {
            const [cx, cy] = vigil.camera,
              dx = x - (cx + 18),
              dy = y - (cy + 18),
              s = 720 / vigil.height;
            return [640 + ((dx - dy) / Math.SQRT2) * s, 360 - ((dx + dy) / Math.SQRT2) * 0.5 * s];
          }, goal);
          assert.ok(
            pixel[0] > 0 && pixel[0] < 1280 && pixel[1] > 120 && pixel[1] < 590,
            `floor target covered or offscreen: ${goal} -> ${pixel}`,
          );
          if (touch) {
            const cdp = await page.context().newCDPSession(page);
            await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true });
            await cdp.send("Input.dispatchTouchEvent", {
              type: "touchStart",
              touchPoints: [{ x: pixel[0], y: pixel[1], id: 1 }],
            });
            await frames(3);
            await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
            await cdp.detach();
          } else {
            await page.mouse.move(...pixel);
            await page.mouse.down();
            await frames(3);
            await page.mouse.up();
          }
          try {
            await page.waitForFunction(
              ([x, y]) => Math.hypot(vigil.position[0] - x, vigil.position[1] - y) < 0.45,
              goal,
              {
                timeout: 15000,
              },
            );
          } catch (error) {
            console.log("Failed floor click", JSON.stringify({ goal, pixel, state: await page.evaluate(() => vigil) }));
            await capture("route-failure");
            throw error;
          }
          console.log("Reached", goal);
          await frames(5);
          if (goal[0] === 12 && goal[1] === 12) await sight(false);
        };
        await page.keyboard.down("Shift");
        for (const point of [
          [12, 8],
          [12, 12],
          [8, 12],
          [4, 12],
        ])
          await walk(point);
        await press("e");
        if (routeGalleryOnly) await capture("remembrance");
        for (const point of [
          [7, 12],
          [12, 12],
          [17, 12],
          [21.2, 12.5],
        ])
          await walk(point);
        await press("Space");
        if (routeGalleryOnly) await capture("brothers-rest");
        // The two gallery captures keep their full-resolution width assertions
        // in a separate browser lifetime. Gameplay retains the complete three-
        // shrine route, reset, sight and touch checks under the same deadline.
        if (routeGalleryOnly) {
          await page.keyboard.up("Shift");
          assert.equal(await page.evaluate(() => Object.values(vigil.lights).filter(Boolean).length), 3);
          report.checks.push("full-resolution first- and second-shrine gallery retains native-width rendering");
        } else {
          for (const point of [
            [17, 13],
            [12, 13],
            [12, 17],
            [12, 20],
            [12, 22.7],
          ])
            await walk(point);
          await press("e");
          await page.keyboard.up("Shift");
          assert.equal(await page.evaluate(() => vigil.success), true, "three-light completion count missing");
          assert.equal(
            await page.evaluate(() => Object.values(vigil.lights).filter(Boolean).length),
            4,
            "three shrines and pilgrim light must be enabled",
          );
          await capture("last-light");
          if (process.argv.includes("--capture-preview")) {
            await page.setViewportSize({ width: 960, height: 540 });
            assert.equal(await page.evaluate(() => kartyWorldRenderer.setOptions({ worldScale: 1 })), null);
            await frames(4);
            await page.screenshot({ path: resolve(projectDirectory, "preview.jpg"), type: "jpeg", quality: 94 });
            await writeFile(
              resolve(projectDirectory, "preview.capture.json"),
              JSON.stringify(
                {
                  cartridgeSha256: report.cartridgeSha256,
                  hostSha256: report.hostSha256,
                  width: 960,
                  height: 540,
                  worldScale: 1,
                  antialiasing: false,
                  backend: "Chromium SwiftShader",
                  position: await page.evaluate(() => vigil.position),
                  scope:
                    "Actual three-light completion; software composition capture, not device performance evidence.",
                },
                null,
                2,
              ) + "\n",
            );
            await page.setViewportSize({ width: 1280, height: 720 });
            assert.equal(
              await page.evaluate((scale) => kartyWorldRenderer.setOptions({ worldScale: scale }), movementScale),
              null,
            );
            await frames(4);
          }
          report.checks.push(
            "mouse pathfinding through both transepts and the chapel",
            "all three vigil lights and completion",
            "host-accepted movement across the dungeon",
          );

          await press("r");
          await sight(true);
          report.checks.push("wall sight conceals, reveals and re-conceals transept independently of navigation");
          assert.equal(await page.evaluate(() => Object.values(vigil.lights).filter(Boolean).length), 1);
          await walk([12, 7], true);
          report.checks.push(
            "restart resets location and shrine lights",
            "actual landscape touch tap follows a floor route",
          );
        }
      }

      await page.keyboard.down("w");
      await frames(5);
      await press("Escape");
      await page.keyboard.up("w");
      await frames(5);
      const resetPosition = await page.evaluate(() => vigil.position);
      await frames(8);
      assert.deepEqual(
        await page.evaluate(() => vigil.position),
        resetPosition,
        "Escape failed to cancel held movement",
      );
      report.checks.push("Escape cancels movement");
    }
  }
  report.renderer = await page.evaluate(() => globalThis.kartyWorldRenderer.stats().cameras[0]);
  report.runtimeSpriteFacings = await page.evaluate(() => ({
    counts: vigil.spriteFacingCounts,
    violations: vigil.spriteFacingViolations,
  }));
  report.wholeBody = await page.evaluate(() => ({
    hero: vigil.hero,
    root: vigil.root,
    updates: vigil.spriteFrames,
    violations: vigil.spriteViolations,
  }));
  assert.ok(report.wholeBody.hero && report.wholeBody.root && report.wholeBody.hero !== report.wholeBody.root);
  assert.ok(report.wholeBody.updates > 0);
  assert.deepEqual(
    report.wholeBody.violations,
    [],
    "whole-body ground anchor disagreed with the independent physical root",
  );
  report.checks.push("upright whole-body poses preserve sole anchors and the independent fixed physical root");
  assert.deepEqual(
    report.runtimeSpriteFacings.violations,
    [],
    "runtime world sprites must use upright standing planes or fixed ground planes",
  );
  assert.ok(
    report.runtimeSpriteFacings.counts[2] >= 1 && report.runtimeSpriteFacings.counts[4] >= 1,
    "runtime facing check must observe the upright pilgrim and fixed ground shadow",
  );
  report.checks.push("all observed runtime world sprite creations and updates use upright or fixed facing");
  assert.ok(report.renderer.device.spriteInstances >= 18, "sprite draw instances missing");
  assert.equal(report.renderer.device.gpuAvailable, true);
  assert.deepEqual(report.errors, []);
  assert.ok(
    await page.evaluate(() => vigil.maximumBatch <= 65536),
    "sight commands exceeded the complete-batch byte limit",
  );
  report.navigation = await page.evaluate(() => ({ requests: vigil.pathRequests, results: vigil.pathResults }));
  if (["input", "route", "portrait"].includes(fixture))
    assert.ok(report.navigation.requests.length > 0, "Vigil did not submit engine FindPath commands");
  if (report.navigation.requests.length > 0) {
    const perFrame = new Map();
    for (const request of report.navigation.requests)
      perFrame.set(request.frame, (perFrame.get(request.frame) || 0) + 1);
    assert.ok(
      [...perFrame.values()].every((count) => count <= 1),
      "pointer events were not coalesced into one path request per update",
    );
    const found = report.navigation.results.filter((result) => result.status === 1);
    assert.ok(found.length > 0, "Vigil did not consume successful engine path results");
    assert.ok(
      report.navigation.results.every((result) => result.status !== 7),
      "crypt geometry is unsupported by engine navigation",
    );
    for (const result of report.navigation.results) {
      assert.ok(
        report.navigation.requests.some(
          (request) => request.request === result.request && request.handle === result.handle,
        ),
        "unmatched engine path result",
      );
      assert.ok(result.count <= 128 && result.expanded <= 4096, "unbounded engine route");
    }
    report.checks.push("actual cartridge uses engine FindPath commands and matching bounded path results");
  }
  report.passed = true;
  console.log(JSON.stringify({ passed: true, checks: report.checks, captures: report.captures }, null, 2));
} finally {
  clearTimeout(deadline);
  await writeFile(
    resolve(output, fixture === "route" ? "route-report.json" : "report.json"),
    JSON.stringify(report, null, 2) + "\n",
  );
  await browser.close();
  server.closeAllConnections();
  await new Promise((done) => server.close(done));
}
