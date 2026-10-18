// Qualify ordinary karty package import/export, build and dev on independent games.
// All source edits below affect the disposable fixture, never the authored kit.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { cp, mkdir, mkdtemp, readFile, writeFile, rm } from "node:fs/promises";
import { basename, resolve } from "node:path";
import { chromium } from "playwright";

const artifacts = resolve("dist/validation/local-packages");
await mkdir(artifacts, { recursive: true });
const fixture = await mkdtemp(resolve(artifacts, "fixture-"));
await cp("testdata/local-packages/library", resolve(fixture, "library"), { recursive: true });
for (const game of ["walk", "precision"]) {
  await cp(`testdata/local-packages/games/${game}`, resolve(fixture, "games", game), {
    recursive: true,
    filter: (name) => ![".karty", "dist", "levels"].includes(basename(name)),
  });
  const directory = resolve(fixture, "games", game);
  await cp(resolve(directory, "authoring/room"), resolve(directory, "levels/room"), { recursive: true });
  const worldPath = resolve(directory, "levels/room/world.yaml");
  await writeFile(worldPath, (await readFile(worldPath, "utf8")).replaceAll("std.", "placeholder-kit."));
  await rm(resolve(directory, "authoring"), { recursive: true });
  await writeFile(resolve(directory, "go.mod"), `module example.com/stdlib-${game}\n\ngo 1.27.0\n`);
  await runCLI(["package", "import", resolve(fixture, "library")], directory);
}
const reports = [];
const prefabPath = resolve(fixture, "library/prefabs.yaml");
const controllerPath = resolve(fixture, "library/fps/controller.go");
const originalPrefab = await readFile(prefabPath, "utf8");
const originalCode = await readFile(controllerPath, "utf8");
const originalTexture = await readFile(resolve(fixture, "library/assets/wood.png"));
for (const game of ["walk", "precision"]) {
  // Separate browser lifetimes keep software rendering and startup bounded.
  // Both games still consume the same external package root.
  const browser = await chromium.launch({
    headless: true,
    timeout: 9000,
    args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  const worker = start(game);
  try {
    worker.url = await until(() =>
      worker.output.match(/Karty dev watching project files at (http:\/\/[^\s]+)/)?.[1]?.replace(/\/$/, ""),
    );
    worker.initial = await status(worker, (value) => value.state === "ready");
    const page = await browser.newPage({ viewport: { width: 960, height: 540 } });
    worker.page = page;
    worker.errors = [];
    page.on("pageerror", (error) => worker.errors.push(error.message));
    await page.goto(worker.url + "/?debug=0&worldScale=0.5&aa=false");
    await applied(worker, worker.initial);
    worker.before = await capture(worker, "before");
    assert.ok(worker.before.orange > 100, "original shared wood did not render");
    if (game === "walk") assert.ok(worker.before.cyan > 100, "game-owned material override did not render");

    // An asset-only package edit changes the compiled level and rendered wood.
    await writeFile(
      resolve(fixture, "library/assets/wood.png"),
      await readFile(resolve(fixture, "games/walk/levels/room/accent.png")),
    );
    worker.texture = await status(
      worker,
      (value) => value.state === "ready" && levelHash(value) !== levelHash(worker.initial),
    );
    await applied(worker, worker.texture);
    worker.afterTexture = await capture(worker, "texture-edit");
    assert.ok(worker.afterTexture.cyan > worker.before.cyan + 100, "shared PNG edit did not reach rendered prefab");
    assert.equal(
      worker.texture.generation.runtimeIdentity,
      worker.initial.generation.runtimeIdentity,
      "asset edit changed runtime code identity",
    );

    const taller = originalPrefab.replace("top: {c: 1}", "top: {c: 1.25}");
    assert.notEqual(taller, originalPrefab);
    await writeFile(prefabPath, taller);
    worker.geometry = await status(
      worker,
      (value) => value.state === "ready" && levelHash(value) !== levelHash(worker.texture),
    );
    await applied(worker, worker.geometry);
    await capture(worker, "prefab-edit");

    // Invalid package source must keep the exact previous served generation.
    await writeFile(prefabPath, taller + "\nunsupported: true\n");
    const failed = await status(worker, (value) => value.state === "error");
    assert.equal(failed.generation.id, worker.geometry.generation.id, "invalid package replaced working preview");
    const response = await fetch(worker.url + "/" + failed.generation.baseURL + failed.generation.levels[0].artifact);
    assert.equal(response.status, 200);
    assert.equal(sha(await response.arrayBuffer()), levelHash(failed));
    assert.equal(await page.evaluate(() => document.documentElement.dataset.kartyState), "running");
    await writeFile(prefabPath, taller);
    worker.recovered = await status(
      worker,
      (value) => value.state === "ready" && value.generation.revision > worker.geometry.generation.revision,
    );
    await applied(worker, worker.recovered);

    // Go uses the ordinary local module replacement; no package hooks run.
    const edited = originalCode.replace("RunMultiplier: 2,", "RunMultiplier: 2.2,");
    assert.notEqual(edited, originalCode);
    await writeFile(controllerPath, edited);
    worker.code = await status(
      worker,
      (value) =>
        value.state === "ready" && value.generation.runtimeIdentity !== worker.recovered.generation.runtimeIdentity,
    );
    await applied(worker, worker.code);
    assert.deepEqual(worker.errors, []);
    reports.push({
      game: worker.game,
      compiler: "tinygo",
      initial: worker.initial.generation,
      textureEdit: worker.texture.generation,
      prefabEdit: worker.geometry.generation,
      codeEdit: worker.code.generation,
      pixelCounts: { before: worker.before, afterTexture: worker.afterTexture },
      checks: [
        "external package watched",
        "PNG edit rendered",
        "prefab edit compiled and applied",
        "invalid package retains previous generation",
        "recovery",
        "shared Go edit recompiles",
      ],
      backend: "SwiftShader",
      worldScale: 0.5,
      antialiasing: false,
    });
  } finally {
    await browser.close();
    worker.process.kill("SIGTERM");
    await until(() => worker.process.exitCode !== null || worker.process.signalCode !== null);
    await writeFile(resolve(artifacts, `${worker.game}.log`), worker.output);
    await writeFile(prefabPath, originalPrefab);
    await writeFile(controllerPath, originalCode);
    await writeFile(resolve(fixture, "library/assets/wood.png"), originalTexture);
  }
}
const archive = resolve(fixture, "placeholder-kit-0.1.0.kpkg");
await runCLI(["package", "export", "--output", archive, resolve(fixture, "library")]);
const archiveSHA256 = sha(await readFile(archive));
for (const game of ["walk", "precision"]) {
  await runCLI(["package", "import", "--replace", archive], resolve(fixture, "games", game));
}
await rm(resolve(fixture, "library"), { recursive: true });
await rm(archive);
for (const game of ["walk", "precision"]) {
  const directory = resolve(fixture, "games", game);
  await rm(resolve(directory, ".karty"), { recursive: true });
  await runCLI(["package", "restore"], directory);
  await runCLI(["schema", "--check"], directory);
  for (const compiler of ["go", "tinygo"]) {
    const args = ["build", "--target", "web", "--compiler", compiler, "--bundled-runtime"];
    if (process.env.KARTY_HOST_WEB) args.push("--host", process.env.KARTY_HOST_WEB);
    await runCLI(args, directory);
    await cp(resolve(directory, "dist/web"), resolve(directory, "dist/variants", compiler), { recursive: true });
    console.log(`${game}/${compiler}: archive build passed after removing original sources and caches`);
  }
}
await runProcess(process.execPath, ["scripts/check-package-controllers.mjs"], process.cwd(), {
  ...process.env,
  KARTY_STDLIB_GAMES: resolve(fixture, "games"),
  KARTY_STDLIB_ARTIFACTS: resolve(artifacts, "controllers"),
});
await writeFile(resolve(artifacts, "report.json"), JSON.stringify({ fixture, archiveSHA256, reports }, null, 2) + "\n");
console.log(
  JSON.stringify(
    reports.map(({ game, checks, pixelCounts }) => ({ game, checks, pixelCounts })),
    null,
    2,
  ),
);

async function runCLI(args, directory = fixture) {
  return runProcess(resolve("dist/karty"), args, directory, process.env);
}
async function runProcess(executable, args, directory, env) {
  const child = spawn(executable, args, { cwd: directory, env });
  let output = "";
  child.stdout.on("data", (chunk) => {
    output += chunk;
  });
  child.stderr.on("data", (chunk) => {
    output += chunk;
  });
  const result = await new Promise((done, reject) => {
    child.once("error", reject);
    child.once("exit", (code, signal) => done({ code, signal }));
  });
  await writeFile(resolve(artifacts, "commands.log"), output, { flag: "a" });
  assert.equal(result.code, 0, `${args.join(" ")}: ${result.signal || ""}\n${output}`);
  return output;
}

function start(game) {
  const process = spawn(resolve("dist/karty"), ["dev", "--compiler", "tinygo", "--port", "0"], {
    cwd: resolve(fixture, "games", game),
    env: globalThis.process.env,
  });
  const worker = { process, game, output: "" };
  process.stdout.on("data", (chunk) => {
    worker.output += chunk;
  });
  process.stderr.on("data", (chunk) => {
    worker.output += chunk;
  });
  return worker;
}
async function until(read) {
  const deadline = Date.now() + 60000;
  while (Date.now() < deadline) {
    const value = await read();
    if (value) return value;
    await new Promise((done) => setTimeout(done, 100));
  }
  throw new Error("Package dev check timed out");
}
async function status(worker, accept) {
  return until(async () => {
    if (worker.process.exitCode !== null) throw new Error(worker.output);
    const value = await (await fetch(worker.url + "/__karty_dev/status")).json();
    return accept(value) ? value : undefined;
  });
}
async function applied(worker, status) {
  await worker.page.bringToFront();
  try {
    await worker.page.waitForFunction(
      (id) =>
        document.documentElement.dataset.kartyState === "running" &&
        globalThis.kartyDevelopmentClient?.state.applied === id,
      status.generation.id,
      { timeout: 15000, polling: 100 },
    );
  } catch (error) {
    console.error(
      worker.game,
      "expected",
      status.generation.id,
      await worker.page.evaluate(() => ({
        state: document.documentElement.dataset.kartyState,
        dev: globalThis.kartyDevelopmentClient?.state,
        text: document.body.innerText,
      })),
      worker.errors,
    );
    throw error;
  }
}
function levelHash(status) {
  return status.generation.levels[0].contentSha256;
}
function sha(value) {
  return createHash("sha256").update(new Uint8Array(value)).digest("hex");
}
async function capture(worker, label) {
  const png = await worker.page.screenshot({ fullPage: true, timeout: 9000 });
  await writeFile(resolve(artifacts, `${worker.game}-${label}.png`), png);
  return worker.page.evaluate(async (encoded) => {
    const bitmap = await createImageBitmap(await (await fetch("data:image/png;base64," + encoded)).blob());
    const canvas = new OffscreenCanvas(bitmap.width, bitmap.height);
    const context = canvas.getContext("2d");
    context.drawImage(bitmap, 0, 0);
    const pixels = context.getImageData(0, 0, bitmap.width, bitmap.height).data;
    let orange = 0,
      cyan = 0;
    for (let i = 0; i < pixels.length; i += 4) {
      const [r, g, b] = pixels.slice(i, i + 3);
      if (r > 55 && r > g * 1.3 && r > b * 1.7) orange++;
      if (g > 60 && g > r * 1.6 && b > r * 1.6) cyan++;
    }
    return { orange, cyan };
  }, png.toString("base64"));
}
