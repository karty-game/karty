// Qualify an archive-only copy of the tracked Tiled Room consumer. Run after
// freezing packages/tiled; this script performs no browser work or publication.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { cp, lstat, mkdir, mkdtemp, readFile, readdir, realpath, rm, writeFile } from "node:fs/promises";
import { basename, dirname, isAbsolute, relative, resolve, sep } from "node:path";

const root = process.cwd();
const argument = (name, fallback) => {
  const index = process.argv.indexOf(name);
  return index < 0 ? fallback : process.argv[index + 1];
};
const source = resolve(root, "packages/tiled");
const consumerSource = resolve(root, "testdata/tiled-room");
const archive = resolve(argument("--archive", "dist/packages/tiled-0.1.0.kpkg"));
const output = resolve("dist/validation/tiled-package/archive");
const host = process.env.KARTY_HOST_WEB ? resolve(process.env.KARTY_HOST_WEB) : undefined;
const go = process.env.KARTY_GO || "go";
const cli = resolve(root, "dist/karty");
const sha = (bytes) => createHash("sha256").update(bytes).digest("hex");
const within = (parent, child) => child === parent || child.startsWith(parent + sep);
const excluded = new Set([".karty", "dist", ".git", ".codex", ".agents"]);
const env = {
  ...process.env,
  KARTY_GO: go,
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
  source,
  consumerSource,
  commands: [],
  scope: "Native asset/package import and Go WASI web compilation; no WASM execution or browser graphics.",
};
try {
  report.sourceSha256 = await sourceDigest(source);
  const prior = await optionalJSON(archive + ".source.json");
  if (await exists(archive)) {
    assert.ok(
      prior && prior.sourceSha256 === report.sourceSha256,
      "Archive exists for another or unrecorded source snapshot; choose a fresh --archive path.",
    );
    report.archiveSha256 = sha(await readFile(archive));
    assert.equal(report.archiveSha256, prior.archiveSha256, "Frozen archive bytes changed");
    report.reusedFrozenArchive = true;
  } else {
    await command(cli, ["package", "export", "--output", archive, source], root);
    report.archiveSha256 = sha(await readFile(archive));
    await writeFile(
      archive + ".source.json",
      JSON.stringify({ sourceSha256: report.sourceSha256, archiveSha256: report.archiveSha256 }, null, 2) + "\n",
    );
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
  assert.ok(!within(await realpath(source), moduleDirectory), "Go still resolves a working source-folder link");
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
  assert.equal(
    await sourceDigest(source),
    report.sourceSha256,
    "Package source changed while the archive was being qualified",
  );
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
async function optionalJSON(name) {
  try {
    return JSON.parse(await readFile(name, "utf8"));
  } catch (error) {
    if (error.code === "ENOENT") return null;
    throw error;
  }
}
async function sourceDigest(directory) {
  const files = [];
  async function visit(current) {
    for (const item of await readdir(current, { withFileTypes: true })) {
      if (excluded.has(item.name)) continue;
      const name = resolve(current, item.name);
      assert.ok(!item.isSymbolicLink(), "Portable package source contains a symlink");
      if (item.isDirectory()) await visit(name);
      else if (item.isFile()) files.push(name);
    }
  }
  await visit(directory);
  const digest = createHash("sha256");
  for (const name of files.sort()) {
    const bytes = await readFile(name);
    digest.update(relative(directory, name).split(sep).join("/") + "\0" + bytes.length + "\0");
    digest.update(bytes);
  }
  return digest.digest("hex");
}
