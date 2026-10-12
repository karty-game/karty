// Capture the existing web builds. Rebuild changed samples before running this task.
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { readFile, realpath, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { dirname, extname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const samples = ["ui-demo", "world-camera", "hangar", "media-lab"];
const captures = process.argv.length > 2 ? process.argv.slice(2) : samples;
if (captures.some((name) => !samples.includes(name))) {
  throw new Error("Choose ui-demo, world-camera, hangar or media-lab.");
}
const mime = {
  ".html": "text/html",
  ".js": "text/javascript",
  ".wasm": "application/wasm",
  ".kart": "application/wasm",
};
const hash = (data) => createHash("sha256").update(data).digest("hex");
const git = (...args) => execFileSync("git", args, { cwd: root, encoding: "utf8" }).trim();
const sourceCommit = git("rev-parse", "HEAD");
const site = process.env.KARTY_SAMPLE_SITE ? await realpath(resolve(process.env.KARTY_SAMPLE_SITE)) : null;
const buildDirectory = (name) => (site ? join(site, name) : join(root, "samples", name, "dist", "web"));

// One free loopback port, owned and closed by this process.
const server = createServer(async (request, response) => {
  try {
    const parts = decodeURIComponent(new URL(request.url, "http://localhost").pathname).split("/").filter(Boolean);
    const name = parts.shift();
    if (!samples.includes(name) && name !== "runtime") throw new Error("unknown sample");
    const directory = await realpath(
      name === "runtime" ? join(site ?? join(root, "dist", "samples-site"), "runtime") : buildDirectory(name),
    );
    const file = await realpath(resolve(directory, ...parts, parts.length ? "" : "index.html"));
    if (!file.startsWith(directory + sep)) throw new Error("path outside sample");
    response.writeHead(200, {
      "Content-Type": mime[extname(file)] ?? "application/octet-stream",
      "Cache-Control": "no-store",
    });
    response.end(await readFile(file));
  } catch {
    response.writeHead(404).end();
  }
});
await new Promise((done, reject) => {
  server.once("error", reject);
  server.listen(0, "127.0.0.1", done);
});

let browser;
try {
  browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
  for (const name of captures) {
    const page = await browser.newPage({ viewport: { width: 960, height: 540 } });
    const errors = [];
    const hosts = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("response", (response) => {
      if (new URL(response.url()).pathname.endsWith(".wasm")) hosts.push(response.body().then(hash));
    });
    await page.goto(`http://127.0.0.1:${server.address().port}/${name}/`);
    await page.waitForFunction(() => document.documentElement.dataset.kartyState === "running", null, {
      timeout: 30000,
    });
    const frames = async (count) => {
      const start = await page.evaluate(() =>
        String(globalThis.kartyFrame ?? document.documentElement.dataset.kartyFrame),
      );
      await page.waitForFunction(
        ({ start, count }) =>
          BigInt(globalThis.kartyFrame ?? document.documentElement.dataset.kartyFrame) >= BigInt(start) + BigInt(count),
        { start, count },
        { timeout: 30000 },
      );
    };
    const click = async (x, y) => {
      const box = await page.locator("canvas").boundingBox();
      await page.mouse.click(box.x + (x * box.width) / 960, box.y + (y * box.height) / 540);
      await frames(4);
    };
    await frames(30);
    await page.evaluate(() => globalThis.kartyDebugWidget?.(false));
    await page.addStyleTag({
      content: "#karty-debug-widget, #karty-renderer-debug, #fullscreen { display: none !important; }",
    });

    // Use the composed menu for UI and a representative scene for each other demo.
    if (name === "world-camera") {
      await click(130, 66);
      await frames(30);
      await page.keyboard.press("Escape");
    }
    if (name === "hangar") {
      await page.keyboard.down("V");
      await frames(4);
      await page.keyboard.up("V");
      await frames(4);
      await page.keyboard.press("H");
    }
    if (name === "media-lab") await click(400, 220);
    await frames(30);
    if (errors.length) throw new Error(`${name}: ${errors.join("; ")}`);

    const directory = join(root, "samples", name);
    const image = await page.locator("canvas").screenshot({ type: "jpeg", quality: 85 });
    await writeFile(join(directory, "preview.jpg"), image);
    const inputs = git("ls-files", "--", `samples/${name}`)
      .split("\n")
      .filter((path) => path && !/\/(README\.md|preview\.[^/]+)$/.test(path));
    const sourceHash = createHash("sha256");
    for (const path of inputs) {
      sourceHash
        .update(relative(directory, join(root, path)))
        .update("\0")
        .update(await readFile(join(root, path)))
        .update("\0");
    }
    const report = {
      sample: name,
      capturedAt: new Date().toISOString(),
      sourceCommit,
      sourceSha256: sourceHash.digest("hex"),
      cartridgeSha256: hash(await readFile(join(buildDirectory(name), "game.kart"))),
      hostSha256: (await Promise.all(hosts))[0],
      previewSha256: hash(image),
      width: 960,
      height: 540,
      browser: `Chromium ${browser.version()}`,
      backend: "headless browser",
      errors,
      build: `Existing ${relative(root, buildDirectory(name))} artifacts. Capture does not rebuild the sample.`,
    };
    await writeFile(join(directory, "preview.capture.json"), JSON.stringify(report, null, 2) + "\n");
    console.log(`${name}: ${image.length} bytes, no page errors`);
    await page.close();
  }
} finally {
  await browser?.close();
  await new Promise((done) => server.close(done));
}
