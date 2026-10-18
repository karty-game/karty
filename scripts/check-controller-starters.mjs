// Qualify the SDK-distributed templates through the public CLI, from independent
// project directories. The SDK and hosts must already be installed/selected.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = process.cwd();
const cli = resolve(root, "dist/karty");
const directory = await mkdtemp("/tmp/karty-controller-starters-");
const output = resolve(root, "dist/validation/controllers/starters");
await mkdir(output, { recursive: true });
const report = {
  passed: false,
  directory,
  sdk: "0.0.13",
  builds: [],
  scope:
    "Public scaffolding, local package restoration, Go/TinyGo WASM compilation; no actual WASM or graphics execution.",
};
try {
  if (process.env.KARTY_HOME)
    report.sdkArchiveSHA256 = await hash(resolve(process.env.KARTY_HOME, "sdks", report.sdk, "sdk.zip"));
  for (const template of ["2d", "fps", "isometric"]) {
    const name = "starter-" + template;
    await command(["new", "--yes", "--sdk", report.sdk, "--template", template, "--gitignore", name], directory);
    const game = resolve(directory, name);
    const config = await readFile(resolve(game, "karty.toml"), "utf8");
    assert.match(config, /\[packages\.controllers\]/);
    assert.match(config, /folder = "packages\/controllers"/);
    assert.match(await readFile(resolve(game, "src/main.go"), "utf8"), /github.com\/karty-game\/controllers/);
    for (const compiler of ["go", "tinygo"]) {
      await rm(resolve(game, ".karty"), { recursive: true, force: true });
      const args = ["build", "--target", "web", "--compiler", compiler];
      if (process.env.KARTY_HOST_WEB) args.push("--host", process.env.KARTY_HOST_WEB);
      await command(args, game);
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
