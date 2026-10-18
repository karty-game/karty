// Export a frozen package copy, delete that publisher, and compile a fresh
// consumer from only the copied archive. No browser or publication is performed.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { cp, mkdir, mkdtemp, readFile, realpath, rm, writeFile } from "node:fs/promises";
import { resolve, sep } from "node:path";

const root = process.cwd();
const cli = resolve(root, "dist/karty");
const fixture = await mkdtemp("/tmp/karty-controllers-archive-");
const publisher = resolve(fixture, "publisher");
const game = resolve(fixture, "game");
const archive = resolve(fixture, "controllers-0.1.0.kpkg");
const output = resolve(root, "dist/validation/controllers/archive");
await mkdir(output, { recursive: true });
const report = {
  passed: false,
  fixture,
  commands: [],
  builds: [],
  scope: "Archive restoration and Go/TinyGo WASM compilation; no WASM execution or graphics.",
};
const digest = async (path) =>
  createHash("sha256")
    .update(await readFile(path))
    .digest("hex");
try {
  await cp(resolve(root, "packages/controllers"), publisher, { recursive: true });
  await cp(resolve(root, "testdata/controllers-consumer"), game, {
    recursive: true,
    filter: (path) => !path.split(sep).some((part) => [".karty", "dist"].includes(part)),
  });
  await command(cli, ["package", "export", "--output", archive, publisher], root);
  report.archiveSHA256 = await digest(archive);
  await command(cli, ["package", "import", archive], game);
  await rm(publisher, { recursive: true });
  await rm(archive);
  await rm(resolve(game, ".karty"), { recursive: true, force: true });
  const config = await readFile(resolve(game, "karty.toml"), "utf8");
  assert.ok(config.includes("[packages.controllers]"));
  assert.ok(!/^folder\s*=/m.test(config), "consumer retained linked source");
  for (const compiler of ["go", "tinygo"]) {
    const args = ["build", "--target", "web", "--compiler", compiler];
    if (process.env.KARTY_HOST_WEB) args.push("--host", process.env.KARTY_HOST_WEB);
    await command(cli, args, game);
    report.builds.push({
      compiler,
      cartridgeSHA256: await digest(resolve(game, "dist/web/game.kart")),
      hostSHA256: await digest(resolve(game, "dist/web/karty-host.wasm")),
    });
    // Each compiler must independently restore from the retained game archive.
    const module = JSON.parse(await command("go", ["list", "-m", "-json", "github.com/karty-game/controllers"], game));
    const directory = await realpath(module.Dir || module.Replace.Dir);
    const store = await realpath(resolve(game, ".karty/packages/.store"));
    assert.ok(directory.startsWith(store + sep), "module resolved outside the archive store");
    await rm(resolve(game, ".karty"), { recursive: true, force: true });
  }
  report.passed = true;
  console.log(JSON.stringify(report.builds, null, 2));
} finally {
  await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
}

async function command(program, args, cwd) {
  const entry = { program, args, cwd };
  report.commands.push(entry);
  const began = Date.now();
  return new Promise((done, reject) => {
    const child = spawn(program, args, {
      cwd,
      env: {
        ...process.env,
        GOWORK: "off",
        GOFLAGS: [process.env.GOFLAGS, "-buildvcs=false"].filter(Boolean).join(" "),
      },
      stdio: ["ignore", "pipe", "pipe"],
    });
    let stdout = "",
      stderr = "";
    child.stdout.on("data", (data) => {
      stdout += data;
    });
    child.stderr.on("data", (data) => {
      stderr += data;
    });
    child.once("error", reject);
    child.once("exit", (code) => {
      entry.code = code;
      entry.elapsedMs = Date.now() - began;
      entry.stdout = stdout;
      entry.stderr = stderr;
      if (code !== 0) reject(new Error(`${program} ${args.join(" ")} failed (${code})\n${stdout}\n${stderr}`));
      else done(stdout);
    });
  });
}
