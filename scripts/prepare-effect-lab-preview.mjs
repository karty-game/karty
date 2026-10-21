import { createHash } from "node:crypto";
import { copyFile, readFile, realpath, writeFile } from "node:fs/promises";
import { join, resolve, sep } from "node:path";

// Add sample-owned diagnostics to a disposable copy, never generated build sources.
export async function prepareEffectLabPreview(directory) {
  await copyFile(
    new URL("./effect-lab-performance.mjs", import.meta.url),
    join(directory, "effect-lab-performance.js"),
  );
  const path = join(directory, "index.html");
  const html = await readFile(path, "utf8");
  if (!html.includes('src="./effect-lab-performance.js"'))
    await writeFile(
      path,
      html.replace("</body>", '<script type="module" src="./effect-lab-performance.js"></script></body>'),
    );
  await refreshPreviewRelease(directory);
}

// SDK release assets are integrity checked. Intentional changes in this disposable
// preview need a fresh manifest; original release manifests remain immutable.
async function refreshPreviewRelease(directory) {
  const pointerPath = join(directory, "karty-latest.json");
  let pointer;
  try {
    pointer = JSON.parse(await readFile(pointerPath, "utf8"));
  } catch (error) {
    if (error.code === "ENOENT") return; // Older SDK previews have no release pointer.
    throw error;
  }
  const root = await realpath(directory);
  const confined = async (url) => {
    if (typeof url !== "string" || !url || url.startsWith("/") || /[\\?#]/.test(url))
      throw new Error("Invalid preview release asset path");
    const path = await realpath(resolve(root, url));
    if (!path.startsWith(root + sep)) throw new Error("Preview release asset escapes distribution");
    return path;
  };
  const original = await confined(pointer.manifest);
  const manifest = JSON.parse(await readFile(original, "utf8"));
  if (!manifest.files.some((file) => file.url === "effect-lab-performance.js"))
    manifest.files.push({
      url: "effect-lab-performance.js",
      type: "text/javascript; charset=utf-8",
      role: "startup",
    });
  const hash = (body) => createHash("sha256").update(body).digest("hex");
  for (const file of manifest.files) {
    const body = await readFile(await confined(file.url));
    file.size = body.length;
    file.sha256 = hash(body);
  }
  const body = JSON.stringify(manifest);
  pointer.release = hash(body);
  pointer.manifest = "karty-releases/" + pointer.release + ".json";
  const releases = await realpath(join(root, "karty-releases"));
  if (!releases.startsWith(root + sep)) throw new Error("Preview release directory escapes distribution");
  await writeFile(join(releases, pointer.release + ".json"), body);
  await writeFile(pointerPath, JSON.stringify(pointer));
}
