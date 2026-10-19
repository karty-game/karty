// Capture the existing web builds. Rebuild changed samples before running this task.
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { readFile, realpath, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { dirname, extname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const samples = ["hangar", "vigil", "ui-demo", "media-lab", "particle-lab"];
const captures = process.argv.length > 2 ? process.argv.slice(2) : samples;
if (captures.some((name) => !samples.includes(name))) {
  throw new Error(`Choose ${samples.join(", ")}.`);
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
const servedWasm = new Map();

// One free loopback port, owned and closed by this process.
const server = createServer(async (request, response) => {
  try {
    const url = new URL(request.url, "http://localhost");
    const parts = decodeURIComponent(url.pathname).split("/").filter(Boolean);
    const name = parts.shift();
    if (!samples.includes(name) && name !== "runtime") throw new Error("unknown sample");
    const directory = await realpath(
      name === "runtime" ? join(site ?? join(root, "dist", "samples-site"), "runtime") : buildDirectory(name),
    );
    const file = await realpath(resolve(directory, ...parts, parts.length ? "" : "index.html"));
    if (!file.startsWith(directory + sep)) throw new Error("path outside sample");
    const data = await readFile(file);
    const headers = {
      "Content-Type": mime[extname(file)] ?? "application/octet-stream",
      "Cache-Control": "no-store",
    };
    if (extname(file) === ".wasm") {
      const requestPath = url.pathname + url.search;
      const sha256 = hash(data);
      servedWasm.set(requestPath, { requestPath, sha256, bytes: data.length });
      headers["X-Karty-Served-Sha256"] = sha256;
    }
    response.writeHead(200, headers);
    response.end(data);
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
  browser = await chromium.launch({
    headless: true,
    args: ["--no-sandbox", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
  for (const name of captures) {
    const page = await browser.newPage({ viewport: { width: 960, height: 540 } });
    const errors = [];
    const hosts = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (message) => {
      if (message.type() === "error") errors.push(message.text());
    });
    page.on("response", (response) => {
      const url = new URL(response.url());
      if (!response.ok() || !url.pathname.endsWith(".wasm")) return;
      const served = servedWasm.get(url.pathname + url.search);
      // Hash the actual server bytes, then confirm this browser response completed.
      // Chromium can evict large WASM bodies from its inspector cache during startup.
      hosts.push(
        (async () => {
          try {
            const failure = await response.finished();
            if (failure) throw failure;
            if (!served || response.headers()["x-karty-served-sha256"] !== served.sha256) {
              throw new Error("WASM response has no matching locally served checksum");
            }
            return { ...served, status: response.status() };
          } catch (error) {
            return { error: error.message };
          }
        })(),
      );
    });
    const settings = "";
    await page.goto(`http://127.0.0.1:${server.address().port}/${name}/${settings}`);
    await page.waitForFunction(() => document.documentElement.dataset.kartyState === "running", null, {
      timeout: 30000,
    });
    const frames = async (count) => {
      const start = await page.evaluate(() =>
        String(globalThis.kartyFrame ?? document.documentElement.dataset.kartyFrame),
      );
      await page
        .waitForFunction(
          ({ start, count }) =>
            BigInt(globalThis.kartyFrame ?? document.documentElement.dataset.kartyFrame) >=
            BigInt(start) + BigInt(count),
          { start, count },
          { timeout: 30000 },
        )
        .catch(async (error) => {
          const diagnostic = await page.evaluate(() => ({
            state: document.documentElement.dataset.kartyState,
            frame: String(globalThis.kartyFrame ?? document.documentElement.dataset.kartyFrame),
          }));
          throw new Error(`${name}: frames stalled: ${JSON.stringify(diagnostic)}; ${errors.join("; ")}`, {
            cause: error,
          });
        });
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
    const wasmResponses = await Promise.all(hosts);
    if (!wasmResponses.length || wasmResponses.some((response) => response.error)) {
      throw new Error(`${name}: incomplete WASM provenance ${JSON.stringify(wasmResponses)}`);
    }

    const directory = join(root, "samples", name);
    const image = await page.locator("canvas").screenshot({ type: "jpeg", quality: 85 });
    await writeFile(join(directory, "preview.jpg"), image);
    const deleted = new Set(git("ls-files", "--deleted", "--", `samples/${name}`).split("\n"));
    const inputs = git("ls-files", "--cached", "--others", "--exclude-standard", "--", `samples/${name}`)
      .split("\n")
      .filter((path) => path && !deleted.has(path) && !/\/(README\.md|preview\.[^/]+)$/.test(path))
      .sort();
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
      hostSha256: wasmResponses[0].sha256,
      wasmResponses,
      wasmProvenance: "Locally served bytes matched successful, completed browser responses.",
      previewSha256: hash(image),
      width: 960,
      height: 540,
      browser: `Chromium ${browser.version()}`,
      backend: "Chromium SwiftShader WebGL2",
      settings,
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
