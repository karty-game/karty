// Real CLI consumers of a UI-only package. All edits affect disposable copies.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { createServer } from "node:http";
import { cp, mkdir, mkdtemp, readFile, realpath, rm, writeFile } from "node:fs/promises";
import { dirname, extname, relative, resolve } from "node:path";
import { chromium } from "playwright";

const artifacts = resolve("dist/validation/package-ui");
await mkdir(artifacts, { recursive: true });
const fixture = await mkdtemp(resolve(artifacts, "fixture-"));
// Freeze an override and its matching glue before any dev/build invocation.
// Other engine work may rebuild the source artifact during this qualification.
let hostSHA256;
if (process.env.KARTY_HOST_WEB) {
  const originalHost = resolve(process.env.KARTY_HOST_WEB);
  const hostDirectory = resolve(fixture, "runtime-host");
  await mkdir(hostDirectory);
  await cp(originalHost, resolve(hostDirectory, "karty-host.wasm"));
  await cp(resolve(dirname(originalHost), "wasm_exec.js"), resolve(hostDirectory, "wasm_exec.js"));
  process.env.KARTY_HOST_WEB = resolve(hostDirectory, "karty-host.wasm");
  hostSHA256 = digest(await readFile(process.env.KARTY_HOST_WEB));
}
const library = resolve(fixture, "library");
await cp("testdata/package-ui/library", library, { recursive: true });
const font = await readFile(process.env.KARTY_PACKAGE_UI_FONT || "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf");
const replacementFont = await readFile(
  process.env.KARTY_PACKAGE_UI_REPLACEMENT_FONT || "/usr/share/fonts/truetype/dejavu/DejaVuSerif.ttf",
);
assert.notEqual(digest(font), digest(replacementFont), "font edit needs distinct source bytes");
await writeFile(resolve(library, "assets/body.ttf"), font);
const manifestPath = resolve(library, "karty-package.toml");
await writeFile(
  manifestPath,
  (await readFile(manifestPath, "utf8")) + "\n[[ui.fonts]]\nrole='body'\nsource='assets/body.ttf'\n",
);
const cases = [
  { name: "one", title: "Game One", start: 10, step: 1, color: "#cc3300ff" },
  { name: "two", title: "Game Two", start: 20, step: 5, color: "#209b40ff" },
];
for (const game of cases) {
  game.directory = resolve(fixture, game.name);
  await mkdir(resolve(game.directory, "src"), { recursive: true });
  await mkdir(resolve(game.directory, "ui"));
  const module = `example.com/package-ui-${game.name}`;
  let source = await readFile("testdata/package-ui/game/main.go.tmpl", "utf8");
  for (const [key, value] of Object.entries({ MODULE: module, TITLE: game.title, START: game.start, STEP: game.step }))
    source = source.replaceAll(`{{${key}}}`, String(value));
  await writeFile(resolve(game.directory, "src/main.go"), source);
  await writeFile(resolve(game.directory, "go.mod"), `module ${module}\n\ngo 1.27.0\n`);
  await writeFile(
    resolve(game.directory, "karty.toml"),
    `[project]\nname='ui-${game.name}'\n[project.resolution]\nwidth=960\nheight=640\n[sdk]\nversion='0.0.13'\n`,
  );
  await cp("testdata/package-ui/game/game-screen.kui", resolve(game.directory, "ui/game-screen.kui"));
  await writeFile(resolve(game.directory, "ui/theme.toml"), `extends='dark'\n[colors]\nprimary='${game.color}'\n`);
  await cli(["package", "import", library], game.directory);
}
const reports = [];
const badgePath = resolve(library, "ui/hud-badge.kui");
const original = await readFile(badgePath, "utf8");
const texturePath = resolve(library, "assets/panel.png");
const texture = await readFile(texturePath);
for (const game of cases) {
  const worker = start(["dev", "--compiler", "tinygo", "--port", "0", "--bind", "127.0.0.1"], game.directory);
  const browser = await launch();
  try {
    const url = await until(
      () => worker.output.match(/Karty dev watching project files at (http:\/\/[^\s]+)/)?.[1]?.replace(/\/$/, ""),
      worker,
    );
    const status = async (accept) =>
      until(async () => {
        const value = await (await fetch(url + "/__karty_dev/status")).json();
        return accept(value) ? value : null;
      }, worker);
    const initial = await status((v) => v.state === "ready");
    const page = await openUI(browser, url, game);
    await assertUI(page, game, `${game.start}`);
    const pixels = await capture(page, `${game.name}-folder`, game);
    await clickButton(page, pixels);
    await assertUI(page, game, `${game.start + game.step}`);
    await writeFile(badgePath, original.replace(">Change</button>", ">Apply</button>"));
    const template = await status((v) => v.state === "ready" && v.generation.id !== initial.generation.id);
    await applied(page, template);
    assert.equal(
      template.generation.runtimeIdentity,
      initial.generation.runtimeIdentity,
      "static package UI edit changed guest code",
    );
    await assertUI(page, game, `${game.start + game.step}`);
    await writeFile(texturePath, await readFile("testdata/local-packages/library/assets/wood.png"));
    const image = await status((v) => v.state === "ready" && v.generation.id !== template.generation.id);
    await applied(page, image);
    // Global image assets use the ordinary guest reload policy. UI templates
    // and fonts use the existing UI resource update path and retain state.
    assert.notEqual(image.generation.runtimeIdentity, initial.generation.runtimeIdentity);
    await assertUI(page, game, `${game.start}`);
    const changed = await capture(page, `${game.name}-image-edit`, game);
    assert.notEqual(changed.sha256, pixels.sha256, "package image edit did not change framebuffer");
    await clickButton(page, changed);
    await assertUI(page, game, `${game.start + game.step}`);
    await writeFile(resolve(library, "assets/body.ttf"), replacementFont);
    const fontEdit = await status((v) => v.state === "ready" && v.generation.id !== image.generation.id);
    await applied(page, fontEdit);
    assert.equal(fontEdit.generation.runtimeIdentity, image.generation.runtimeIdentity, "font edit changed guest code");
    await assertUI(page, game, `${game.start + game.step}`);
    await writeFile(badgePath, "invalid package UI");
    const failed = await status((v) => v.state === "error");
    assert.equal(failed.generation.id, fontEdit.generation.id, "invalid package UI replaced served generation");
    await assertUI(page, game, `${game.start + game.step}`);
    await writeFile(badgePath, original);
    const recovered = await status((v) => v.state === "ready" && v.generation.revision > fontEdit.generation.revision);
    await applied(page, recovered);
    await writeFile(
      badgePath,
      original.replace(
        "func setup(props HudBadgeProps) {}",
        'func setup(props HudBadgeProps) { props.Title += " shared" }',
      ),
    );
    const code = await status(
      (v) => v.state === "ready" && v.generation.runtimeIdentity !== recovered.generation.runtimeIdentity,
    );
    await applied(page, code);
    await assertUI(page, { ...game, title: game.title + " shared" }, `${game.start}`);
    assert.deepEqual(page.packageErrors, []);
    reports.push({
      game: game.name,
      mode: "folder",
      compiler: "tinygo",
      initial: initial.generation,
      template: template.generation,
      image: image.generation,
      font: fontEdit.generation,
      code: code.generation,
      pixels,
      checks: [
        "typed child props",
        "game-owned callback/state",
        "consumer theme override",
        "shared UI/font edits retain state",
        "shared image uses ordinary reload",
        "invalid UI retention/recovery",
        "shared setup code reload",
      ],
    });
  } finally {
    await browser.close();
    worker.child.kill("SIGTERM");
    await until(() => worker.child.exitCode !== null || worker.child.signalCode !== null);
    await writeFile(resolve(artifacts, `${game.name}-dev.log`), worker.output);
    await writeFile(badgePath, original);
    await writeFile(texturePath, texture);
    await writeFile(resolve(library, "assets/body.ttf"), font);
  }
}
const archive = resolve(fixture, "hud-kit.kpkg");
await cli(["package", "export", "--output", archive, library], fixture);
const archiveSHA256 = digest(await readFile(archive));
for (const game of cases) await cli(["package", "import", "--replace", archive], game.directory);
await rm(library, { recursive: true });
await rm(archive);
for (const game of cases) {
  await rm(resolve(game.directory, ".karty"), { recursive: true });
  await cli(["package", "restore"], game.directory);
  for (const compiler of ["go", "tinygo"]) {
    await cli(["build", "--target", "web", "--compiler", compiler, "--bundled-runtime"], game.directory);
    const cartridge = await readFile(resolve(game.directory, "dist/web/game.kart"));
    const stagedHostSHA256 = digest(await readFile(resolve(game.directory, "dist/web/karty-host.wasm")));
    if (hostSHA256) assert.equal(stagedHostSHA256, hostSHA256);
    const module = await WebAssembly.compile(cartridge);
    const ui = WebAssembly.Module.customSections(module, "karty.ui.v1");
    assert.equal(ui.length, 1);
    assert.deepEqual(assetNames(ui[0]), ["hud-kit.badge", "ui.game-screen"], "reachable child/unused UI stripping");
    const server = await serve(resolve(game.directory, "dist/web"));
    const browser = await launch();
    try {
      const page = await openUI(browser, `http://127.0.0.1:${server.address().port}`, game);
      await assertUI(page, game, `${game.start}`);
      const pixels = await capture(page, `${game.name}-${compiler}-archive`, game);
      await clickButton(page, pixels);
      await assertUI(page, game, `${game.start + game.step}`);
      assert.deepEqual(page.packageErrors, []);
      reports.push({
        game: game.name,
        mode: "archive",
        compiler,
        cartridgeSHA256: digest(cartridge),
        hostSHA256: stagedHostSHA256,
        pixels,
        checks: [
          "clean restore without originals",
          "typed child props/callback",
          "consumer theme override",
          "package image/font",
          "unused UI stripped",
        ],
      });
    } finally {
      await browser.close();
      server.closeAllConnections();
      await new Promise((done) => server.close(done));
    }
  }
}
await writeFile(
  resolve(artifacts, "report.json"),
  JSON.stringify(
    {
      fixture,
      cliSHA256: digest(await readFile(resolve("dist/karty"))),
      hostSHA256,
      archiveSHA256,
      backend: "SwiftShader",
      worldScale: 0.5,
      antialiasing: false,
      input: "synthesized mouse",
      reports,
    },
    null,
    2,
  ) + "\n",
);
console.log(`Package UI passed ${reports.length} folder/archive browser cases: ${fixture}`);

