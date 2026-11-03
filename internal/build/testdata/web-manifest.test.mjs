import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import vm from "node:vm";

const source = await readFile(new URL("../templates/web-service-worker.js.tmpl", import.meta.url), "utf8");
const validation = source.slice(
  source.indexOf("function validateManifest("),
  source.indexOf("async function manifestFor("),
);
const validate = vm.runInNewContext(validation + ";validateManifest", {
  URL,
  scopeURL: new URL("https://game.example/demo/"),
  hexPattern: /^[a-f0-9]{64}$/,
});
const file = { url: "karty-host.wasm", sha256: "ab".repeat(32), size: 8, role: "startup", type: "application/wasm" };
const check = (files) => validate({ version: 1, files }, "cd".repeat(32));
assert.equal(check([file]).files[0].size, 8);
assert.equal(check([{ ...file, url: "../runtime/0.1.0/hash/karty-host.wasm" }]).files.length, 1);
for (const invalid of [
  { size: 0 },
  { size: -1 },
  { size: 134217729 },
  { url: "https://cdn.example/host.wasm" },
  { url: "karty-host.wasm?shared=1" },
  { url: "../outside.kld", role: "background" },
]) {
  assert.throws(() => check([file, { ...file, url: "other.wasm", ...invalid }]));
}
assert.throws(() => check([file, file]), "duplicate final entry must reject the entire manifest");
console.log(
  "PASS: current complete bundles have exact sizes, same-origin sharing and bounded complete manifest admission",
);
