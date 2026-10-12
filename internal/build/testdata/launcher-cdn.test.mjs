// CDN startup verification with real Web Crypto and a real minimal WASM module.
import assert from "node:assert/strict";
import { createHash, webcrypto } from "node:crypto";
import { readFile } from "node:fs/promises";
import { runInNewContext } from "node:vm";

const template = await readFile(process.argv[2], "utf8");
const bytes = Uint8Array.from([0, 97, 115, 109, 1, 0, 0, 0]);
const checksum = createHash("sha256").update(bytes).digest("hex");
const url = "../runtime/0.0.11/release.wasm";
const start = template.indexOf("  const hostSource =");
const end = template.indexOf("  const loadClientWasm =");
assert.ok(start > 0 && end > start);
const source = template.slice(start, end).replace("@@HOST_SOURCE@@", JSON.stringify({ url, sha256: checksum }));

for (const mode of ["valid", "mismatch", "http-error", "too-large", "insecure"]) {
  let instantiated = false;
  let cancelled = false;
  const context = {
    Uint8Array,
    status() {},
    crypto: mode === "insecure" ? undefined : webcrypto,
    fetchChecked: async (requested) => {
      assert.equal(requested, url);
      if (mode === "http-error") throw new Error("HTTP 404");
      let read = false;
      return {
        body: {
          getReader() {
            return {
              async read() {
                if (read) return { done: true };
                read = true;
                if (mode === "too-large") return { value: { length: 128 * 1024 * 1024 + 1 }, done: false };
                const value = bytes.slice();
                if (mode === "mismatch") value[value.length - 1] ^= 1;
                return { value, done: false };
              },
              async cancel() {
                cancelled = true;
              },
              releaseLock() {},
            };
          },
        },
      };
    },
    WebAssembly: {
      async instantiate(value, imports) {
        instantiated = true;
        return WebAssembly.instantiate(value, imports);
      },
    },
  };
  runInNewContext(source + "\n globalThis.result = loadHostWasm({});", context);
  if (mode === "valid") {
    const result = await context.result;
    assert.ok(result.instance instanceof WebAssembly.Instance);
    assert.equal(instantiated, true);
  } else {
    await assert.rejects(context.result, /SDK release|HTTP 404|download limit|HTTPS or localhost/);
    assert.equal(instantiated, false, "failed verification must not compile or instantiate the host");
  }
  assert.equal(cancelled, mode === "too-large");
}
console.log("PASS: CDN host pin, HTTP failure, checksum failure, secure context and bounded download");
