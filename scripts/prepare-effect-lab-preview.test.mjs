import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, mkdir, readFile, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { prepareEffectLabPreview } from "./prepare-effect-lab-preview.mjs";

const hash = (body) => createHash("sha256").update(body).digest("hex");

test("preview additions and replacement hosts retain verified release assets", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "karty-lab-preview-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  await mkdir(join(root, "karty-releases"));
  const html = "<!doctype html><body>lab</body>";
  await writeFile(join(root, "index.html"), html);
  await writeFile(join(root, "karty-host.wasm"), "replacement host");
  const manifest = JSON.stringify({
    version: 1,
    files: ["index.html", "karty-host.wasm"].map((url) => ({ url, sha256: "old", size: 1, role: "startup" })),
  });
  await writeFile(join(root, "karty-releases/original.json"), manifest);
  await writeFile(
    join(root, "karty-latest.json"),
    JSON.stringify({ version: 1, release: "original", manifest: "karty-releases/original.json" }),
  );
  await prepareEffectLabPreview(root);
  const pointer = JSON.parse(await readFile(join(root, "karty-latest.json"), "utf8"));
  const body = await readFile(join(root, pointer.manifest));
  assert.equal(hash(body), pointer.release);
  const release = JSON.parse(body);
  assert.equal(release.files.length, 3);
  for (const file of release.files) {
    const asset = await readFile(join(root, file.url));
    assert.equal(file.sha256, hash(asset));
    assert.equal(file.size, asset.length);
  }
  assert.equal(await readFile(join(root, "karty-releases/original.json"), "utf8"), manifest);
  await prepareEffectLabPreview(root);
  assert.deepEqual(JSON.parse(await readFile(join(root, "karty-latest.json"), "utf8")), pointer);
  const htmlAfter = await readFile(join(root, "index.html"), "utf8");
  assert.equal(htmlAfter.split('src="./effect-lab-performance.js"').length, 2);

  await writeFile(join(root, "karty-latest.json"), JSON.stringify({ manifest: "../outside.json" }));
  await assert.rejects(prepareEffectLabPreview(root));
  await symlink(join(root, ".."), join(root, "escape"));
  await writeFile(join(root, "karty-latest.json"), JSON.stringify({ manifest: "escape/outside.json" }));
  await assert.rejects(prepareEffectLabPreview(root));
});
