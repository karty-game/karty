// Qualify the SDK-distributed templates through the public CLI, from independent
// project directories. The SDK and hosts must already be installed/selected.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { lstat, mkdir, mkdtemp, readdir, readFile, rm, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = process.cwd();
const cli = resolve(root, "dist/karty");
const release = await readFile(resolve(root, "internal/release/version.go"), "utf8");
const sdk = release.match(/const CurrentSDK = "([^"]+)"/)?.[1];
assert.ok(sdk, "current SDK version is missing");
const directory = await mkdtemp("/tmp/karty-controller-starters-");
const output = resolve(root, "dist/validation/controllers/starters");
await mkdir(output, { recursive: true });
const archiveCacheDirectory = resolve(
  process.env.KARTY_PACKAGE_ARCHIVE_CACHE ||
    resolve(process.env.KARTY_TEST_PACKAGES || "dist/validation/released-packages", ".karty/packages/.downloads"),
);
const archiveCache = await loadArchiveCache();
const report = {
  passed: false,
  directory,
  sdk,
  builds: [],
  fixtureCache: {
    directory: archiveCacheDirectory,
    archives: archiveCache.map(({ name, data }) => ({ sha256: name.slice(0, -5), bytes: data.length })),
    scope:
      "Local archive bytes verified against their SHA-256 filenames; this does not establish public release availability.",
  },
  scope:
    "Public scaffolding, checksum-pinned package restoration and Go/TinyGo WASM compilation; optional verified local archive cache; no actual WASM or graphics execution.",
};
try {
  if (process.env.KARTY_HOME)
    report.sdkArchiveSHA256 = await hash(resolve(process.env.KARTY_HOME, "sdks", report.sdk, "sdk.zip"));
  for (const template of ["2d", "fps", "isometric"]) {
    const name = "starter-" + template;
    await command(["new", "--yes", "--sdk", report.sdk, "--template", template, "--gitignore", name], directory);
    const game = resolve(directory, name);
    const config = await readFile(resolve(game, "karty.toml"), "utf8");
    const source = await readFile(resolve(game, "src/main.go"), "utf8");
    if (template === "fps") {
      assert.doesNotMatch(config, /\[packages\.controllers\]/);
      assert.match(config, /\[packages\.controllers-3d\]/);
      assert.match(config, /use = "controllers-3d\.sdk"/);
      assert.match(config, /\[packages\.fps-kit\]/);
      assert.match(
        config,
        /archive = "https:\/\/github.com\/karty-game\/karty-packages\/releases\/download\/fps-kit-v/,
      );
      assert.match(source, /AttachFirstPerson/);
      assert.match(source, /\.karty\/processed\/code\/controllers-3d\.sdk\/controller/);
      assert.doesNotMatch(source, /engine\.FirstPerson|game\.AttachFirstPerson/);
      assert.match(source, /LoadLevel\(assets\.LevelRoom,/);
      assert.doesNotMatch(source, /OnKeyDown|OnKeyUp|OnInputReset|OnPointer|OnUpdate|OnLevelReady/);
    } else {
      const name = template === "2d" ? "controllers-topdown" : "controllers-isometric";
      assert.ok(config.includes(`[packages.${name}]`));
      assert.ok(config.includes(`archive = "https://github.com/karty-game/karty-packages/releases/download/${name}-v`));
      assert.ok(source.includes("github.com/karty-game/" + name));
    }
    assert.doesNotMatch(config, /^folder\s*=/m);
    assert.match(config, /^sha256 = "[a-f0-9]{64}"/m);
    for (const compiler of ["go", "tinygo"]) {
      await rm(resolve(game, ".karty"), { recursive: true, force: true });
      await seedArchiveCache(game);
      const args = ["build", "--target", "web", "--compiler", compiler];
      if (process.env.KARTY_HOST_WEB) args.push("--host", process.env.KARTY_HOST_WEB);
      await command(args, game);
      const packageName =
        template === "fps" ? "fps-kit" : template === "2d" ? "controllers-topdown" : "controllers-isometric";
      assert.ok((await readFile(resolve(game, ".karty/packages", packageName, "karty-package.toml"))).length);
      if (template === "fps") {
        const controller = await readFile(resolve(game, ".karty/packages/controllers-3d/karty-package.toml"), "utf8");
        assert.match(controller, /name = "controllers-3d"/);
        assert.ok(
          (await readdir(resolve(game, ".karty/processed/code/controllers-3d.sdk/controller"))).some((name) =>
            name.endsWith(".go"),
          ),
        );
      }
      report.builds.push({
        template,
        compiler,
        cartridgeSHA256: await hash(resolve(game, "dist/web/game.kart")),
        hostSHA256: await hash(resolve(game, "dist/web/karty-host.wasm")),
      });
    }
  }
  report.passed = true;
  console.log(`Compiled ${report.builds.length} independently scaffolded starter/compiler combinations.`);
} finally {
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
}
async function hash(path) {
  return createHash("sha256")
    .update(await readFile(path))
    .digest("hex");
}
async function loadArchiveCache() {
  let entries;
  try {
    entries = await readdir(archiveCacheDirectory, { withFileTypes: true });
  } catch (error) {
    if (error.code === "ENOENT" && !process.env.KARTY_PACKAGE_ARCHIVE_CACHE) return [];
    throw error;
  }
  const archives = [];
  for (const entry of entries.sort((a, b) => a.name.localeCompare(b.name))) {
    if (!entry.isFile() || !/^[a-f0-9]{64}\.kpkg$/.test(entry.name)) continue;
    const path = resolve(archiveCacheDirectory, entry.name);
    const info = await lstat(path);
    assert.ok(info.isFile() && info.size <= 256 << 20, `invalid fixture archive ${path}`);
    const data = await readFile(path);
    assert.equal(
      createHash("sha256").update(data).digest("hex"),
      entry.name.slice(0, -5),
      `fixture archive changed: ${path}`,
    );
    archives.push({ name: entry.name, data });
  }
  if (archives.length)
    console.log(
      `Using ${archives.length} SHA-256-verified local archives; public release availability is not checked.`,
    );
  return archives;
}
async function seedArchiveCache(game) {
  if (!archiveCache.length) return;
  const directory = resolve(game, ".karty/packages/.downloads");
  await mkdir(directory, { recursive: true });
  for (const { name, data } of archiveCache) await writeFile(resolve(directory, name), data, { flag: "wx" });
}
async function command(args, cwd) {
  console.log(`karty ${args.join(" ")}`);
  await new Promise((done, reject) => {
    const child = spawn(cli, args, {
      cwd,
      env: {
        ...process.env,
        GOWORK: "off",
        GOFLAGS: [process.env.GOFLAGS, "-buildvcs=false"].filter(Boolean).join(" "),
      },
      stdio: "inherit",
    });
    child.once("error", reject);
    child.once("exit", (code) => (code === 0 ? done() : reject(new Error(`karty ${args.join(" ")} exited ${code}`))));
  });
}
