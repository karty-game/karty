// Build every current sample through the public CLI and its installed SDK.
import assert from "node:assert/strict";
import { execFileSync, spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = process.cwd();
const expectedSDK = execFileSync(resolve(root, "dist/karty"), ["sdk", "current"], { encoding: "utf8" }).trim();
assert.match(expectedSDK, /^\d+\.\d+\.\d+$/);
const guestGo = execFileSync("go", ["run", "./cmd/karty-test-sdk", "--go-path"], {
  encoding: "utf8",
  stdio: ["ignore", "pipe", "inherit"],
}).trim();
const available = (await readdir("samples", { withFileTypes: true }))
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name);
const requested = process.argv.slice(2);
for (const name of requested) assert.ok(available.includes(name), `unknown sample ${name}`);
const names = requested.length ? requested : available;
const output = resolve("dist/validation/api-migration");
await mkdir(output, { recursive: true });
const report = {
  passed: false,
  builds: [],
  scope: "Actual public CLI web builds; browser execution is a separate check.",
};
try {
  for (const name of names.sort()) {
    const directory = resolve(root, "samples", name);
    let config;
    try {
      config = await readFile(resolve(directory, "karty.toml"), "utf8");
    } catch (error) {
      if (error.code === "ENOENT" && !requested.length) continue;
      throw error;
    }
    const sdk = config.match(/\[sdk\]\s*version\s*=\s*["']([^"']+)/)?.[1];
    // Maintained samples select the released SDK and its matching host.
    assert.equal(sdk, expectedSDK, `${name} must use SDK ${expectedSDK}`);
    console.log(`Building ${name} against SDK ${sdk}`);
    const args = ["build", "--target", "web"];
    if (process.env.KARTY_HOST_WEB) args.push("--host", process.env.KARTY_HOST_WEB);
    await new Promise((done, reject) => {
      const child = spawn(resolve(root, "dist/karty"), args, {
        cwd: directory,
        stdio: "inherit",
        env: {
          ...process.env,
          KARTY_GO: guestGo,
          GOWORK: "off",
          GOFLAGS: [process.env.GOFLAGS, "-buildvcs=false"].filter(Boolean).join(" "),
        },
      });
      child.once("error", reject);
      child.once("exit", (code, signal) =>
        code === 0 ? done() : reject(new Error(`${name} failed: ${code ?? signal}`)),
      );
    });
    report.builds.push({
      name,
      sdk,
      sdkArchiveSHA256: process.env.KARTY_HOME
        ? await hash(resolve(process.env.KARTY_HOME, "sdks", sdk, "sdk.zip"))
        : undefined,
      cartridgeSHA256: await hash(resolve(directory, "dist/web/game.kart")),
      hostSHA256: await hash(resolve(directory, "dist/web/karty-host.wasm")),
    });
  }
  report.passed = true;
} finally {
  await writeFile(resolve(output, "samples.json"), JSON.stringify(report, null, 2) + "\n");
}
async function hash(path) {
  return createHash("sha256")
    .update(await readFile(path))
    .digest("hex");
}
