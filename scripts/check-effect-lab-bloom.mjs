// Run from the CLI root. Engine overrides are optional explicit artifacts, not dependencies.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { cp, mkdir, mkdtemp, readFile, realpath, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { dirname, extname, join, resolve, sep } from "node:path";
import { launchEffectLabBrowser } from "./effect-lab-browser.mjs";
import { prepareEffectLabPreview } from "./prepare-effect-lab-preview.mjs";

await mkdir("tmp", { recursive: true });
const output = await mkdtemp(resolve("tmp/effect-lab-bloom-check-"));
const distribution = join(output, "web");
await cp(resolve(process.env.KARTY_EFFECT_LAB_DISTRIBUTION ?? "samples/effect-lab/dist/web"), distribution, {
  recursive: true,
});
if (process.env.KARTY_EFFECT_LAB_HOST) {
  const host = resolve(process.env.KARTY_EFFECT_LAB_HOST);
  await cp(host, join(distribution, "karty-host.wasm"));
  await cp(join(dirname(host), "wasm_exec.js"), join(distribution, "wasm_exec.js"));
}
await prepareEffectLabPreview(distribution);
const hash = (body) => createHash("sha256").update(body).digest("hex");
const assets = {
  hostSha256: hash(await readFile(join(distribution, "karty-host.wasm"))),
  cartridgeSha256: hash(await readFile(join(distribution, "game.kart"))),
};
const server = createServer(async (request, response) => {
  try {
    const name = decodeURIComponent(new URL(request.url, "http://localhost").pathname);
    const file = await realpath(resolve(distribution, "." + (name === "/" ? "/index.html" : name)));
    const type = {
      ".html": "text/html",
      ".js": "text/javascript",
      ".json": "application/json",
      ".wasm": "application/wasm",
      ".kart": "application/wasm",
      ".kld": "application/wasm",
      ".kaud": "application/octet-stream",
    }[extname(file)];
    if (!type || !file.startsWith(distribution + sep)) throw Error("Outside distribution");
    const body = await readFile(file);
    response.writeHead(200, { "Content-Type": type, "Cache-Control": "no-store", "X-Karty-Hash": hash(body) });
    response.end(body);
  } catch {
    response.writeHead(404).end();
  }
});
await new Promise((done, reject) => {
  server.once("error", reject);
  server.listen(0, "127.0.0.1", done);
});
let browser;
try {
  browser = await launchEffectLabBrowser();
  const session = await browser.newBrowserCDPSession();
  const { gpu } = await session.send("SystemInfo.getInfo");
  assert.equal(gpu.featureStatus.webgl, "enabled");
  assert.doesNotMatch(JSON.stringify(gpu.devices), /swiftshader|llvmpipe|softpipe/i);
  const errors = [];
  const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
  page.setDefaultTimeout(5000);
  page.setDefaultNavigationTimeout(5000);
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  await page.addInitScript((value) => {
    globalThis.kartyRomanEvidenceAssets = value;
  }, assets);
  await page.goto(`http://127.0.0.1:${server.address().port}/?effects=1&freezeTime=4`);
  await page.waitForFunction(
    () =>
      document.documentElement.dataset.kartyState === "running" &&
      globalThis.kartyWorldRenderer?.metadata().cameras.length === 1,
  );
  assert.equal(await page.evaluate(() => typeof globalThis.kartyBloomSpike), "undefined");
  assert.equal(await page.locator("#bloom-spike").count(), 0);
  const frames = async (count = 8) => {
    const start = await page.evaluate(() => kartyDebugWidget().frames);
    await page.waitForFunction(({ start, count }) => kartyDebugWidget().frames >= start + count, { start, count });
  };
  const device = () => page.evaluate(() => kartyWorldRenderer.stats().cameras[0].device);
  const apply = async (options) => {
    assert.equal(await page.evaluate((value) => kartyWorldRenderer.setOptions(value), options), null);
    await frames();
  };
  const capture = async (name) => {
    await frames();
    const data = await page.evaluate(
      () =>
        new Promise((resolve, reject) => {
          const timer = setTimeout(() => reject(Error("Canvas export exceeded 5 seconds")), 5000);
          requestAnimationFrame(() => {
            clearTimeout(timer);
            resolve(document.querySelector("canvas").toDataURL("image/png"));
          });
        }),
    );
    const body = Buffer.from(data.slice(data.indexOf(",") + 1), "base64");
    await writeFile(join(output, name + ".png"), body);
    return hash(body);
  };
  await page.locator("#karty-debug-tab-settings").click();
  await page.selectOption("#karty-debug-setting-profile", "medium");
  await page.locator("#karty-debug-tab-effects").click();
  await page.locator("#karty-effects-preset-Clean").click();
  const baseline = await capture("clean");
  await page.locator("#karty-effects-preset-Bloom").click();
  await page.waitForFunction(() => kartyWorldRenderer.stats().cameras[0].device.bloomActive);
  const approved = await capture("bloom");
  assert.notEqual(approved, baseline);
  const initial = await device();
  assert.equal(initial.bloomPasses, 6);
  assert.equal(initial.bloomError, "");
  for (const [quality, passes] of [
    [0, 2],
    [1, 4],
    [2, 6],
  ]) {
    await page.selectOption("#karty-effect-bloomQuality", String(quality));
    await frames();
    assert.equal((await device()).bloomPasses, passes);
  }
  for (const view of [1, 2]) {
    await page.selectOption("#karty-effect-bloomView", String(view));
    await frames();
    assert.notEqual(await capture("bloom-view-" + view), approved);
  }
  await apply({ bloomView: 0, bloomFormat: 1 });
  await capture("bloom-rgba8");
  assert.ok((await device()).bloomBytes < initial.bloomBytes);
  await apply({ bloomFormat: 0 });
  const revisions = (await device()).resourceRevisions;
  await apply({ bloomThreshold: 0.45, bloomKnee: 0.2, bloomRadius: 1.2, bloomScatter: 0.6, bloomIntensity: 0.8 });
  assert.equal((await device()).resourceRevisions, revisions);
  await apply({ effectsEnabled: false });
  assert.equal((await device()).bloomActive, false);
  assert.equal(await capture("master-bypass"), baseline);
  await apply({ effectsEnabled: true, bloomIntensity: 0 });
  assert.equal((await device()).bloomActive, false);
  await apply({ bloomIntensity: 1, materialView: "normals" });
  assert.equal((await device()).bloomActive, false);
  await apply({ materialView: "shaded" });
  assert.equal((await device()).bloomActive, true);
  await apply({ colorGrade: true, gradeExposure: -0.1, outline: true });
  await page.waitForFunction(
    () =>
      kartyWorldRenderer.stats().cameras[0].device.outlineActive &&
      !kartyWorldRenderer.stats().cameras[0].device.gradePending,
  );
  await capture("bloom-grade-outline");

  // The same public lab benchmark now measures bloom and its formats/depths.
  await page.waitForFunction(() => typeof globalThis.kartyEffectLabBenchmark === "function");
  const benchmark = await page.evaluate(() => kartyEffectLabBenchmark({ warmup: 8, frames: 24, repeats: 1 }));
  for (const name of ["bloom", "bloom-compact", "bloom-balanced", "bloom-rgba8", "stack"]) {
    const row = benchmark.rows.find((entry) => entry.name === name);
    assert.equal(row.renderStats.bloomActive, true);
    assert.equal(row.renderStats.bloomError, "");
    assert.equal(row.resourceRevisions, 0);
  }
  await writeFile(join(output, "performance.json"), JSON.stringify(benchmark, null, 2));
  await page.locator("#karty-effects-preset-Bloom").click();
  const layouts = [];
  for (const width of [393, 320]) {
    await page.setViewportSize({ width, height: 851 });
    await frames();
    await page.locator("#karty-effect-bloom").scrollIntoViewIfNeeded();
    const layout = await page
      .locator("#karty-debug-widget")
      .evaluate((panel) => ({ width: innerWidth, overflow: panel.scrollWidth > panel.clientWidth }));
    assert.equal(layout.overflow, false);
    layouts.push(layout);
    await page.screenshot({ path: join(output, "mobile-" + width + ".png"), timeout: 5000 });
  }
  await page.close();

  // Unsupported float targets must leave the game alive and support explicit RGBA8.
  const unsupported = await browser.newPage({ viewport: { width: 800, height: 600 } });
  unsupported.setDefaultTimeout(5000);
  unsupported.setDefaultNavigationTimeout(5000);
  unsupported.on("pageerror", (error) => errors.push(error.message));
  await unsupported.addInitScript(() => {
    const prototype = WebGL2RenderingContext.prototype,
      original = prototype.getExtension;
    prototype.getExtension = function (name) {
      return /^(EXT_color_buffer_float|EXT_color_buffer_half_float)$/.test(name)
        ? null
        : Reflect.apply(original, this, [name]);
    };
  });
  await unsupported.goto(`http://127.0.0.1:${server.address().port}/?effects=1&bloom=1`);
  await unsupported.waitForFunction(
    () =>
      document.documentElement.dataset.kartyState === "running" &&
      kartyWorldRenderer.stats().cameras[0].device.bloomError.includes("RGBA16F"),
  );
  assert.equal(await unsupported.locator("#karty-debug-bloom-status").isVisible(), true);
  await unsupported.selectOption("#karty-effect-bloomFormat", "1");
  await unsupported.waitForFunction(
    () =>
      kartyWorldRenderer.stats().cameras[0].device.bloomActive &&
      kartyWorldRenderer.stats().cameras[0].device.bloomError === "",
  );
  await unsupported.close();
  const restored = await browser.newPage({ viewport: { width: 800, height: 600 } });
  restored.setDefaultTimeout(5000);
  restored.setDefaultNavigationTimeout(5000);
  restored.on("pageerror", (error) => errors.push(error.message));
  await restored.goto(`http://127.0.0.1:${server.address().port}/?effects=1&bloom=1`);
  await restored.waitForFunction(() => globalThis.kartyWorldRenderer?.stats().cameras[0]?.device?.bloomActive);
  const contextRestore = await restored.evaluate(() => {
    const canvas = document.querySelector("canvas"),
      extension = canvas.getContext("webgl2").getExtension("WEBGL_lose_context");
    if (!extension) return { available: false };
    const timeOrigin = performance.timeOrigin;
    // The browser host's existing Ebitengine policy reloads on context loss.
    extension.loseContext();
    return { available: true, mode: "page reload", timeOrigin };
  });
  if (contextRestore.available) {
    await restored.waitForFunction((before) => {
      const device = globalThis.kartyWorldRenderer?.stats().cameras[0]?.device;
      return (
        performance.timeOrigin > before.timeOrigin &&
        document.documentElement.dataset.kartyState === "running" &&
        device?.bloomActive &&
        device.bloomError === ""
      );
    }, contextRestore);
  }
  await restored.close();
  assert.deepEqual(errors, []);
  await writeFile(
    join(output, "report.json"),
    JSON.stringify(
      {
        assets,
        gpu,
        layouts,
        contextRestore,
        errors,
        scope:
          "Actual desktop hardware WebGL2, not physical Pixel 6 FPS; benchmark windows retain five-second deadlines.",
      },
      null,
      2,
    ),
  );
  console.log("Bloom checks passed:", output);
} finally {
  await browser?.close();
  await new Promise((done) => server.close(done));
}
