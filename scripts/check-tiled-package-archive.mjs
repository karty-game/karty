// Qualify an archive-only copy of Tiled Room using the verified published
// Tiled release. This script performs no browser work or publication.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { cp, lstat, mkdir, mkdtemp, readFile, readdir, realpath, rm, writeFile } from "node:fs/promises";
import { basename, dirname, isAbsolute, resolve, sep } from "node:path";

const root = process.cwd();
const argument = (name, fallback) => {
  const index = process.argv.indexOf(name);
  return index < 0 ? fallback : process.argv[index + 1];
};
const consumerSource = resolve(root, "testdata/tiled-room");
const archive = resolve(argument("--archive", "dist/packages/tiled-0.0.1.kpkg"));
const output = resolve("dist/validation/tiled-package/archive");
const host = process.env.KARTY_HOST_WEB ? resolve(process.env.KARTY_HOST_WEB) : undefined;
const go = process.env.KARTY_GO || "go";
const cli = resolve(root, "dist/karty");
const sha = (bytes) => createHash("sha256").update(bytes).digest("hex");
const within = (parent, child) => child === parent || child.startsWith(parent + sep);
const excluded = new Set([".karty", "dist", ".git", ".codex", ".agents"]);
const env = {
  ...process.env,
  GOFLAGS: [process.env.GOFLAGS, "-buildvcs=false"].filter(Boolean).join(" "),
  GOWORK: "off",
};
await mkdir(output, { recursive: true });
await mkdir(dirname(archive), { recursive: true });
const fixture = await mkdtemp("/tmp/karty-tiled-archive-");
const game = resolve(fixture, "game");
const report = {
  passed: false,
  fixture,
  game,
  archive,
  consumerSource,
  commands: [],
  scope: "Native asset/package import and Go WASI web compilation; no WASM execution or browser graphics.",
};
try {
  const published = (await command("go", ["run", "./cmd/karty-test-packages", "--archive", "tiled"], root)).trim();
  report.archiveSha256 = sha(await readFile(published));
  if (await exists(archive)) {
    assert.equal(
      sha(await readFile(archive)),
      report.archiveSha256,
      "Existing archive differs from the pinned published release",
    );
    report.reusedFrozenArchive = true;
  } else {
    await cp(published, archive);
    report.reusedFrozenArchive = false;
  }
  await cp(consumerSource, game, { recursive: true, filter: (name) => !excluded.has(basename(name)) });
  assert.equal(await exists(resolve(game, ".karty")), false);
  await command(cli, ["package", "import", "--replace", archive], game);
  const config = await readFile(resolve(game, "karty.toml"), "utf8");
  const section = config.match(/^\[packages\.tiled\]\s*\n([\s\S]*?)(?=^\[|$(?![\s\S]))/m)?.[1];
  assert.ok(section, "Imported configuration has no tiled package");
  assert.ok(!/^\s*folder\s*=/m.test(section), "Archive consumer retained a source-folder dependency");
  const dependency = section.match(/^\s*archive\s*=\s*(['"])([^'"]+)\1/m)?.[2];
  assert.ok(dependency && !isAbsolute(dependency), "Archive dependency must be portable and project-relative");
  report.dependencyArchive = await realpath(resolve(game, dependency));
  assert.ok(within(await realpath(game), report.dependencyArchive), "Imported archive escaped the disposable project");
  assert.equal(sha(await readFile(report.dependencyArchive)), report.archiveSha256);
  assert.equal(section.match(/^\s*sha256\s*=\s*['"]([a-f0-9]{64})['"]/m)?.[1], report.archiveSha256);
  // Discard the import's restored cache. Ordinary build must restore solely from
  // the copied archive, rather than using the source-linked game's old cache.
  await rm(resolve(game, ".karty"), { recursive: true, force: true });
  await command(cli, ["build", "--target", "web", "--bundled-runtime", ...(host ? ["--host", host] : [])], game);
  const moduleOutput = await command(go, ["list", "-m", "-json", "github.com/karty-game/tiled"], game);
  const module = JSON.parse(moduleOutput);
  assert.equal(module.Path, "github.com/karty-game/tiled");
  const moduleDirectory = await realpath(module.Dir || module.Replace?.Dir);
  const store = await realpath(resolve(game, ".karty/packages/.store"));
  assert.ok(within(store, moduleDirectory), "Go resolved the package outside the archive store");
  report.module = { path: module.Path, directory: moduleDirectory, replace: module.Replace };
  report.webDirectory = resolve(game, "dist/web");
  report.hostSha256 = sha(await readFile(resolve(report.webDirectory, "karty-host.wasm")));
  if (host) assert.equal(report.hostSha256, sha(await readFile(host)));
  report.cartridgeSha256 = sha(await readFile(resolve(report.webDirectory, "game.kart")));
  report.levels = [];
  for (const name of (await readdir(resolve(report.webDirectory, "content")))
    .filter((name) => name.endsWith(".kld"))
    .sort()) {
    report.levels.push({ name, sha256: sha(await readFile(resolve(report.webDirectory, "content", name))) });
  }
  assert.ok(report.levels.length > 0);
  assert.equal(sha(await readFile(archive)), report.archiveSha256, "Published archive changed during qualification");
  report.passed = true;
} catch (error) {
  report.failure = String(error);
  throw error;
} finally {
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
  console.log(JSON.stringify(report));
}

async function command(executable, args, cwd) {
  const child = spawn(executable, args, { cwd, env });
  let stdout = "",
    stderr = "";
  child.stdout.on("data", (bytes) => {
    stdout += bytes;
  });
  child.stderr.on("data", (bytes) => {
    stderr += bytes;
  });
  const deadline = setTimeout(() => child.kill("SIGKILL"), 120000);
  let result;
  try {
    result = await new Promise((done, reject) => {
      child.once("error", reject);
      child.once("exit", (code, signal) => done({ code, signal }));
    });
  } finally {
    clearTimeout(deadline);
  }
  report.commands.push({ executable, args, cwd, ...result });
  await writeFile(resolve(output, "commands.log"), `\n${executable} ${args.join(" ")}\n${stdout}${stderr}`, {
    flag: "a",
  });
  assert.equal(result.code, 0, `${args.join(" ")}: ${result.signal || ""}\n${stdout}${stderr}`);
  return stdout;
}
async function exists(name) {
  try {
    await lstat(name);
    return true;
  } catch (error) {
    if (error.code === "ENOENT") return false;
    throw error;
  }
}
