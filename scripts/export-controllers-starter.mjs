// Canonical source: packages/controllers. Export template-ready, user-owned
// package files without depending on an engine checkout or a published module.
import assert from "node:assert/strict";
import { mkdir, readdir, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const check = process.argv.includes("--check");
const destination = resolve(process.argv.slice(2).find((arg) => arg !== "--check") || "dist/controllers-starter");
const source = resolve("packages/controllers");
const names = (await readdir(source))
  .filter(
    (name) =>
      (name.endsWith(".go") && !name.endsWith("_test.go")) ||
      ["go.mod", "karty-package.toml", "LICENSE.md", "README.md"].includes(name),
  )
  .sort();
const files = new Map();
for (const name of names) {
  const contents = await readFile(resolve(source, name), "utf8");
  files.set(name + ".tmpl", contents.replaceAll("{{", '{{"{{"}}'));
}
for (const name of (await readdir(resolve(source, "docs"))).sort()) {
  if (name.endsWith(".md")) files.set("docs/" + name + ".tmpl", await readFile(resolve(source, "docs", name), "utf8"));
}
if (check) {
  // Compare every distributed source byte, including the public contract/license.
  for (const [name, contents] of files)
    assert.equal(await readFile(resolve(destination, name), "utf8"), contents, name);
  assert.deepEqual(
    (await readdir(destination)).sort(),
    [...files.keys()]
      .filter((name) => !name.includes("/"))
      .concat("docs")
      .sort(),
  );
  assert.deepEqual(
    (await readdir(resolve(destination, "docs"))).sort(),
    [...files.keys()]
      .filter((name) => name.startsWith("docs/"))
      .map((name) => name.slice(5))
      .sort(),
  );
} else {
  for (const [name, contents] of files) {
    const target = resolve(destination, name);
    await mkdir(resolve(target, ".."), { recursive: true });
    await writeFile(target, contents);
  }
}
console.log(`${check ? "Verified" : "Exported"} controllers 0.1.0 starter sources: ${destination}`);
