// Executes real passive chunk WASM; browser graphics and the Go host are absent.
import assert from "node:assert/strict";
import { createHash, webcrypto } from "node:crypto";
import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { runInNewContext } from "node:vm";

const source = await readFile(process.argv[2], "utf8");
const fixture = new Uint8Array(await readFile(process.argv[3]));
const substituted = new Uint8Array(await readFile(join(dirname(process.argv[3]), "chunk-other.kld")));
const envelope = new Uint8Array(await readFile(join(dirname(process.argv[3]), "envelope.bin")));
const digest = (bytes) => createHash("sha256").update(bytes).digest("hex");
const flipped = fixture.slice();
flipped[flipped.length - 1] ^= 1;
for (const wrong of [flipped, substituted]) {
  assert.equal(wrong.length, fixture.length, "checksum cases preserve artifact size");
  assert(WebAssembly.validate(wrong), "checksum cases remain valid WASM modules");
  assert.notEqual(digest(wrong), digest(fixture));
}
let bytes = fixture,
  responseMode = "stream",
  requests = [],
  callbacks = [],
  instances = 0,
  compilations = 0,
  readers = 0;
let delayedResolve;
const context = {
  console: { error() {}, warn() {} },
  AbortController,
  TextDecoder,
  Uint8Array,
  DataView,
  URL,
  crypto: webcrypto,
  Go: class {
    constructor() {
      throw new Error("No game host in chunk bridge fixture");
    }
  },
  WebAssembly: {
    compile: async (...args) => {
      compilations++;
      return WebAssembly.compile(...args);
    },
    Module: WebAssembly.Module,
    instantiate: async (...args) => {
      instances++;
      return WebAssembly.instantiate(...args);
    },
  },
  fetch: async (url, options) => {
    requests.push(url);
    assert.equal(url, `content/${digest(bytes)}.kld`, "chunk paths must derive solely from the approved digest");
    const captured = bytes;
    if (responseMode === "delayed")
      await new Promise((resolve) => {
        delayedResolve = resolve;
      });
    let read = false;
    return {
      ok: responseMode !== "http",
      status: 503,
      headers: {
        get: (name) =>
          name === "Content-Encoding"
            ? responseMode === "compressed"
              ? "gzip"
              : null
            : responseMode === "header" || responseMode === "compressed"
              ? String(captured.length + 1)
              : null,
      },
      ...(responseMode === "fallback"
        ? {}
        : {
            body: {
              getReader: () => {
                readers++;
                return {
                  read: async () => {
                    if (read) return { done: true };
                    read = true;
                    if (responseMode === "oversize") return { done: false, value: new Uint8Array(captured.length + 1) };
                    if (responseMode === "truncated")
                      return { done: false, value: captured.subarray(0, captured.length - 1) };
                    if (responseMode === "hash") {
                      return { done: false, value: flipped };
                    }
                    if (responseMode === "substitute") return { done: false, value: substituted };
                    return { done: false, value: captured };
                  },
                  cancel: async () => {},
                  releaseLock() {},
                };
              },
            },
          }),
      arrayBuffer: async () => captured.buffer.slice(captured.byteOffset, captured.byteOffset + captured.byteLength),
    };
  },
  kartyHostChunkLoaded: (id, data) => callbacks.push({ id, data }),
  kartyHostChunkFailed: (id, code, message) => callbacks.push({ id, code, message }),
};
runInNewContext(source, context);
const settle = async () => {
  for (let index = 0; index < 12; index++) await new Promise((resolve) => setImmediate(resolve));
};
const completed = async () => {
  const deadline = Date.now() + 1000;
  while (!callbacks.length && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 1));
};
const request = async (mode, expectedError) => {
  const beforeCompilations = compilations;
  const beforeInstances = instances;
  const beforeReaders = readers;
  callbacks = [];
  responseMode = mode;
  context.kartyRequestChunk(1, digest(bytes), bytes.length, 2);
  await completed();
  assert.equal(callbacks.length, 1, mode);
  if (expectedError) {
    assert.match(callbacks[0].message, expectedError);
    assert.equal("data" in callbacks[0], false, "invalid chunks never invoke hostChunkLoaded");
    if (mode !== "stream") {
      assert.equal(compilations, beforeCompilations, `${mode} rejects before WASM compilation`);
      assert.equal(instances, beforeInstances, `${mode} rejects before WASM instantiation`);
    }
    if (mode === "header") assert.equal(readers, beforeReaders, "wrong declared size rejects before reading the body");
  } else assert.deepEqual(callbacks[0].data, envelope, "executed ABI must return the original envelope");
};
await request("stream");
await request("fallback", /require a readable response stream/);
await request("compressed");
assert.equal(instances, 2, "valid chunks must execute real WASM");
await request("http", /HTTP 503/);
await request("header", /size does not match/);
await request("oversize", /exceeds/);
await request("truncated", /size does not match/);
await request("hash", /hash does not match/);
await request("substitute", /hash does not match/);

for (const [hash, size, version] of [
  ["../escape", 8, 2],
  [digest(bytes).toUpperCase(), 8, 2],
  [digest(bytes), 33554433, 2],
  [digest(bytes), bytes.length, 1],
]) {
  const count = requests.length;
  callbacks = [];
  context.kartyRequestChunk(2, hash, size, version);
  assert.equal(requests.length, count, "invalid descriptors must not fetch any URL");
  assert.equal(callbacks.length, 1);
}

callbacks = [];
responseMode = "delayed";
context.kartyRequestChunk(3, digest(bytes), bytes.length, 2);
await settle();
context.kartyCancelChunk(3);
delayedResolve();
await settle();
assert.equal(callbacks.length, 0, "canceled chunks must not complete even if fetch ignores its signal");

callbacks = [];
responseMode = "delayed";
context.kartyRequestChunk(4, digest(bytes), bytes.length, 2);
await settle();
const previousResolve = delayedResolve;
responseMode = "stream";
context.kartyRequestChunk(4, digest(bytes), bytes.length, 2);
await completed();
previousResolve();
await settle();
assert.equal(callbacks.length, 1, "replacement requests must suppress stale completions");

// Add a start section before code: a chunk can never run a load-time function.
function unsigned(value) {
  const result = [];
  do {
    const current = value & 127;
    value >>>= 7;
    result.push(current | (value ? 128 : 0));
  } while (value);
  return result;
}
function sectionOffset(id) {
  let offset = 8;
  while (offset < fixture.length) {
    const start = offset,
      section = fixture[offset++];
    let size = 0,
      shift = 0,
      current;
    do {
      current = fixture[offset++];
      size += (current & 127) * 2 ** shift;
      shift += 7;
    } while (current & 128);
    if (section === id) return { start, payload: offset, end: offset + size };
    offset += size;
  }
}
const code = sectionOffset(10);
bytes = new Uint8Array([...fixture.subarray(0, code.start), 8, 1, 0, ...fixture.subarray(code.start)]);
const beforePassive = instances;
await request("stream", /not passive/);
const memory = sectionOffset(5);
const memoryPayload = [1, 0, ...unsigned(258)];
bytes = new Uint8Array([
  ...fixture.subarray(0, memory.start),
  5,
  ...unsigned(memoryPayload.length),
  ...memoryPayload,
  ...fixture.subarray(memory.end),
]);
await request("stream", /memory exceeds/);
assert.equal(instances, beforePassive, "unsafe chunks must be rejected before WASM execution");
console.log("Chunk bridge checks passed (actual passive WASM, bounded fetch, cancellation and confined paths).");
