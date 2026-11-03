// Executes the actual staged reactor through its packaged browser ABI bridge.
// Host rendering is a test double; browser presentation is checked separately.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const directory = process.argv[2];
assert.ok(directory, "a staged host bundle is required");
execFileSync(
  process.execPath,
  [fileURLToPath(new URL("./host-bundle-shell.test.mjs", import.meta.url)), join(directory, "index.html")],
  {
    stdio: "inherit",
    timeout: 9000,
  },
);
const html = await readFile(join(directory, "index.html"), "utf8");
const host = await readFile(join(directory, "karty-host.wasm"));
const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");
assert.equal(/data-wasm-sha256="([a-f0-9]{64})"/.exec(html)?.[1], hash(host));
for (const match of html.matchAll(/<script(?: defer)? src="([^?"]+)\?v=([a-f0-9]+)"/g)) {
  assert.equal(match[2], hash(await readFile(join(directory, match[1]))).slice(0, 16));
}
for (const warning of ["DO NOT EDIT", "DO NOT MODIFY", "WILL BE OVERWRITTEN", "Source of truth:", "Regenerate with:"]) {
  assert.ok(html.includes(warning));
}
const protocolSource = await readFile(join(directory, "../../.karty/engine/protocol.go"), "utf8");
const protocol = Number(/protocolVersion\s*=\s*uint16\((\d+)\)/.exec(protocolSource)?.[1]);
assert.ok(Number.isInteger(protocol) && protocol > 0);
const bytes = await readFile(join(directory, "game.kart"));
const { loadCartridge } = await import(pathToFileURL(join(directory, "client.mjs")));
const submissions = [];
let validated = 0;
globalThis.kartyWebGPUValidateCartridge = async (candidate) => {
  assert.deepEqual(candidate, bytes);
  validated++;
};
globalThis.kartyHostSubmitCommands = (commands) => submissions.push(Uint8Array.from(commands));
globalThis.kartyHostLog = () => {};
globalThis.kartyHostAction = () => 0;
for (const service of [
  "PlaySound",
  "PlaySFXFrom",
  "SetSoundEmitter",
  "StopSoundEmitter",
  "SetAudioReceiver",
  "ClearAudioReceiver",
  "PlayMusic",
  "StopMusic",
  "PlayEnvironment",
  "StopEnvironment",
  "PlayVideo",
  "StopVideo",
]) {
  globalThis["kartyHost" + service] = () => {};
}
let imports;
const instantiate = WebAssembly.instantiate;
WebAssembly.instantiate = async (module, provided) => {
  assert.equal(validated, 1, "host admission must finish before instantiation");
  imports = provided;
  return instantiate(module, provided);
};
const deadline = setTimeout(() => {
  throw new Error("Host-bundle client did not complete within 9000 ms");
}, 9000);
try {
  let client;
  try {
    client = await loadCartridge(bytes);
  } finally {
    WebAssembly.instantiate = instantiate;
  }
  assert.equal(validated, 1, "host admission precedes reactor execution");
  assert.equal(submissions.length, 0);
  assert.throws(() => kartyClientUpdate(1n, new Uint8Array(24)), /not initialized/);

  function commands(frame) {
    assert.equal(submissions.length, 1, "one complete batch per lifecycle call");
    const bytes = submissions.shift();
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    assert.equal(view.getUint32(0, true), 0x4259544b);
    assert.equal(view.getUint16(4, true), protocol);
    assert.equal(view.getUint8(6), 2);
    assert.equal(view.getBigUint64(8, true), frame);
    assert.equal(view.getUint32(20, true), bytes.length - 24);
    let offset = 24;
    const tags = [];
    for (let index = 0; index < view.getUint32(16, true); index++) {
      tags.push(view.getUint8(offset));
      offset += 3 + view.getUint16(offset + 1, true);
      assert.ok(offset <= bytes.length);
    }
    assert.equal(offset, bytes.length);
    return tags;
  }

  function eventFrame(frame, pointer = false) {
    const bytes = new Uint8Array(pointer ? 36 : 29);
    const view = new DataView(bytes.buffer);
    view.setUint32(0, 0x4259544b, true);
    view.setUint16(4, protocol, true);
    view.setUint8(6, 1);
    view.setBigUint64(8, frame, true);
    view.setUint32(16, 1, true);
    view.setUint32(20, bytes.length - 24, true);
    view.setUint8(24, pointer ? 3 : 1);
    view.setUint16(25, pointer ? 9 : 2, true);
    if (pointer) {
      view.setFloat32(27, 20, true);
      view.setFloat32(31, 30, true);
      view.setUint8(35, 1);
    } else view.setUint16(27, 1, true);
    return bytes;
  }

  kartyClientInitialize();
  const initial = commands(0n);
  for (const tag of [1, 8, 10]) assert.ok(initial.includes(tag), `missing create tag ${tag}`);
  assert.throws(() => kartyClientInitialize(), /already initialized/);
  for (const frame of [1n, 2n]) {
    kartyClientUpdate(frame, eventFrame(frame, frame === 2n));
    const tags = commands(frame);
    assert.ok(tags.includes(4));
    if (frame === 2n) assert.ok(tags.includes(9));
  }
  assert.throws(() => kartyClientUpdate(3n, new Uint8Array(65537)), /Invalid cartridge memory range/);
  assert.throws(() => imports.karty.submit_commands(0xffffffff, 32), /Invalid cartridge memory range/);
  assert.equal(submissions.length, 0, "invalid guest/event ranges must never reach the host");
  const largeFrame = (1n << 60n) + 17n;
  kartyClientUpdate(largeFrame, eventFrame(largeFrame));
  commands(largeFrame);
  kartyClientShutdown();
  commands(largeFrame);
  assert.throws(() => kartyClientUpdate(4n, eventFrame(4n)), /not initialized/);
  assert.ok(client.instance.exports.memory instanceof WebAssembly.Memory);
  console.log(
    `Host-bundle launcher checks passed (actual ${process.env.KARTY_TEST_COMPILER || "tinygo"} cartridge; rendering mocked).`,
  );
} finally {
  clearTimeout(deadline);
}
