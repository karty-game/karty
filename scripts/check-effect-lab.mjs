// Run from the CLI root after building samples/effect-lab with the intended host.
// Optional distribution/host overrides test an unchanged cartridge with a new engine.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { cp, mkdir, mkdtemp, readFile, realpath, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { dirname, extname, join, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { launchEffectLabBrowser } from "./effect-lab-browser.mjs";
import { prepareEffectLabPreview } from "./prepare-effect-lab-preview.mjs";

const lensOnly = process.argv.includes("--lens");
const outlineOnly = process.argv.includes("--outline");
const outlineFailureOnly = process.argv.includes("--outline-failure");
const colorOnly = process.argv.includes("--color") || outlineOnly || lensOnly || outlineFailureOnly;
const presetsOnly = process.argv.includes("--presets-only");
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const sample = join(root, "samples/effect-lab");
await mkdir(join(root, "tmp"), { recursive: true });
const output = await mkdtemp(join(root, "tmp/effect-lab-check-"));
const distribution = join(output, "web");
await cp(resolve(process.env.KARTY_EFFECT_LAB_DISTRIBUTION ?? join(sample, "dist/web")), distribution, {
  recursive: true,
  dereference: false,
});
if (process.env.KARTY_EFFECT_LAB_HOST) {
  const host = resolve(process.env.KARTY_EFFECT_LAB_HOST);
  await cp(host, join(distribution, "karty-host.wasm"));
  await cp(join(dirname(host), "wasm_exec.js"), join(distribution, "wasm_exec.js"));
}
await prepareEffectLabPreview(distribution);
const hash = (data) => createHash("sha256").update(data).digest("hex");
const provenance = {};
for (const name of ["game.kart", "karty-host.wasm", "wasm_exec.js"]) {
  provenance[name] = hash(await readFile(join(distribution, name)));
}
const responses = [];
const server = createServer(async (request, response) => {
  try {
    const pathname = decodeURIComponent(new URL(request.url, "http://localhost").pathname);
    const target = await realpath(resolve(distribution, "." + (pathname === "/" ? "/index.html" : pathname)));
    if (!target.startsWith(distribution + sep)) throw new Error("outside distribution");
    const data = await readFile(target);
    response.writeHead(200, {
      "Content-Type":
        { ".html": "text/html", ".js": "text/javascript", ".wasm": "application/wasm", ".kart": "application/wasm" }[
          extname(target)
        ] ?? "application/octet-stream",
      "Cache-Control": "no-store",
      "X-Karty-Hash": hash(data),
    });
    response.end(data);
  } catch {
    response.writeHead(404).end();
  }
});
await new Promise((done, reject) => {
  server.once("error", reject);
  server.listen(0, "127.0.0.1", done);
});
let browser;
const errors = [];
try {
  browser = await launchEffectLabBrowser();
  const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
  page.setDefaultTimeout(5000);
  page.setDefaultNavigationTimeout(5000);
  if (outlineFailureOnly) {
    await page.addInitScript(() => {
      const fixture = { armed: false, pending: false, hits: 0 };
      globalThis.kartyOutlineFailureFixture = fixture;
      const prototype = WebGL2RenderingContext.prototype;
      const allocate = prototype.texImage2D;
      const getError = prototype.getError;
      prototype.texImage2D = function (...args) {
        if (fixture.armed && args[2] === 0x8229) {
          fixture.armed = false;
          fixture.pending = true;
          fixture.hits++;
          return;
        }
        return Reflect.apply(allocate, this, args);
      };
      prototype.getError = function () {
        if (fixture.pending) {
          fixture.pending = false;
          return 0x0505;
        }
        return Reflect.apply(getError, this, []);
      };
    });
  }
  const gpuSession = await browser.newBrowserCDPSession();
  const { gpu } = await gpuSession.send("SystemInfo.getInfo");
  assert.equal(gpu.featureStatus.webgl, "enabled", "WebGL hardware acceleration unavailable");
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });
  page.on("response", (r) => {
    const name = new URL(r.url()).pathname.slice(1);
    if (name in provenance)
      responses.push(
        r.finished().then((error) => {
          assert.equal(error, null);
          assert.equal(r.headers()["x-karty-hash"], provenance[name]);
          return name;
        }),
      );
  });
  await page.goto(`http://127.0.0.1:${server.address().port}/?effects=1&freezeTime=4`);
  await page.waitForFunction(
    () =>
      document.documentElement.dataset.kartyState === "running" &&
      globalThis.kartyWorldRenderer?.metadata().cameras.length === 1,
    null,
    { timeout: 5000 },
  );
  const frames = async (count = 3) => {
    const start = await page.evaluate(() => String(globalThis.kartyFrame));
    await page.waitForFunction(
      ({ start, count }) =>
        globalThis.kartyFrame >= BigInt(start) + BigInt(count) &&
        globalThis.kartyWorldRenderer.stats().cameras.every((camera) => camera.device?.gradePending !== true),
      { start, count },
      { timeout: 5000 },
    );
  };
  const options = () => page.evaluate(() => globalThis.kartyWorldRenderer.metadata().options);
  const set = async (values) => {
    assert.equal(await page.evaluate((v) => globalThis.kartyWorldRenderer.setOptions(v), values), null);
    await frames();
  };
  const canvas = page.locator("canvas").first();
  const capture = async (name) => {
    await frames();
    const widget = page.locator("#karty-debug-widget");
    await widget.evaluate((e) => (e.style.visibility = "hidden"));
    let data;
    try {
      const encoded = await page.evaluate(
        () =>
          new Promise((resolve, reject) => {
            const timer = setTimeout(() => reject(new Error("Canvas export exceeded 5 seconds")), 5000);
            requestAnimationFrame(() => {
              clearTimeout(timer);
              resolve(document.querySelector("canvas").toDataURL("image/png"));
            });
          }),
      );
      assert.ok(encoded.startsWith("data:image/png;base64,"), "canvas export failed");
      data = Buffer.from(encoded.slice("data:image/png;base64,".length), "base64");
    } finally {
      await widget.evaluate((e) => (e.style.visibility = ""));
    }
    await writeFile(join(output, name + ".png"), data);
    return hash(data);
  };
  await set({ worldScale: 1 });
  await page.locator("#karty-effects-preset-Clean").click();
  const baseline = await capture("clean");
  const ditherHashes = [];
  let lensColdActivation = null;
  let outlineFailure = null;
  if (!presetsOnly && !colorOnly) {
    // The panel must not capture the camera; canvas input still moves it.
    await canvas.click({ position: { x: 200, y: 250 } });
    const pose = await page.evaluate(() => globalThis.kartyWorldRenderer.metadata().cameras[0].position);
    await page.keyboard.down("w");
    await frames(12);
    await page.keyboard.up("w");
    await page.keyboard.press("Escape");
    await frames();
    const moved = await page.evaluate(() => globalThis.kartyWorldRenderer.metadata().cameras[0].position);
    assert.notDeepEqual(moved, pose, "FPS movement did not reach the host");
    for (const [key, expected] of [
      ["2", [0, 4.5, 1.65]],
      ["3", [-6.8, -6.2, 1.65]],
    ]) {
      await page.keyboard.press(key);
      await frames();
      const actual = await page.evaluate(() => globalThis.kartyWorldRenderer.metadata().cameras[0].position);
      expected.forEach((value, index) =>
        assert.ok(Math.abs(actual[index] - value) < 0.001, `viewpoint ${key}: expected ${expected}, got ${actual}`),
      );
      await capture("viewpoint-" + key);
    }
    await page.keyboard.press("1");
    await frames();

    await page.locator("#karty-effect-quantize").check();
    await page.locator("#karty-effect-dither").check();
    await page.locator("#karty-effect-colorLevels").evaluate((input) => {
      input.value = "4";
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
    for (let mode = 0; mode < 4; mode++) {
      await page.locator("#karty-effect-ditherMode").selectOption(String(mode));
      assert.equal((await options()).ditherMode, mode);
      ditherHashes.push(await capture(`dither-${mode}`));
    }
    assert.equal(new Set(ditherHashes).size, 4, "dither methods must produce different pixels");
    for (const [mode, name] of [
      [1, "rgb565"],
      [2, "ega16"],
      [3, "green4"],
    ]) {
      await page.locator("#karty-effect-quantizeMode").selectOption(String(mode));
      assert.notEqual(await capture(name), baseline);
    }
    await page.locator("#karty-effect-effectsEnabled").uncheck();
    assert.equal(await capture("bypass"), baseline, "master bypass must restore the clean scene");
    for (const effect of ["vignette", "scanlines", "chromatic", "grain", "fog"]) {
      await page.locator("#karty-effects-preset-Clean").click();
      await page.locator("#karty-effect-" + effect).check();
      assert.notEqual(await capture("isolated-" + effect), baseline);
    }
  }
  if (!colorOnly) {
    await page.locator("#karty-effects-preset-CRT").click();
    assert.notEqual(await capture("crt"), baseline);
    await page.screenshot({ path: join(output, "controls.png") });
    await page.locator("#karty-effects-preset-Handheld").click();
    assert.notEqual(await capture("handheld"), baseline);
    await page.locator("#karty-effects-preset-Hazy").click();
    assert.notEqual(await capture("fog"), baseline);
    if (presetsOnly) {
      for (const preset of ["Bloom", "Warm", "Cool", "Ink"]) {
        await page.locator("#karty-effects-preset-" + preset).click();
        assert.notEqual(await capture("preset-" + preset.toLowerCase()), baseline);
      }
    }
  }
  await page.locator("#karty-effects-preset-Clean").click();
  if (!presetsOnly && !colorOnly) {
    await page.locator("#karty-effect-grain").check();
    const grain = await capture("grain");
    assert.equal(await capture("grain-frozen"), grain);
    await set({ freezeTime: 4.1 });
    assert.notEqual(await capture("grain-next"), grain);
    const before = await options();
    assert.equal(
      typeof (await page.evaluate(() =>
        globalThis.kartyWorldRenderer.setOptions({ grainStrength: 0.1, colorLevels: 99 }),
      )),
      "string",
    );
    assert.deepEqual(await options(), before, "invalid objects must not partially publish");
    await page.locator("#karty-effects-preset-Clean").click();
  }
  if (lensOnly || (!presetsOnly && !colorOnly)) {
    for (const effect of ["lensDistortion", "sharpen", "lensSurface"])
      assert.equal(await page.locator("#karty-effect-" + effect).count(), 1);
    await set({ lensDistortion: true, lensAmount: 0.3 });
    const barrel = await capture("lens-barrel");
    assert.notEqual(barrel, baseline);
    await set({ lensAmount: -0.25 });
    const pincushion = await capture("lens-pincushion");
    assert.notEqual(pincushion, barrel);
    await set({ lensAmount: 0.3, lensCenterX: 0.25, lensCenterY: 0.65 });
    assert.notEqual(await capture("lens-offset"), barrel);
    await page.locator("#karty-effects-preset-Clean").click();
    await set({ sharpen: true });
    assert.notEqual(await capture("sharpen-native"), baseline);
    await set({ sharpen: false, worldScale: 0.5 });
    const low = await capture("sharpen-base-half");
    await set({ sharpen: true, sharpness: 0.5 });
    assert.notEqual(await capture("sharpen-half"), low);
    await set({ worldScale: 1 });
    await page.locator("#karty-effects-preset-Clean").click();
    lensColdActivation = await page.locator("#karty-effect-lensSurface").evaluate(
      (input) =>
        new Promise((resolve, reject) => {
          const api = globalThis.kartyWorldRenderer;
          const before = [],
            after = [];
          let toggleAt = null,
            dispatchMs = null,
            raf;
          const timer = setTimeout(() => {
            cancelAnimationFrame(raf);
            reject(new Error("Cold lens activation sampling exceeded 5 seconds"));
          }, 5000);
          const tick = () => {
            try {
              const device = api.stats().cameras[0]?.device;
              if (!device || !Number.isFinite(device.cpuMs)) throw new Error("Device CPU statistics unavailable");
              const sample = {
                atMs: performance.now(),
                frame: device.frames,
                cpuMs: device.cpuMs,
                resourceRevisions: device.resourceRevisions,
              };
              if (toggleAt === null) {
                before.push(sample);
                if (before.length === 4) {
                  if (input.checked) throw new Error("Lens surface was already active before cold measurement");
                  toggleAt = performance.now();
                  input.click();
                  dispatchMs = performance.now() - toggleAt;
                }
              } else {
                after.push(sample);
                if (after.length >= 8 && sample.frame >= before.at(-1).frame + 6) {
                  clearTimeout(timer);
                  const initial = before.at(-1);
                  const activationIndex = after.findIndex((row) => row.resourceRevisions > initial.resourceRevisions);
                  const activation = after[activationIndex];
                  const previous = after[activationIndex - 1] ?? initial;
                  const stableBaseline = before.every((row) => row.resourceRevisions === initial.resourceRevisions);
                  const attributed =
                    stableBaseline &&
                    activation?.frame === previous.frame + 1 &&
                    activation.resourceRevisions === initial.resourceRevisions + 1;
                  const samples = [...before, ...after];
                  resolve({
                    scope: "First lens-surface toggle on a fresh renderer device; composition shader already warmed",
                    attribution: attributed
                      ? "Observed atlas resource creation in one device frame"
                      : "Unavailable: resource transition was absent, ambiguous, or its first frame was missed",
                    firstActivationRendererCpuMs: attributed ? activation.cpuMs : null,
                    toggleToObservedActivationMs: activation ? activation.atMs - toggleAt : null,
                    uiDispatchMs: dispatchMs,
                    rafIntervalsMs: samples.slice(1).map((row, index) => row.atMs - samples[index].atMs),
                    before,
                    after,
                    limits:
                      "Renderer CPU includes texture allocation and GL upload enqueue plus all other renderer work; immutable PNG data is decoded at host startup, before interactive frames; it does not isolate upload or GPU completion. RAF intervals include browser scheduling and observation overhead. Single cold observation; no warm-performance or mobile claim, and no 16.67 ms pass/fail gate.",
                  });
                  return;
                }
              }
              raf = requestAnimationFrame(tick);
            } catch (error) {
              clearTimeout(timer);
              reject(error);
            }
          };
          raf = requestAnimationFrame(tick);
        }),
    );
    const modes = [];
    for (const [mode, name] of [
      [0, "water"],
      [1, "dirt"],
      [2, "blood"],
    ]) {
      await page.locator("#karty-effect-lensSurfaceMode").selectOption(String(mode));
      const shot = await capture("lens-" + name);
      assert.notEqual(shot, baseline);
      modes.push(shot);
    }
    assert.equal(new Set(modes).size, 3);
    await page.locator("#karty-effect-lensSurfaceMode").selectOption("0");
    const surfaceBeforeSpecular = await options();
    await set({ lensSurfaceStrength: 1, lensCoverage: 1, lensSize: 0.5, lensRefraction: 16 });
    const waterSpecular = [];
    for (const value of [0, 1]) {
      await page.locator("#karty-effect-lensSpecular").evaluate((input, value) => {
        input.value = String(value);
        input.dispatchEvent(new Event("input", { bubbles: true }));
      }, value);
      await frames();
      assert.equal((await options()).lensSpecular, value, "Water specular slider did not update its setting");
      waterSpecular.push(await capture("lens-water-specular-" + value));
    }
    assert.notEqual(waterSpecular[0], waterSpecular[1], "Water specular slider did not change highlights");
    assert.notEqual(waterSpecular[0], baseline, "Disabling water specular removed the water surface");
    await page.locator("#karty-effect-lensSpecular").evaluate((input) => {
      input.value = "0.35";
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await frames();
    assert.equal((await options()).lensSpecular, 0.35);
    await set({
      lensSurfaceStrength: surfaceBeforeSpecular.lensSurfaceStrength,
      lensCoverage: surfaceBeforeSpecular.lensCoverage,
      lensSize: surfaceBeforeSpecular.lensSize,
      lensRefraction: surfaceBeforeSpecular.lensRefraction,
    });
    await set({ lensSurfaceMode: 0, lensRefraction: 0 });
    assert.notEqual(await capture("lens-water-no-refraction"), modes[0], "Water did not refract the scene");
    await set({ lensRefraction: 8, lensDistortion: true, lensAmount: 0.12, sharpen: true });
    assert.notEqual(await capture("lens-stack"), modes[0]);
    await page.getByLabel("Jump to effect", { exact: true }).selectOption("lensSurface");
    await page.screenshot({ path: join(output, "lens-controls.png"), clip: { x: 0, y: 0, width: 1280, height: 720 } });
    await set({ effectsEnabled: false });
    assert.equal(await capture("lens-bypass"), baseline);
    const before = await options();
    const invalid = await page.evaluate(() =>
      globalThis.kartyWorldRenderer.setOptions({ sharpness: 0.5, lensAmount: 0.8 }),
    );
    assert.match(invalid, /lensAmount/);
    assert.deepEqual(await options(), before);
    await page.locator("#karty-effects-preset-Clean").click();
    await set({
      lensDistortion: true,
      sharpen: true,
      lensSurface: true,
      lensAmount: 0,
      sharpness: 0,
      lensSurfaceStrength: 0,
    });
    assert.equal(await capture("lens-neutral"), baseline);
    await page.locator("#karty-effects-preset-Clean").click();
  }
  if (outlineFailureOnly) {
    await page.locator("#karty-effect-vignette").check();
    const vignette = await capture("outline-failure-vignette-reference");
    assert.notEqual(vignette, baseline);
    const before = await page.evaluate(() => globalThis.kartyWorldRenderer.stats().cameras[0].device);
    await page.evaluate(() => {
      globalThis.kartyOutlineFailureFixture.armed = true;
    });
    await page.locator("#karty-effect-outline").check();
    await frames(6);
    const failed = await page.evaluate(() => globalThis.kartyWorldRenderer.stats().cameras[0].device);
    assert.equal(await page.evaluate(() => document.documentElement.dataset.kartyState), "running");
    assert.ok(failed.frames > before.frames, "renderer stopped after outline mask allocation failed");
    assert.match(failed.outlineError, /mask/i);
    assert.match(failed.outlineError, /(?:0x)?0?505/i);
    assert.equal(failed.outlineActive, false);
    assert.equal(failed.outlineBytes, 0, "failed outline retained render targets");
    assert.equal(
      await capture("outline-failure-vignette-preserved"),
      vignette,
      "outline setup failure changed another effect or the scene",
    );
    await page.waitForFunction(
      () =>
        document
          .getElementById("karty-debug-effects-status")
          .textContent.startsWith("Geometry outlines unavailable:") &&
        document.getElementById("karty-debug-effects-status").textContent.includes("Toggle off/on to retry."),
      null,
      { timeout: 5000 },
    );
    const status = page.locator("#karty-debug-outline-status");
    await status.scrollIntoViewIfNeeded();
    assert.equal(await status.isVisible(), true);
    await page.screenshot({ path: join(output, "outline-failure-controls.png") });
    const warning = await status.textContent();
    assert.match(warning, /^Geometry outlines unavailable:/);
    await frames(8);
    const held = await page.evaluate(() => globalThis.kartyWorldRenderer.stats().cameras[0].device);
    assert.ok(held.frames > failed.frames);
    assert.equal(held.resourceRevisions, failed.resourceRevisions, "failed outline retried allocation every frame");
    assert.equal(held.outlineError, failed.outlineError);
    const fixture = await page.evaluate(() => globalThis.kartyOutlineFailureFixture);
    assert.deepEqual(fixture, { armed: false, pending: false, hits: 1 });
    await page.locator("#karty-effect-outline").uncheck();
    await frames();
    await page.locator("#karty-effect-outline").check();
    await page.waitForFunction(
      () => {
        const device = globalThis.kartyWorldRenderer.stats().cameras[0].device;
        return device.outlineActive && device.outlineError === "";
      },
      null,
      { timeout: 5000 },
    );
    const recovered = await page.evaluate(() => globalThis.kartyWorldRenderer.stats().cameras[0].device);
    assert.ok(recovered.outlineBytes > 0);
    assert.notEqual(await capture("outline-failure-recovered"), vignette, "outline retry did not affect pixels");
    await page.waitForFunction(() => document.getElementById("karty-debug-effects-status").textContent === "", null, {
      timeout: 5000,
    });
    assert.equal(await status.isVisible(), false);
    assert.equal(await page.evaluate(() => globalThis.kartyOutlineFailureFixture.hits), 1);
    outlineFailure = { vignette, warning, fixture, before, failed, held, recovered };
    await page.locator("#karty-effects-preset-Clean").click();
  }
  if (!lensOnly && !outlineFailureOnly && (colorOnly || !presetsOnly)) {
    const slider = async (name, value) => {
      await page.locator("#karty-effect-" + name).evaluate((input, value) => {
        input.value = String(value);
        input.dispatchEvent(new Event("input", { bubbles: true }));
      }, value);
      await frames();
    };
    if (!outlineOnly) {
      await page.locator("#karty-effects-preset-CRT").click();
      assert.notEqual(await capture("crt-regression"), baseline);
      await page.locator("#karty-effects-preset-Clean").click();
      await page.locator("#karty-effect-saturation").check();
      await page.locator("#karty-effect-contrast").check();
      await page.locator("#karty-effect-colorGrade").check();
      assert.equal(await capture("color-neutral"), baseline, "neutral color controls must bypass exactly");
      await page.locator("#karty-effects-preset-Clean").click();
      await page.locator("#karty-effect-saturation").check();
      await slider("saturationAmount", 0);
      assert.notEqual(await capture("desaturated"), baseline);
      await slider("saturationAmount", 1.5);
      const saturationHashes = [];
      for (let method = 0; method < 2; method++) {
        await page.locator("#karty-effect-saturationMode").selectOption(String(method));
        saturationHashes.push(await capture("saturation-" + method));
      }
      assert.notEqual(saturationHashes[0], saturationHashes[1]);
      await page.locator("#karty-effects-preset-Clean").click();
      await page.locator("#karty-effect-contrast").check();
      const contrastHashes = [];
      for (const value of [0.75, 1.35]) {
        await slider("contrastAmount", value);
        contrastHashes.push(await capture("contrast-" + value));
      }
      assert.notEqual(contrastHashes[0], contrastHashes[1]);
      for (const preset of ["Warm", "Cool", "Ink"]) {
        await page.locator("#karty-effects-preset-" + preset).click();
        assert.notEqual(await capture("preset-" + preset.toLowerCase()), baseline);
      }
    }
    await page.locator("#karty-effects-preset-Clean").click();
    await page.locator("#karty-effect-outline").check();
    await page.getByLabel("Jump to effect", { exact: true }).selectOption("outline");
    await frames();
    await page.screenshot({
      path: join(output, "outline-controls.png"),
      clip: { x: 0, y: 0, width: 1280, height: 720 },
    });
    const thin = await capture("outline-thin");
    assert.notEqual(thin, baseline);
    await slider("outlineAngle", 5);
    assert.notEqual(await capture("outline-low-angle"), baseline);
    await slider("outlineAngle", 40);
    await slider("outlineWidth", 2);
    const wide = await capture("outline-wide");
    assert.notEqual(wide, thin);
    const features = [];
    for (const [method, name] of [
      [1, "depth"],
      [2, "creases"],
    ]) {
      await page.locator("#karty-effect-outlineMode").selectOption(String(method));
      const image = await capture("outline-" + name);
      assert.notEqual(image, baseline);
      features.push(image);
    }
    assert.notEqual(features[0], features[1], "depth and normal-only contours must differ");
    assert.notEqual(wide, features[0], "combined contours must include the room's normal creases");
    await page.locator("#karty-effects-preset-Ink").click();
    await page.locator("#karty-effect-quantize").check();
    await page.locator("#karty-effect-dither").check();
    await slider("colorLevels", 8);
    assert.notEqual(await capture("ink-quantized"), baseline);
    await page.locator("#karty-effect-effectsEnabled").uncheck();
    assert.equal(await capture("color-bypass"), baseline);
    const before = await options();
    assert.equal(
      typeof (await page.evaluate(() =>
        globalThis.kartyWorldRenderer.setOptions({ gradeOffsetBlue: 0.02, outlineWidth: 7 }),
      )),
      "string",
    );
    assert.deepEqual(await options(), before, "invalid grading/outline update partially published");
    await page.locator("#karty-effects-preset-Clean").click();
  }
  await page.keyboard.press("1");
  await frames();
  await page.evaluate(() => globalThis.kartyDebugWidget(false));
  await page.addStyleTag({ content: "#karty-debug-widget{display:none!important}" });
  await page.setViewportSize({ width: 960, height: 540 });
  await frames(6);
  await page.screenshot({ path: join(sample, "preview.jpg"), type: "jpeg", quality: 88 });
  assert.equal(await page.evaluate(() => globalThis.kartyWorldRenderer.stats().error), null);
  const grading = await page.evaluate(() => {
    const device = globalThis.kartyWorldRenderer.stats().cameras[0].device;
    return {
      pending: device.gradePending ?? null,
      lastFrameBuildMs: device.gradeBuildMs ?? null,
      lastFrameBuildNodes: device.gradeBuildNodes ?? null,
      gradeGPU: device.gradeGPU ?? null,
      gradeBakeError: device.gradeBakeError ?? null,
      scope: "Captures wait for atomic grading publication; build diagnostics describe only the final frame slice",
    };
  });
  assert.notEqual(grading.pending, true, "color grading did not finish before capture");
  assert.deepEqual(errors, []);
  assert.deepEqual(new Set(await Promise.all(responses)), new Set(Object.keys(provenance)));
  await writeFile(
    join(output, "report.json"),
    JSON.stringify(
      {
        provenance,
        browser: browser.version(),
        baseline,
        ditherHashes,
        lensColdActivation,
        outlineFailure,
        grading,
        checks: outlineFailureOnly
          ? "One-shot R8 outline mask allocation OOM; actual WASM frame continuation, exact retained vignette pixels, UI failure status, no repeated allocations, successful checkbox retry, hardware provenance"
          : lensOnly
            ? "Hardware lens mapping, off-center warp, native/half-resolution sharpening, water/grime/blood, refraction, water specular slider, combined optics, controls, exact neutral/bypass, atomic rejection and provenance"
            : outlineOnly
              ? "Geometry outline width, low angle, depth/crease modes, UI navigation, stacked quantization, exact bypass, atomic rejection, artifact provenance"
              : colorOnly
                ? "Color neutral identity, saturation methods, contrast ranges, grade presets, geometry outline modes and width, stacked quantization, legacy CRT, exact bypass, atomic rejection, artifact provenance"
                : presetsOnly
                  ? "WASM lifecycle, GPU preset pixels, UI preset controls, artifact provenance"
                  : "WASM lifecycle, GPU pixels, UI controls, bypass, frozen grain, validation rollback, FPS movement",
        backend: "Hardware-accelerated Chromium; synthesized input; no mobile FPS claim",
        gpu,
        errors,
      },
      null,
      2,
    ) + "\n",
  );
  console.log(`Effect lab browser checks passed: ${output}`);
} finally {
  await browser?.close();
  await new Promise((done) => server.close(done));
}
