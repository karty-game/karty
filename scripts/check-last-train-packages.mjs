// Build independent consumers of published dialogue/savegame archives with both
// cartridge compilers. No publisher source checkout is required.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { cp, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { basename, resolve } from "node:path";

const output = resolve("dist/validation/last-train/packages");
await mkdir(output, { recursive: true });
const fixture = await mkdtemp(resolve(output, "fixture-"));
const game = resolve(fixture, "game");
await cp("samples/last-train", game, {
  recursive: true,
  filter: (name) => ![".karty", "dist"].includes(basename(name)),
});
const manifest = await readFile(resolve(game, "karty.toml"), "utf8");
await writeFile(resolve(game, "karty.toml"), manifest.split("\n[packages.")[0]);
const digest = (bytes) => createHash("sha256").update(bytes).digest("hex");
const report = { fixture, archives: [], consumers: [] };
for (const name of ["dialogue", "savegame"]) {
  const archive = resolve(fixture, name + ".kpkg");
  const published = (
    await command("go", ["run", "./cmd/karty-test-packages", "--archive", name], process.cwd())
  ).trim();
  await cp(published, archive);
  report.archives.push({ name, sha256: digest(await readFile(archive)) });
  await cli(["package", "import", archive], game);
  await rm(archive);
}
await rm(resolve(game, ".karty"), { recursive: true });
await cli(["package", "restore"], game);
const unifiedFiles = [
  "go.mod",
  "model.go",
  "reveal.go",
  "stage.go",
  "program.go",
  "story.go",
  "story_snapshot.go",
  "binary.go",
  "strings.go",
  "snapshot.go",
  "cmd/yarn-processor/main.go",
  "compiler/Program.cs",
  "compiler/packages.lock.json",
  "global.json",
  "ui/novel-dialogue.kui",
  "ui/novel-history.kui",
  "ui/layouts/novel-frame.kui",
];
for (const name of unifiedFiles)
  assert.ok(
    (await readFile(resolve(game, ".karty/packages/dialogue", name))).length,
    `missing unified export: ${name}`,
  );
const restoredManifest = await readFile(resolve(game, ".karty/packages/dialogue/karty-package.toml"), "utf8");
assert.equal((restoredManifest.match(/^\[\[processors\]\]/gm) || []).length, 1);
assert.equal((restoredManifest.match(/^\[\[ui\.components\]\]/gm) || []).length, 4);
const consumerManifest = await readFile(resolve(game, "karty.toml"), "utf8");
assert.deepEqual([...consumerManifest.matchAll(/^\[packages\.([^\]]+)\]/gm)].map((m) => m[1]).sort(), [
  "dialogue",
  "savegame",
]);
assert.ok(!(await readFile(resolve(game, "go.mod"), "utf8")).includes("github.com/karty-game/yarn"));
report.unifiedDialogueExports = unifiedFiles;
for (const compiler of ["go", "tinygo"]) {
  const args = ["build", "--target", "web", "--compiler", compiler, "--bundled-runtime"];
  if (process.env.KARTY_HOST_WEB) args.push("--host", process.env.KARTY_HOST_WEB);
  await cli(args, game);
  const distribution = resolve(game, "dist/web");
  await command(process.execPath, ["scripts/check-last-train-web.mjs", distribution, "--quick"], process.cwd(), {
    ...process.env,
    KARTY_LAST_TRAIN_OUTPUT: resolve(output, compiler),
  });
  const browser = JSON.parse(await readFile(resolve(output, compiler, "report.json"), "utf8"));
  assert.ok(browser.passed);
  report.consumers.push({ compiler, ...browser });
}
await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
console.log("Last Train archives passed clean restore, real Yarn compilation and Go/TinyGo browser play");

function cli(args, cwd) {
  return command(resolve("dist/karty"), args, cwd);
}
async function command(executable, args, cwd, env = process.env) {
  const child = spawn(executable, args, { cwd, env });
  let log = "";
  let stdout = "";
  child.stdout.on("data", (bytes) => {
    log += bytes;
    stdout += bytes;
  });
  child.stderr.on("data", (bytes) => {
    log += bytes;
  });
  const code = await new Promise((done, reject) => {
    child.once("error", reject);
    child.once("exit", done);
  });
  await writeFile(resolve(output, "commands.log"), log, { flag: "a" });
  assert.equal(code, 0, log);
  return stdout;
}
