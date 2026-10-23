// Keep controller evidence independent of concurrent sample authoring/builds.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { cp, mkdir, mkdtemp, readFile, readdir, writeFile } from "node:fs/promises";
import { basename, resolve } from "node:path";

const root = process.cwd();
const source = resolve(root, "samples/willowbrook");
const directory = await mkdtemp("/tmp/karty-controllers-willowbrook-source-");
const output = resolve(root, "dist/validation/controllers/willowbrook-snapshot.json");
const excluded = new Set([".karty", "dist", ".git", ".codex", ".agents"]);
const sha = (data) => createHash("sha256").update(data).digest("hex");
const report = { passed: false, source, directory, commands: [] };
await mkdir(resolve(root, "dist/validation/controllers"), { recursive: true });
try {
  const before = await inventory(source);
  await cp(source, directory, { recursive: true, filter: (path) => !excluded.has(basename(path)) });
  const copied = await inventory(directory);
  assert.deepEqual(copied, before, "Source changed during snapshot; retry after authoring finishes");
  assert.deepEqual(await inventory(source), before, "Source changed during snapshot; retry after authoring finishes");
  report.sourceFiles = before;
  const configuration = await readFile(resolve(directory, "karty.toml"), "utf8");
  for (const name of ["controllers-topdown", "tiled"]) {
    const section = configuration.match(
      new RegExp(`^\\[packages\\.${name}\\]\\s*\\n([\\s\\S]*?)(?=^\\[|$(?![\\s\\S]))`, "m"),
    )?.[1];
    assert.ok(section, `Missing published ${name} dependency`);
    assert.doesNotMatch(section, /^\s*folder\s*=/m);
    assert.match(
      section,
      /^\s*archive\s*=\s*["']https:\/\/github.com\/karty-game\/karty-packages\/releases\/download\//m,
    );
    assert.match(section, /^\s*sha256\s*=\s*["'][a-f0-9]{64}["']/m);
  }
  await command(["package", "restore"]);
  const args = ["build", "--target", "web", "--development", "--compiler", "tinygo"];
  if (process.env.KARTY_HOST_WEB) args.push("--host", process.env.KARTY_HOST_WEB);
  await command(args);
  assert.equal(
    await readFile(resolve(directory, "karty.toml"), "utf8"),
    configuration,
    "Build changed authored package pins",
  );
  report.cartridgeSHA256 = sha(await readFile(resolve(directory, "dist/web/game.kart")));
  report.hostSHA256 = sha(await readFile(resolve(directory, "dist/web/karty-host.wasm")));
  report.passed = true;
  console.log(JSON.stringify({ directory, cartridgeSHA256: report.cartridgeSHA256 }, null, 2));
} finally {
  await writeFile(output, JSON.stringify(report, null, 2) + "\n");
}

async function inventory(base, path = "") {
  const result = {};
  for (const item of (await readdir(resolve(base, path), { withFileTypes: true })).sort((a, b) =>
    a.name.localeCompare(b.name),
  )) {
    if (excluded.has(item.name)) continue;
    const name = path ? path + "/" + item.name : item.name;
    assert.ok(!item.isSymbolicLink(), "Snapshot sources must not contain symbolic links");
    if (item.isDirectory()) Object.assign(result, await inventory(base, name));
    else if (item.isFile()) result[name] = sha(await readFile(resolve(base, name)));
  }
  return result;
}

async function command(args) {
  const entry = { args, cwd: directory };
  report.commands.push(entry);
  return new Promise((done, reject) => {
    const child = spawn(resolve(root, "dist/karty"), args, {
      cwd: directory,
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
      Object.assign(entry, { code, stdout, stderr });
      if (code === 0) done();
      else reject(new Error(`karty ${args.join(" ")} failed (${code})\n${stdout}\n${stderr}`));
    });
  });
}
