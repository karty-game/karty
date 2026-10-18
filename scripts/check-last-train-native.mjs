// Native startup evidence: actual game WASM/UI/audio initialization, Xvfb and
// ALSA's null backend. This does not qualify physical audio or native visuals.
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const output = resolve("dist/validation/last-train/native");
await mkdir(output, { recursive: true });
const alsa = resolve(output, "alsa.conf");
await writeFile(alsa, "pcm.!default { type null }\n");
const host = resolve("samples/last-train/dist/native/karty-host");
const cartridge = resolve("samples/last-train/dist/native/game.kart");
const started = Date.now();
const child = spawn("xvfb-run", ["-a", host, "--cartridge", cartridge], {
  detached: true,
  env: { ...process.env, ALSA_CONFIG_PATH: alsa, XDG_CONFIG_HOME: resolve(output, "config") },
});
let log = "",
  failure;
child.stdout.on("data", (bytes) => {
  log += bytes;
});
child.stderr.on("data", (bytes) => {
  log += bytes;
});
const exited = new Promise((done) => {
  child.once("error", (error) => {
    failure = error;
    done();
  });
  child.once("exit", done);
});
try {
  while (!log.includes("game initialized")) {
    if (failure) throw failure;
    assert.ok(child.exitCode === null && child.signalCode === null, log);
    assert.ok(Date.now() - started < 15000, "native startup deadline: " + log);
    await new Promise((done) => setTimeout(done, 50));
  }
  const startupMs = Date.now() - started;
  await new Promise((done) => setTimeout(done, 2000));
  assert.equal(child.exitCode, null, log);
  assert.doesNotMatch(log, /panic:|fatal error:|karty-host:/);
  const digest = async (file) =>
    createHash("sha256")
      .update(await readFile(file))
      .digest("hex");
  await writeFile(
    resolve(output, "report.json"),
    JSON.stringify(
      {
        passed: true,
        startupMs,
        hostSHA256: await digest(host),
        cartridgeSHA256: await digest(cartridge),
        backend: "Linux amd64; Xvfb; ALSA null; startup only, no physical audio/visual qualification",
      },
      null,
      2,
    ) + "\n",
  );
  console.log(`Native Last Train initialized and remained running; ${startupMs} ms with null audio`);
} finally {
  if (child.pid && child.exitCode === null) {
    try {
      process.kill(-child.pid, "SIGTERM");
    } catch (error) {
      if (error.code !== "ESRCH") throw error;
    }
  }
  await exited;
  await writeFile(resolve(output, "host.log"), log);
}
