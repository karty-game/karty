import { copyFile, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";

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
}