function digest(bytes) {
  return createHash("sha256").update(new Uint8Array(bytes)).digest("hex");
}
function launch() {
  return chromium.launch({
    headless: true,
    timeout: 9000,
    args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
}
function start(args, directory) {
  const child = spawn(resolve("dist/karty"), args, { cwd: directory, env: process.env });
  const worker = { child, output: "" };
  child.stdout.on("data", (b) => (worker.output += b));
  child.stderr.on("data", (b) => (worker.output += b));
  return worker;
}
async function cli(args, directory) {
  const worker = start(args, directory);
  const code = await new Promise((done, reject) => {
    worker.child.once("error", reject);
    worker.child.once("exit", done);
  });
  await writeFile(resolve(artifacts, "commands.log"), worker.output, { flag: "a" });
  assert.equal(code, 0, worker.output);
  return worker.output;
}
async function until(read, worker) {
  const end = Date.now() + 60000;
  while (Date.now() < end) {
    const value = await read();
    if (value) return value;
    if (worker && (worker.child.exitCode !== null || worker.child.signalCode !== null)) throw new Error(worker.output);
    await new Promise((done) => setTimeout(done, 50));
  }
  throw new Error("Timed out: " + (worker?.output || ""));
}
async function openUI(browser, url, game) {
  const page = await browser.newPage({ viewport: { width: 960, height: 640 } });
  page.setDefaultTimeout(15000);
  page.packageErrors = [];
  page.on("pageerror", (e) => page.packageErrors.push(e.message));
  await page.addInitScript(() => {
    globalThis.packageUI = { assets: [], values: {} };
    let submit;
    Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
      configurable: true,
      get: () => submit,
      set(original) {
        submit = (bytes) => {
          const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
          const text = (at, n) => new TextDecoder().decode(bytes.subarray(at, at + n));
          for (let at = 24; at < bytes.length;) {
            const tag = view.getUint8(at),
              size = view.getUint16(at + 1, true),
              p = at + 3;
            if (tag === 18 || tag === 23) {
              const offset = tag === 18 ? 8 : 20;
              packageUI.assets.push(text(p + offset + 2, view.getUint16(p + offset, true)));
            }
            if (tag === 20)
              packageUI.values[view.getUint32(p, true) + ":" + view.getUint32(p + 4, true)] = text(
                p + 11,
                view.getUint16(p + 9, true),
              );
            at = p + size;
          }
          return original(bytes);
        };
      },
    });
  });
  await page.goto(url + "/?debug=0&worldScale=0.5&aa=false");
  await assertUI(page, game, `${game.start}`);
  return page;
}
async function assertUI(page, game, value) {
  await page.waitForFunction(
    ({ title, value }) =>
      document.documentElement.dataset.kartyState === "running" &&
      packageUI.assets.includes("hud-kit.badge") &&
      Object.values(packageUI.values).includes(title) &&
      Object.values(packageUI.values).includes("Value " + value),
    { title: game.title, value },
    { timeout: 15000, polling: 100 },
  );
}
async function applied(page, status) {
  await page.waitForFunction(
    (id) =>
      document.documentElement.dataset.kartyState === "running" &&
      globalThis.kartyDevelopmentClient?.state.applied === id,
    status.generation.id,
    { timeout: 15000, polling: 100 },
  );
}
async function clickButton(page, pixels) {
  await page.mouse.move(pixels.x, pixels.y);
  await page.mouse.down();
  await page.waitForTimeout(150);
  await page.mouse.up();
  await page.waitForTimeout(200);
}
async function capture(page, name, game) {
  const bytes = await page.screenshot({ fullPage: true, timeout: 9000 });
  await writeFile(resolve(artifacts, name + ".png"), bytes);
  const bounds = await page.evaluate(
    async ({ image, color }) => {
      const bitmap = await createImageBitmap(await (await fetch("data:image/png;base64," + image)).blob());
      const canvas = new OffscreenCanvas(bitmap.width, bitmap.height),
        ctx = canvas.getContext("2d");
      ctx.drawImage(bitmap, 0, 0);
      const data = ctx.getImageData(0, 0, bitmap.width, bitmap.height).data;
      let minX = bitmap.width,
        minY = bitmap.height,
        maxX = 0,
        maxY = 0,
        count = 0;
      for (let y = 0; y < bitmap.height; y++)
        for (let x = 0; x < bitmap.width; x++) {
          const at = (y * bitmap.width + x) * 4,
            r = data[at],
            g = data[at + 1],
            b = data[at + 2];
          if (Math.abs(r - color[0]) < 5 && Math.abs(g - color[1]) < 5 && Math.abs(b - color[2]) < 5) {
            count++;
            minX = Math.min(minX, x);
            maxX = Math.max(maxX, x);
            minY = Math.min(minY, y);
            maxY = Math.max(maxY, y);
          }
        }
      return { x: (minX + maxX) / 2, y: (minY + maxY) / 2, count };
    },
    { image: bytes.toString("base64"), color: [1, 3, 5].map((at) => parseInt(game.color.slice(at, at + 2), 16)) },
  );
  assert.ok(bounds.count > 500, "consumer theme button color absent from framebuffer");
  return { ...bounds, sha256: digest(bytes) };
}
function assetNames(buffer) {
  const data = new DataView(buffer),
    bytes = new Uint8Array(buffer),
    names = [];
  let at = 16;
  for (let i = 0; i < data.getUint32(8, true); i++) {
    const n = data.getUint16(at, true),
      size = data.getUint32(at + 4, true);
    at += 8;
    names.push(new TextDecoder().decode(bytes.subarray(at, at + n)));
    at += n + size;
  }
  return names;
}
async function serve(directory) {
  const root = await realpath(directory);
  const server = createServer(async (req, res) => {
    try {
      const file = await realpath(
        resolve(root, "." + new URL(req.url, "http://test").pathname.replace(/\/$/, "/index.html")),
      );
      if (relative(root, file).startsWith("..")) throw new Error("escape");
      res.writeHead(200, {
        "Content-Type":
          { ".html": "text/html", ".js": "text/javascript", ".wasm": "application/wasm", ".kart": "application/wasm" }[
            extname(file)
          ] || "application/octet-stream",
      });
      res.end(await readFile(file));
    } catch {
      res.writeHead(404);
      res.end();
    }
  });
  await new Promise((done, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", done);
  });
  return server;
}
