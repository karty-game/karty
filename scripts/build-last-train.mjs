import { spawnSync } from "node:child_process";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

await import("./generate-last-train-packs.mjs");

const directory = resolve("samples/last-train");
const cli = resolve("dist/karty");
function run(args) {
  const result = spawnSync(cli, args, { cwd: directory, stdio: "inherit", env: process.env });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
for (const name of ["dialogue", "savegame"]) {
  if (!(await readFile(resolve(directory, "karty.toml"), "utf8")).includes(`[packages.${name}]`))
    run(["package", "import", resolve("packages", name)]);
}
const target = process.argv[2] || "web";
const args = ["build", "--target", target];
if (target === "web" && process.env.KARTY_HOST_WEB) args.push("--bundled-runtime");
const host = target === "web" ? process.env.KARTY_HOST_WEB : process.env.KARTY_HOST_NATIVE;
if (host) args.push("--host", host);
if (process.env.KARTY_LAST_TRAIN_COMPILER) args.push("--compiler", process.env.KARTY_LAST_TRAIN_COMPILER);
run(args);
