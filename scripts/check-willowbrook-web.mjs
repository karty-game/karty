import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";
import { startVillageServer } from "./willowbrook-server.mjs";
import { assertQuestAnchors, checkQuestPixels } from "./check-willowbrook-quest.mjs";
import { checkNavigation } from "./check-willowbrook-navigation.mjs";

const portrait = process.argv.includes("--portrait");
const artOnly = process.argv.includes("--art");
const structures = process.argv.includes("--structures");
const motion = process.argv.includes("--motion");
const interactions = process.argv.includes("--interactions");
const hideQuest = process.argv.includes("--hide-quest");
const navigation = process.argv.includes("--navigation");
const bridgeFixture = process.argv.includes("--bridge");
const collision = process.argv.includes("--collision-house")
  ? "house"
  : process.argv.includes("--collision-tree")
    ? "tree"
    : process.argv.includes("--collision-terrain")
      ? "terrain"
      : "";
const fixture = navigation
  ? "navigation"
  : collision
    ? "collision-" + collision
    : bridgeFixture
      ? "bridge"
      : portrait
        ? "portrait"
        : artOnly
          ? "art"
          : structures
            ? "structures"
            : motion
              ? "motion"
              : interactions
                ? "interactions"
                : "gameplay";
const output = resolve(process.env.KARTY_WILLOWBROOK_OUTPUT || "dist/validation/willowbrook");
await mkdir(output, { recursive: true });
const projectDirectory = resolve(process.env.KARTY_WILLOWBROOK_PROJECT || "samples/willowbrook");
const directory = resolve(process.env.KARTY_WILLOWBROOK_WEB || resolve(projectDirectory, "dist/web"));
const art = JSON.parse(await readFile(resolve(projectDirectory, "internal/village/map.generated.json"), "utf8"));
const mapBytes = await readFile(resolve(projectDirectory, "internal/village/village.tmj"));
const map = JSON.parse(mapBytes);
const property = (list, name, fallback) => list?.find((p) => p.name === name)?.value ?? fallback;
const objects = map.layers.flatMap((l) => l.objects || []);
const spawn = objects.find((o) => o.class === "spawn");
const stations = objects
  .filter((o) => ["clue", "treasure"].includes(o.class))
  .sort((a, b) => property(a.properties, "order", 0) - property(b.properties, "order", 0));
const stationTexts = stations.map((o) => property(o.properties, "message", ""));
const heroHeight = property(map.properties, "heroHeight", 60);
const hash = async (name) =>
  createHash("sha256")
    .update(await readFile(resolve(directory, name)))
    .digest("hex");
const levelFile = (await readdir(resolve(directory, "content"))).find((n) => n.endsWith(".kld"));
const report = {
  passed: false,
  fixture,
  hideQuest,
  scope:
    "Actual Karty WASM and Ebiten 2D sprites in Chromium SwiftShader at full scale; synthetic keyboard/mouse/touch. No physical-device/FPS qualification.",
  hostSha256: await hash("karty-host.wasm"),
  cartridgeSha256: await hash("game.kart"),
  levelSha256: await hash("content/" + levelFile),
  mapSha256: createHash("sha256").update(mapBytes).digest("hex"),
  checks: [],
  errors: [],
};
const server = await startVillageServer(directory);
let browser, gameFrame, gamePage;
const deadline = setTimeout(() => {
  console.error("Willowbrook exceeded 120s");
  process.exit(1);
}, 120000);
try {
  browser = await chromium.launch({
    headless: true,
    args: ["--no-sandbox", "--use-gl=angle", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  const context = await browser.newContext({
    viewport: portrait ? { width: 390, height: 844 } : { width: 1280, height: 720 },
    hasTouch: true,
  });
  const page = await context.newPage();
  gamePage = page;
  page.on("pageerror", (e) => report.errors.push(e.message));
  page.on("console", (e) => {
    if (e.type() === "error") report.errors.push(e.text());
  });
  await page.addInitScript(
    ({ art, heroHeight, motion, collision, hideQuest }) => {
      const heroIDs = new Set(Array.from({ length: 12 }, (_, i) => art.textures["hero-" + String(i).padStart(2, "0")]));
      const assetNames = Object.fromEntries(Object.entries(art.textures).map(([name, id]) => [id, name]));
      const s = (globalThis.villageEvidence = {
        hero: 0,
        cache: 0,
        position: [],
        camera: [],
        cameraUpdates: 0,
        sprites: 0,
        destroyed: 0,
        world3D: 0,
        legacyText2D: 0,
        texts: [],
        textByID: {},
        heroTextures: [],
        maxBatch: 0,
        maxRecords: 0,
        inputResets: 0,
        frame: 0,
        riverImages: {},
        questSprites: {},
        pathQueries: [],
        pathResults: [],
        pointerDowns: [],
        worldConfigurations: 0,
        worldObstacles: 0,
        motion: motion ? { watched: {}, heroEvents: [], shadows: {} } : null,
        collisionPath: collision ? [] : null,
      });
      let update, submit;
      Object.defineProperty(globalThis, "kartyClientUpdate", {
        configurable: true,
        get: () => update,
        set(original) {
          update = (frame, batch) => {
            s.frame = Number(frame);
            const v = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
            let p = 24;
            for (let i = 0; i < v.getUint32(16, true); i++) {
              if (v.getUint8(p) === 14) s.inputResets++;
              if (v.getUint8(p) === 13 && v.getUint8(p + 8) === 1 && s.camera.length === 3) {
                const x = v.getFloat32(p + 11, true),
                  y = v.getFloat32(p + 15, true);
                s.pointerDowns.push({
                  x: Math.fround(s.camera[0] + Math.fround((x - 640) / s.camera[2])),
                  y: Math.fround(s.camera[1] + Math.fround((y - 360) / s.camera[2])),
                });
              }
              if (v.getUint8(p) === 17) {
                const data = p + 3;
                const count = v.getUint16(data + 10, true);
                s.pathResults.push({
                  request: v.getUint32(data, true),
                  status: v.getUint8(data + 8),
                  expanded: v.getUint16(data + 12, true),
                  points: Array.from({ length: count }, (_, index) => [
                    v.getFloat32(data + 16 + index * 8, true),
                    v.getFloat32(data + 20 + index * 8, true),
                  ]),
                });
              }
              p += 3 + v.getUint16(p + 1, true);
            }
            return original(frame, batch);
          };
        },
      });
      Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
        configurable: true,
        get: () => submit,
        set(original) {
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
              if (tag === 55) {
                s.camera = [0, 4, 8].map((n) => v.getFloat32(p + n, true));
                s.cameraUpdates++;
              }
              if (tag >= 27 && tag <= 35) s.world3D++;
              if (tag === 58) {
                const operation = v.getUint8(p);
                if (operation === 1) s.worldConfigurations++;
                if (operation === 2) s.worldObstacles++;
                if (operation === 11)
                  s.pathQueries.push({
                    request: v.getUint32(p + 12, true),
                    destination: [v.getFloat32(p + 28, true), v.getFloat32(p + 32, true)],
                  });
              }
              if (tag === 16) {
                s.sprites++;
                const asset = v.getUint32(p + 8, true);
                const name = assetNames[asset] || "";
                if (name === "clue-note" || name === "treasure-closed")
                  s.questSprites[id] = {
                    name,
                    x: v.getFloat32(p + 12, true),
                    y: v.getFloat32(p + 16, true),
                    sx: 1,
                    sy: 1,
                    layer: 0,
                  };
                if (name === "riverbed" || name.startsWith("water-surface-")) {
                  s.riverImages[id] = {
                    name,
                    x: v.getFloat32(p + 12, true),
                    y: v.getFloat32(p + 16, true),
                    sx: 1,
                    sy: 1,
                    opacity: 1,
                  };
                }
                if (heroIDs.has(asset)) {
                  s.hero = id;
                  s.position = [v.getFloat32(p + 12, true), v.getFloat32(p + 16, true) + heroHeight / 2];
                }
                if (asset === art.textures["treasure-closed"]) {
                  s.cache = id;
                  s.cacheAsset = asset;
                }
              }
              if (tag === 2) {
                s.destroyed++;
                delete s.textByID[id];
                delete s.riverImages[id];
                delete s.questSprites[id];
              }
              if (tag === 3 && s.questSprites[id])
                Object.assign(s.questSprites[id], {
                  x: v.getFloat32(p + 4, true),
                  y: v.getFloat32(p + 8, true),
                  sx: v.getFloat32(p + 16, true),
                  sy: v.getFloat32(p + 20, true),
                });
              if (tag === 7 && s.questSprites[id]) s.questSprites[id].layer = v.getInt32(p + 4, true);
              if (tag === 3 && s.riverImages[id])
                Object.assign(s.riverImages[id], {
                  x: v.getFloat32(p + 4, true),
                  y: v.getFloat32(p + 8, true),
                  sx: v.getFloat32(p + 16, true),
                  sy: v.getFloat32(p + 20, true),
                });
              if (tag === 6 && s.riverImages[id]) s.riverImages[id].opacity = v.getFloat32(p + 4, true);
              if (tag === 3 && id === s.hero) {
                s.position = [v.getFloat32(p + 4, true), v.getFloat32(p + 8, true) + heroHeight / 2];
                if (s.collisionPath && s.collisionPath.length < 4096)
                  s.collisionPath.push({ frame: s.frame, x: s.position[0], y: s.position[1] });
              }
              if (tag === 17) {
                const asset = v.getUint32(p + 8, true);
                if (id === s.hero && !s.heroTextures.includes(asset)) s.heroTextures.push(asset);
                if (id === s.cache) s.cacheAsset = asset;
                if (s.questSprites[id]) s.questSprites[id].name = assetNames[asset];
              }
              if (s.motion) {
                const m = s.motion;
                if (tag === 16) {
                  const asset = v.getUint32(p + 8, true),
                    name = assetNames[asset] || "";
                  const x = v.getFloat32(p + 12, true),
                    y = v.getFloat32(p + 16, true);
                  const family = ["water", "tree", "flower", "grass", "riverbed"].find((f) => name.startsWith(f));
                  if (
                    family &&
                    !m.watched[family] &&
                    Math.abs(x - s.camera[0]) < 600 &&
                    Math.abs(y - s.camera[1]) < 300
                  )
                    m.watched[family] = { id, initial: { x, y, asset }, assets: [], transforms: [] };
                  if (name === "shadow-soft") m.shadows[id] = { x, y };
                }
                if (tag === 3) {
                  const pose = {
                    frame: s.frame,
                    x: v.getFloat32(p + 4, true),
                    y: v.getFloat32(p + 8, true),
                    rotation: v.getFloat32(p + 12, true),
                    sx: v.getFloat32(p + 16, true),
                    sy: v.getFloat32(p + 20, true),
                  };
                  if (m.shadows[id]) m.shadows[id] = { x: pose.x, y: pose.y };
                  for (const watch of Object.values(m.watched))
                    if (watch.id === id && watch.transforms.length < 512) watch.transforms.push(pose);
                }
                if (tag === 17) {
                  const event = { frame: s.frame, asset: v.getUint32(p + 8, true) };
                  if (id === s.hero && m.heroEvents.length < 512) m.heroEvents.push(event);
                  for (const watch of Object.values(m.watched))
                    if (watch.id === id && watch.assets.length < 512) watch.assets.push(event);
                }
                if (tag === 2) delete m.shadows[id];
              }
              if (tag === 8 || tag === 9) {
                if (tag === 8) s.legacyText2D++;
                const n = v.getUint16(p + 4, true),
                  text = new TextDecoder().decode(batch.slice(p + 6, p + 6 + n));
                if (!s.texts.includes(text)) s.texts.push(text);
                s.textByID[id] = text;
                s.latestText = text;
              }
              // Presentation HUD labels are KartUI elements, independent of
              // the pixel-art world camera and its Text2D entities.
              if (tag === 20) {
                const element = v.getUint32(p + 4, true),
                  n = v.getUint16(p + 9, true),
                  text = new TextDecoder().decode(batch.slice(p + 11, p + 11 + n));
                if (!s.texts.includes(text)) s.texts.push(text);
                s.textByID["ui:" + id + ":" + element] = text;
                s.latestText = text;
              }
              if (tag === 19) {
                for (const key of Object.keys(s.textByID)) if (key.startsWith("ui:" + id + ":")) delete s.textByID[key];
              }
              // Negative control: preserve observed anchors but suppress drawing.
              if (hideQuest && tag === 3 && s.questSprites[id]) {
                v.setFloat32(p + 16, 0, true);
                v.setFloat32(p + 20, 0, true);
              }
              offset += 3 + len;
            }
            return original(batch);
          };
        },
      });
    },
    { art, heroHeight, motion, collision: collision || bridgeFixture || navigation, hideQuest },
  );
  const origin = `http://127.0.0.1:${server.address().port}`;
  await page.route("**/focus-check.html", (route) =>
    route.fulfill({
      contentType: "text/html",
      body: '<style>body{margin:0;overflow:hidden}iframe{display:block;border:0;width:100vw;height:100vh}textarea{position:fixed;right:8px;top:8px;width:180px;height:70px}</style><iframe src="/?debug=0"></iframe><textarea hidden aria-label="Sibling editor">Focus test editor</textarea>',
    }),
  );
  await page.goto(origin + "/focus-check.html");
  const game = page.frames().find((f) => f.parentFrame());
  gameFrame = game;
  assert.ok(game);
  await game.waitForFunction(() => villageEvidence.hero && villageEvidence.texts.includes("WILLOWBROOK"), null, {
    timeout: 30000,
  });
  await game.locator("canvas").evaluate((c) => {
    c.tabIndex = 0;
    c.focus();
  });
  const state = () => game.evaluate(() => structuredClone(villageEvidence));
  const frames = async (count) => {
    const start = (await state()).frame;
    await game.waitForFunction(
      ({ start, count }) => villageEvidence.frame >= start + count,
      { start, count },
      { timeout: 15000 },
    );
  };
  const press = async (key, count) => {
    await page.keyboard.down(key);
    await frames(count);
    await page.keyboard.up(key);
    await frames(4);
  };
  const capture = async (name) => {
    await frames(2);
    await page.screenshot({ path: resolve(output, name + ".png"), fullPage: true, timeout: 15000 });
  };
  const captureStructure = async (name) => {
    await page.setViewportSize({ width: 2560, height: 1440 });
    await frames(4);
    await capture(name);
    const landmark = objects.find((o) => o.name === (name.startsWith("stairs") ? "intact-staircase" : "intact-bridge"));
    const s = await state();
    const topLeft = await project({
      x: landmark.x - landmark.width / 2 - 16,
      y: Math.min(landmark.y - landmark.height, s.position[1] - heroHeight) - 16,
    });
    const bottomRight = await project({
      x: landmark.x + landmark.width / 2 + 16,
      y: Math.max(landmark.y, s.position[1]) + 16,
    });
    const x = Math.floor(topLeft.x),
      y = Math.floor(topLeft.y);
    await page.screenshot({
      path: resolve(output, name + "-detail.png"),
      clip: { x, y, width: Math.ceil(bottomRight.x) - x, height: Math.ceil(bottomRight.y) - y },
      timeout: 15000,
    });
    (report.captures ??= []).push({ name, position: s.position, camera: s.camera });
    await page.setViewportSize({ width: 1280, height: 720 });
    await frames(4);
  };
  const touchSession = await context.newCDPSession(page);
  const tap = async (x, y) => {
    await touchSession.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ id: 1, x, y }] });
    await frames(4);
    await touchSession.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    await frames(4);
  };
  const project = async (point) => {
    const s = await state();
    const box = await game.locator("canvas").boundingBox();
    const scale = box.height / 720;
    return {
      x: box.x + box.width / 2 + (point.x - s.camera[0]) * s.camera[2] * scale,
      y: box.y + box.height / 2 + (point.y - s.camera[1]) * s.camera[2] * scale,
    };
  };
  const clickWorld = async (point, height = 0) => {
    const q = await project({ x: point.x, y: point.y - height });
    const viewport = page.viewportSize();
    assert.ok(
      q.x > 0 && q.x < viewport.width && q.y > 0 && q.y < viewport.height,
      "world target outside visible canvas: " + JSON.stringify(q),
    );
    await tap(q.x, q.y);
  };
  const arrive = async (target) =>
    game.waitForFunction(
      ({ x, y }) => Math.hypot(villageEvidence.position[0] - x, villageEvidence.position[1] - y) < 2,
      target,
      { timeout: 20000 },
    );
  const walkTo = async (point) => {
    // Let the eased camera settle before projecting a precise world target.
    await frames(30);
    report.target = point;
    const before = await state();
    await clickWorld(point);
    if (before.worldConfigurations) {
      await game.waitForFunction((count) => villageEvidence.pathQueries.length > count, before.pathQueries.length, {
        timeout: 3000,
      });
      const request = (await state()).pathQueries.at(-1).request;
      await game.waitForFunction(
        (request) => villageEvidence.pathResults.some((result) => result.request === request),
        request,
        { timeout: 15000 },
      );
      const result = (await state()).pathResults.find((result) => result.request === request);
      assert.equal(result.status, 1, `ground route rejected: ${JSON.stringify({ point, result })}`);
    }
    await arrive(point);
    await frames(8);
  };
  const initial = await state();
  assert.equal(initial.world3D, 0);
  assert.equal(initial.legacyText2D, 0, "Willow text must use the presentation HUD instead of world Text2D");
  const presentationTexts = Object.entries(initial.textByID)
    .filter(([key]) => key.startsWith("ui:"))
    .map(([, text]) => text);
  assert.ok(presentationTexts.includes("WILLOWBROOK"), "title was not submitted through KartUI");
  assert.ok(
    presentationTexts.some((text) => text.includes("JOURNAL: FIND THE COTTAGE SIGN")),
    "journal was not submitted through KartUI",
  );
  report.checks.push("title and journal use KartUI presentation labels; no legacy world Text2D entities are created");
  assert.ok(initial.camera.length === 3 && initial.sprites > 1 && initial.sprites <= 384);
  assert.ok(Math.hypot(initial.position[0] - spawn.x, initial.position[1] - spawn.y) < 0.01);
  assertQuestAnchors(
    initial,
    stations,
    map.layers.findIndex((layer) => layer.name === "Scenery"),
  );
  report.checks.push(
    "both signs and the chest spawn at authored map anchors on the scenery layer after runtime map loading",
  );
  const questPixels = async (index, opened = false) => {
    const name = index < 2 ? "clue-note" : opened ? "treasure-open" : "treasure-closed";
    const source = (await readFile(resolve(projectDirectory, "levels/village", name + ".png"))).toString("base64");
    const result = await checkQuestPixels(game, stations[index], index, source);
    (report.questPixels ??= []).push({ index, name, ...result });
  };
  if (interactions || fixture === "gameplay" || artOnly) await questPixels(0);
  report.checks.push(
    "Tiled static background chunks and dynamic scenery use at most 384 ordinary Karty 2D sprites at startup; zero 3D camera/world commands; bounded startup batches",
  );
  if (navigation) {
    await checkNavigation({ game, page, state, frames, press, clickWorld, arrive, capture, report });
  } else if (collision) {
    const radius = property(map.properties, "playerRadius", 7.5);
    const radiusY = property(map.properties, "playerRadiusY", radius);
    const blocked = async (side, key, predicate) => {
      const before = await state();
      await press(key, 90);
      const after = await state();
      const samples = after.collisionPath.filter((p) => p.frame > before.frame);
      assert.ok(samples.length > 30, "not enough actual held-input movement samples");
      assert.ok(samples.every(predicate), collision + " penetrated from " + side);
      assert.ok(
        Math.hypot(after.position[0] - before.position[0], after.position[1] - before.position[1]) > 8,
        "approach was already stuck before touching prop",
      );
      (report.approaches ??= []).push({ side, start: before.position, end: after.position, samples: samples.length });
      await capture("collision-" + collision + "-" + side);
    };
    if (collision === "terrain") {
      const stair = objects.find((o) => o.class === "stair");
      const centre = stair.x + stair.width / 2;
      const middle = stair.y + stair.height / 2;
      await walkTo({ x: centre, y: stair.y + stair.height + 30 });
      await walkTo({ x: centre, y: middle });
      await blocked("stair-left", "a", (p) => p.x >= stair.x + radius - 1);
      await walkTo({ x: centre, y: middle });
      await blocked("stair-right", "d", (p) => p.x <= stair.x + stair.width - radius + 1);
      await walkTo({ x: centre, y: middle });
      await walkTo({ x: centre, y: stair.y - 32 });
      await walkTo({ x: centre - 72, y: stair.y - 32 });
      await capture("terrain-stair-top-landing");
      await press("r", 1);
      const rock = objects.find((o) => o.name === "bank-rock-4");
      await walkTo({ x: rock.x - 53, y: rock.y + 18 });
      await walkTo({ x: rock.x - 53, y: rock.y - 18 });
      await blocked("rock-left", "d", (p) => p.x <= rock.x - 30);
      await walkTo({ x: rock.x - 53, y: rock.y + 32 });
      await walkTo({ x: rock.x + 4, y: rock.y + 32 });
      await blocked("rock-front", "w", (p) => p.y > rock.y + radiusY - 2);
      report.checks.push(
        "held input cannot cross either stone stair cheek or the visible bank boulder; centre steps and narrow top landing remain traversable with body-sized feet",
      );
    } else if (collision === "house") {
      const p = objects.filter((o) => o.class === "cottage").sort((a, b) => b.y - a.y)[0];
      await walkTo({ x: p.x + 192, y: p.y - 30 });
      await blocked("right", "a", (q) => q.x >= p.x + 124 + radius - 2);
      await walkTo({ x: p.x + 192, y: p.y + 50 });
      await walkTo({ x: p.x, y: p.y + 50 });
      await blocked("front", "w", (q) => q.y >= p.y + radiusY - 2);
      await walkTo({ x: p.x - 160, y: p.y + 50 });
      await walkTo({ x: p.x - 160, y: p.y - 30 });
      await blocked("left", "d", (q) => q.x <= p.x - 124 - radius + 2);
      await walkTo({ x: p.x - 160, y: p.y - 156 });
      await walkTo({ x: p.x, y: p.y - 156 });
      await capture("collision-house-behind-roof");
      await blocked("rear", "s", (q) => q.y <= p.y - 116 - radiusY + 2);
      await walkTo({ x: p.x + 192, y: p.y - 156 });
      await walkTo({ x: p.x + 192, y: p.y - 48 });
      await walkTo({ x: p.x + 192, y: p.y + 50 });
      await page.keyboard.down("a");
      await page.keyboard.down("w");
      await frames(90);
      await page.keyboard.up("a");
      await page.keyboard.up("w");
      await frames(4);
      const corner = (await state()).position;
      assert.ok(
        corner[0] >= p.x + 124 + radius - 2 && corner[1] < p.y - 20,
        "diagonal corner failed to slide outside annex",
      );
      await capture("collision-house-corner");
    } else {
      const p = objects.find((o) => o.name === "tree-9"),
        s = p.width / 160;
      // This lower-bank tree has four independently reachable approaches.
      // The waterfall tree's east corridor is now correctly closed by rocks.
      await walkTo({ x: p.x - 72, y: p.y - 120 });
      await walkTo({ x: p.x - 72, y: p.y + 56 });
      await walkTo({ x: p.x + 72, y: p.y + 56 });
      await walkTo({ x: p.x + 72, y: p.y - 28 });
      await blocked("right", "a", (q) => q.x >= p.x + 20 * s + radius - 2);
      await walkTo({ x: p.x + 72, y: p.y + 56 });
      await walkTo({ x: p.x, y: p.y + 56 });
      await blocked("front", "w", (q) => q.y >= p.y + 2 * s + radiusY - 2);
      await walkTo({ x: p.x - 72, y: p.y + 56 });
      await walkTo({ x: p.x - 72, y: p.y - 28 });
      await blocked("left", "d", (q) => q.x <= p.x - 20 * s - radius + 2);
      await walkTo({ x: p.x - 72, y: p.y - 88 });
      await walkTo({ x: p.x, y: p.y - 88 });
      await capture("collision-tree-behind-canopy");
      await blocked("rear", "s", (q) => q.y <= p.y - 52 * s - radiusY + 2);
    }
    if (collision !== "terrain")
      report.checks.push(
        "actual held keyboard movement blocked at front/back/left/right of authored " +
          collision +
          " body; walking behind roof/canopy remains reachable",
      );
  } else if (bridgeFixture) {
    const bridge = objects.find((o) => o.name === "intact-bridge");
    assert.ok(bridge, "missing intact bridge artwork");
    // These plank edges are traced from the approved 384x147 source image.
    // Water collision expansion must not consume the visible off-centre floor.
    const north = bridge.y - bridge.height + (bridge.height * 30) / 147;
    const south = bridge.y - bridge.height + (bridge.height * 88) / 147;
    const centre = (north + south) / 2;
    const upper = Math.ceil(north) + 2;
    const lower = Math.floor(south) - 2;
    const drive = async (name, keys, count, verify) => {
      const before = await state();
      for (const key of keys) await page.keyboard.down(key);
      await frames(count);
      for (const key of keys) await page.keyboard.up(key);
      await frames(4);
      const after = await state();
      const samples = after.collisionPath.filter((p) => p.frame > before.frame);
      assert.ok(samples.length >= count, "missing held bridge input samples: " + name);
      assert.ok(
        samples.every((p) => p.y >= north - 1 && p.y <= south + 1),
        "held bridge input left the visible planks: " + name,
      );
      verify(before.position, after.position, samples);
      (report.bridgeRuns ??= []).push({ name, start: before.position, end: after.position, samples: samples.length });
    };
    await page.keyboard.down("Shift");
    await walkTo({ x: bridge.x - 136, y: centre });
    await drive("west-to-east-landings", ["d"], 95, (before, after) => {
      assert.ok(after[0] - before[0] > 266, "east landing interrupted the bridge crossing");
    });
    await capture("bridge-east-landing");
    await walkTo({ x: bridge.x + 136, y: centre });
    await drive("east-to-west-landings", ["a"], 95, (before, after) => {
      assert.ok(before[0] - after[0] > 266, "west landing interrupted the bridge crossing");
    });
    await capture("bridge-west-landing");
    await walkTo({ x: bridge.x - 48, y: centre });
    for (const lane of [upper, lower]) {
      for (const direction of [1, -1]) {
        await walkTo({ x: bridge.x - direction * 48, y: lane });
        await drive(
          "lane-" + lane + "-direction-" + direction,
          [direction > 0 ? "d" : "a"],
          36,
          (before, after, samples) => {
            assert.ok((after[0] - before[0]) * direction >= 94, "invisible barrier in visible bridge lane");
            assert.ok(
              samples.every((p) => Math.abs(p.y - lane) < 2),
              "cardinal bridge crossing drifted off its lane",
            );
          },
        );
      }
    }
    for (const vertical of [-1, 1]) {
      await walkTo({ x: bridge.x, y: centre });
      await drive("rail-" + vertical, [vertical < 0 ? "w" : "s"], 30, (_before, after) => {
        assert.ok(
          vertical < 0 ? after[1] <= north + 3 : after[1] >= south - 3,
          "bridge rail stopped the hero before the visible plank edge",
        );
      });
    }
    for (const direction of [1, -1]) {
      const vertical = -direction;
      await walkTo({ x: bridge.x - direction * 48, y: centre });
      await drive(
        "diagonal-" + direction,
        [direction > 0 ? "d" : "a", vertical < 0 ? "w" : "s"],
        45,
        (before, after) => {
          assert.ok((after[0] - before[0]) * direction > 88, "diagonal bridge motion failed to slide along its rail");
          assert.ok(
            vertical < 0 ? after[1] <= north + 3 : after[1] >= south - 3,
            "diagonal input did not reach the visible deck edge",
          );
        },
      );
    }
    await page.keyboard.up("Shift");
    await frames(4);
    await capture("bridge-off-centre-deck");
    report.checks.push(
      "actual held input crosses both full landings and both off-centre plank lanes in either direction; cardinal and diagonal rail pressure stops at the visible deck edges without interrupting horizontal motion",
    );
  } else if (artOnly) {
    const river = Object.values(initial.riverImages);
    const beds = river.filter((s) => s.name === "riverbed"),
      surfaces = river.filter((s) => s.name.startsWith("water-surface-"));
    assert.ok(beds.length >= 4 && surfaces.length >= 4);
    assert.ok(
      beds.every((s) => Math.abs(s.sx) === 1 && Math.abs(s.sy) === 1),
      "riverbed stones were shrunk into 32px cells",
    );
    assert.ok(
      surfaces.every((s) => s.opacity > 0.55 && s.opacity < 0.65),
      "river surface lost its authored translucency",
    );
    report.water = {
      fullSizeBedStamps: beds.length,
      surfaceOpacity: surfaces[0].opacity,
    };
    report.checks.push(
      "full-size riverbed stones under a 0.6-opacity animated surface; actual composition is reviewed against the original artwork separately",
    );
    await capture("village-full");
    await page.setViewportSize({ width: 960, height: 540 });
    await frames(8);
    await page.screenshot({
      path: resolve(projectDirectory, "preview.jpg"),
      type: "jpeg",
      quality: 90,
      fullPage: true,
      timeout: 15000,
    });
  } else if (portrait) {
    await capture("portrait-spawn");
    const before = await state();
    const goal = { x: before.position[0] + 64, y: before.position[1] };
    await clickWorld(goal);
    await arrive(goal);
    await frames(12);
    const after = await state();
    assert.ok(after.camera[0] > before.camera[0] + 40, "portrait camera did not follow");
    const canvas = await game.locator("canvas").boundingBox();
    assert.ok(
      Math.abs(canvas.width - 390) < 2 && Math.abs(canvas.height - 844) < 2,
      "2D-only viewport letterboxed portrait",
    );
    const c = await project({ x: after.camera[0], y: after.camera[1] });
    assert.ok(Math.abs(c.x - 195) < 2 && Math.abs(c.y - 422) < 2);
    await capture("portrait-follow");
    await page.setViewportSize({ width: 844, height: 390 });
    await frames(12);
    await capture("phone-landscape");
    await page.setViewportSize({ width: 390, height: 844 });
    await frames(12);
    await capture("portrait-return");
    report.checks.push(
      "actual portrait touch reaches world point; 2D camera pans;390x844 fills viewport; landscape rotation and return retain HUD/player framing",
    );
  } else if (motion) {
    // Record actual submitted texture/transform changes without changing game
    // time or commands. Sequential captures preserve the real animation state.
    for (let i = 0; i < 8; i++) {
      await frames(30);
      await capture("ambient-" + String(i).padStart(2, "0"));
      (report.ambient ??= []).push({ name: "ambient-" + String(i).padStart(2, "0"), frame: (await state()).frame });
    }
    const ambient = (await state()).motion;
    for (const family of ["water", "tree", "flower"]) {
      const watch = ambient.watched[family];
      assert.ok(
        watch && new Set(watch.assets.map((e) => e.asset)).size >= 2,
        family + " did not animate in actual WASM",
      );
    }
    const grass = ambient.watched.grass;
    assert.ok(
      grass && new Set(grass.transforms.map((p) => p.rotation.toFixed(5))).size > 3,
      "grass breeze stayed static",
    );
    assert.ok(ambient.watched.riverbed, "no visible riverbed sprite beneath water");
    report.checks.push(
      "actual Tiled water/flower/canopy frame swaps, fixed riverbed and gentle grass transforms across eight sequential captures",
    );
    const gait = async (running, count) => {
      await press("r", 1);
      const before = await state();
      if (running) await page.keyboard.down("Shift");
      await page.keyboard.down("d");
      await frames(count);
      await page.keyboard.up("d");
      if (running) await page.keyboard.up("Shift");
      await frames(4);
      const after = await state();
      const events = after.motion.heroEvents.filter((e) => e.frame > before.frame && e.frame <= after.frame);
      const rate = (events.length * 60) / (after.frame - before.frame);
      assert.ok(
        rate > (running ? 7 : 5.5) && rate < (running ? 10.5 : 8.5),
        "incorrect observed gait cadence: " + rate,
      );
      assert.ok(after.position[0] > before.position[0] + 100, "cadence check did not actually walk");
      assert.ok(
        Object.values(after.motion.shadows).some(
          (p) => Math.hypot(p.x - after.position[0] - 2, p.y - after.position[1]) < 0.05,
        ),
        "contact shadow detached from hero feet",
      );
      const idleEvents = after.motion.heroEvents.length;
      await frames(35);
      assert.equal((await state()).motion.heroEvents.length, idleEvents, "idle character continued stepping");
      return { running, rate, frames: after.frame - before.frame, changes: events.length };
    };
    report.gait = [await gait(false, 90), await gait(true, 60)];
    report.checks.push(
      "observed slower walking/running gait, idle freeze and contact shadow follows actual hero soles",
    );
  } else if (structures) {
    await page.keyboard.down("Shift");
    const stair = objects.find((o) => o.class === "stair"),
      bridge = objects.find((o) => o.class === "bridge" && !o.gid);
    assert.ok(stair && bridge);
    await walkTo({ x: stair.x + stair.width / 2, y: stair.y + stair.height + 40 });
    await walkTo({ x: stair.x + 18, y: stair.y + stair.height / 2 });
    await captureStructure("stairs-side-occlusion");
    await walkTo({ x: stair.x + stair.width / 2, y: stair.y + stair.height + 40 });
    await walkTo({ x: bridge.x - 32, y: bridge.y + bridge.height / 2 });
    const leftBank = bridge.x - 32;
    const deckY = bridge.y + bridge.height / 2;
    const bridgeArt = objects.find((o) => o.name === "intact-bridge");
    const nearPost = objects.find((o) => o.name === "bridge-near-left-leg");
    assert.ok(nearPost, "missing visible near bridge support");
    const behindPostY = nearPost.y - property(map.properties, "playerRadiusY", 10) - 2;
    await walkTo({ x: leftBank, y: behindPostY });
    await captureStructure("bridge-behind-post");
    await walkTo({ x: leftBank - 40, y: behindPostY });
    await walkTo({ x: leftBank - 40, y: bridgeArt.y + 24 });
    await walkTo({ x: leftBank, y: bridgeArt.y + 24 });
    await captureStructure("bridge-in-front");
    await walkTo({ x: leftBank - 40, y: bridgeArt.y + 24 });
    await walkTo({ x: leftBank - 40, y: deckY });
    await walkTo({ x: leftBank, y: deckY });
    await walkTo({ x: bridge.x + bridge.width / 2, y: deckY });
    await captureStructure("bridge-traversal");
    await walkTo({ x: bridge.x + bridge.width + 32, y: bridge.y + bridge.height / 2 });
    await capture("bridge");
    await page.keyboard.up("Shift");
    report.checks.push(
      "actual stair traversal beside stone cheek; bridge behind-post, in-front and deck traversal captures at 2560x1440",
    );
  } else if (interactions) {
    const objective = "JOURNAL: FIND THE GARDEN SIGN";
    const currentText = async (text) => {
      await game.waitForFunction(
        (text) => Object.values(villageEvidence.textByID).some((value) => value.includes(text)),
        text,
        {
          timeout: 20000,
        },
      );
    };
    const compact = async () => {
      await currentText(objective);
      const texts = Object.values((await state()).textByID);
      assert.ok(
        !texts.some((value) => value.includes(stationTexts[0])),
        "closed journal retained its visible inscription",
      );
      assert.ok(!texts.some((value) => value.includes("CLOSE")), "closed journal retained the close control");
    };
    const opened = async () => {
      await currentText(stationTexts[0]);
      assert.ok(
        Object.values((await state()).textByID).some((value) => value.includes("CLOSE")),
        "opened inscription has no close control",
      );
    };
    const tapJournal = async (expanded, horizontal = 0.5, vertical = 0.5) => {
      const box = await game.locator("canvas").boundingBox();
      const width = Math.min(288, box.width - 24),
        height = expanded ? 112 : 50;
      const point = {
        x: box.x + box.width / 2 + (horizontal - 0.5) * width,
        y: box.y + box.height - 22 - height + vertical * height,
      };
      const viewport = page.viewportSize();
      assert.ok(point.x > 0 && point.x < viewport.width && point.y > 0 && point.y < viewport.height);
      await tap(point.x, point.y);
    };
    assert.ok(
      Object.values(initial.textByID).some((value) => value.includes("JOURNAL: FIND THE COTTAGE SIGN")),
      "journal did not start as a compact objective",
    );
    await page.keyboard.down("Shift");
    // The painted face is 52px above its ground contact. The previous tiny
    // sign's foot-only selection would miss this actual visible target.
    await clickWorld(stations[0], 52);
    await opened();
    await questPixels(0);
    await page.keyboard.up("Shift");
    const firstRead = await state();
    assert.ok(
      Math.hypot(firstRead.position[0] - stations[0].x, firstRead.position[1] - stations[0].y) <=
        property(map.properties, "interactionRadius", 40) + 0.01,
      "reading happened before the player approached the sign",
    );
    assert.ok(Math.hypot(firstRead.position[0] - spawn.x, firstRead.position[1] - spawn.y) > 80);
    await capture("interactions-sign-read");
    await press("Escape", 1);
    await compact();
    await press("e", 1);
    await opened();
    await press("e", 1);
    await compact();
    await frames(24);
    // The nearby READ action stays in the presentation HUD, above world art.
    await currentText("READ SIGN");
    await tapJournal(false, 0.5, 0.75);
    await opened();
    await tapJournal(true, 0.9, 0.1);
    await compact();
    await tapJournal(false, 0.5, 0.75);
    await opened();
    await press("Escape", 1);
    await compact();
    await tapJournal(false, 0.4, 0.25);
    await opened();
    await tapJournal(true, 0.9, 0.1);
    await compact();
    assert.equal((await state()).cacheAsset, art.textures["treasure-closed"], "reading a sign opened the chest early");
    report.checks.push(
      "visible painted sign face tap approaches through actual map collision before reading; E reopens and closes; presentation HUD READ action and journal title/close regions all respond to actual input",
    );
    await page.setViewportSize({ width: 390, height: 844 });
    await frames(20);
    await capture("interactions-portrait-compact");
    await tapJournal(false, 0.5, 0.75);
    await opened();
    await capture("interactions-portrait-read");
    await tapJournal(true, 0.9, 0.1);
    await compact();
    const after = await state();
    assert.ok(Math.hypot(after.position[0] - firstRead.position[0], after.position[1] - firstRead.position[1]) < 0.1);
    report.checks.push(
      "390x844 portrait keeps compact objective, tappable READ action, expanded inscription and CLOSE inside the visible canvas; HUD taps do not move the player; chest remains locked until ordered hunt completes in the separate gameplay fixture",
    );
  } else {
    await page.keyboard.down("Shift");
    await clickWorld(stations[0], 12);
    await game.waitForFunction((text) => villageEvidence.texts.some((value) => value.includes(text)), stationTexts[0], {
      timeout: 20000,
    });
    await questPixels(0);
    await capture("cottage-note");
    await press("Escape", 1);
    await clickWorld(stations[1], 12);
    await game.waitForFunction((text) => villageEvidence.texts.some((value) => value.includes(text)), stationTexts[1], {
      timeout: 30000,
    });
    await questPixels(1);
    await capture("terrace");
    await press("Escape", 1);
    await clickWorld(stations[2], 20);
    await game.waitForFunction((text) => villageEvidence.texts.some((value) => value.includes(text)), stationTexts[2], {
      timeout: 20000,
    });
    await page.keyboard.up("Shift");
    await frames(4);
    assert.equal((await state()).cacheAsset, art.textures["treasure-open"]);
    // The approaching hero overlaps the chest. Walk to clear ground with
    // normal pointer input before comparing its artwork; retain tree occlusion.
    await walkTo({ x: stations[2].x + 16, y: stations[2].y + 96 });
    await questPixels(2, true);
    await capture("treasure-found");
    report.checks.push(
      "actual clue taps route through Tiled collisions, stairs and bridge; cache swaps to open texture after ordered treasure hunt",
    );
    await press("r", 1);
    assert.ok(Math.hypot((await state()).position[0] - spawn.x, (await state()).position[1] - spawn.y) < 1);
    assert.equal((await state()).cacheAsset, art.textures["treasure-closed"]);
    await page.keyboard.down("d");
    await frames(12);
    const resets = (await state()).inputResets;
    await page.locator("textarea").evaluate((e) => {
      e.hidden = false;
      e.focus();
    });
    await game.waitForFunction((n) => villageEvidence.inputResets > n, resets, { timeout: 10000 });
    assert.equal(await game.evaluate(() => document.hasFocus()), false);
    const paused = (await state()).position;
    await game.evaluate(
      () =>
        new Promise((resolve) => {
          let n = 0;
          function step() {
            if (++n >= 20) resolve();
            else requestAnimationFrame(step);
          }
          requestAnimationFrame(step);
        }),
    );
    assert.ok(Math.hypot((await state()).position[0] - paused[0], (await state()).position[1] - paused[1]) < 0.01);
    await page.keyboard.up("d");
    await page.locator("textarea").evaluate((e) => {
      e.hidden = true;
    });
    await game.locator("canvas").focus();
    await frames(4);
    await press("a", 12);
    const stair = objects.find((o) => o.class === "stair"),
      bridge = objects.find((o) => o.class === "bridge" && !o.gid);
    const river = objects.find((o) => o.name === "river-north");
    await press("r", 1);
    await walkTo({ x: bridge.x - 32, y: bridge.y + bridge.height / 2 });
    const water = { x: river.x + river.width / 2, y: bridge.y - 64 };
    await clickWorld(water);
    await frames(110);
    const bank = (await state()).position;
    assert.ok(
      Math.hypot(bank[0] - water.x, bank[1] - water.y) > 30,
      "direct movement crossed the authored river collider",
    );
    await press("r", 1);
    const cliff = objects.find((o) => o.name === "cliff-west");
    const cliffX = stair.x - 64;
    await walkTo({ x: cliffX, y: cliff.y + cliff.height + 48 });
    const upperGround = { x: cliffX, y: cliff.y - 48 };
    await walkTo(upperGround);
    assert.ok(
      Math.hypot((await state()).position[0] - upperGround.x, (await state()).position[1] - upperGround.y) < 2,
      "ground click did not route around the cliff to the upper ground",
    );
    report.checks.push("ordinary ground clicks reject blocked water and route around cliffs through the stairs");
    const moved = await state();
    assert.ok(moved.heroTextures.length >= 4 && moved.cameraUpdates > 30);
    report.checks.push(
      "R restarts at authored spawn with closed cache; real focus loss clears held input; keyboard animation and camera follow remain active",
    );
  }
  const s = await state();
  assert.ok(s.maxBatch <= 65536 && s.maxRecords <= 4096);
  assert.equal(s.world3D, 0);
  assert.equal(s.legacyText2D, 0, "gameplay recreated legacy Text2D labels");
  report.state = s;
  report.stats = await game.evaluate(() => globalThis.kartyWorldRenderer?.stats() ?? null);
  if (report.stats?.cameras) assert.equal(report.stats.cameras.length, 0, "2D scene allocated a 3D camera");
  report.framebufferSum = await game.evaluate(
    () =>
      new Promise((resolve) =>
        requestAnimationFrame(() => {
          const c = document.querySelector("canvas"),
            gl = c.getContext("webgl2");
          const pixels = new Uint8Array(32 * 32 * 4);
          gl.readPixels(
            Math.floor(c.width / 2) - 16,
            Math.floor(c.height / 2) - 16,
            32,
            32,
            gl.RGBA,
            gl.UNSIGNED_BYTE,
            pixels,
          );
          let n = 0;
          for (let i = 0; i < pixels.length; i += 4) n += pixels[i] + pixels[i + 1] + pixels[i + 2];
          resolve(n);
        }),
      ),
  );
  assert.ok(report.framebufferSum > 10000, "black 2D framebuffer");
  assert.deepEqual(report.errors, []);
  report.passed = true;
  report.checks.push("live Ebiten framebuffer, no 3D render views and no browser errors");
} catch (error) {
  report.failure = String(error);
  if (gameFrame) report.state = await gameFrame.evaluate(() => globalThis.villageEvidence).catch(() => null);
  if (gamePage)
    await gamePage
      .screenshot({ path: resolve(output, fixture + "-failure.png"), fullPage: true, timeout: 10000 })
      .catch(() => {});
  throw error;
} finally {
  clearTimeout(deadline);
  if (browser) await browser.close();
  await new Promise((r) => server.close(r));
  await writeFile(resolve(output, fixture + "-report.json"), JSON.stringify(report, null, 2) + "\n");
}
console.log(JSON.stringify({ passed: report.passed, checks: report.checks }));
