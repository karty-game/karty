// Run after building samples/media-lab for web with its pinned SDK and host.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import { dirname, join, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "../samples/media-lab/dist/web");
const requests = [];
const server = createServer(async (request, response) => {
  const pathname = decodeURIComponent(new URL(request.url, "http://localhost").pathname);
  const file = join(root, pathname === "/" ? "index.html" : pathname);
  if (!file.startsWith(root + sep)) {
    response.writeHead(403).end();
    return;
  }
  try {
    const data = await readFile(file);
    requests.push(pathname);
    response.setHeader(
      "Content-Type",
      file.endsWith(".wasm")
        ? "application/wasm"
        : file.endsWith(".js")
          ? "text/javascript"
          : file.endsWith(".html")
            ? "text/html"
            : "application/octet-stream",
    );
    response.end(data);
  } catch {
    response.writeHead(404).end();
  }
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
let browser;
try {
  browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
  const page = await browser.newPage({ viewport: { width: 1000, height: 650 } });
  const errors = [];
  page.on("pageerror", (error) => errors.push(String(error)));
  page.on("console", (message) => {
    if (message.type() === "error" || message.text().includes("video playback failed")) errors.push(message.text());
  });
  await page.goto(`http://127.0.0.1:${server.address().port}/`);
  await page.waitForSelector("canvas", { timeout: 9000 });
  await page.waitForTimeout(2000);
  const box = await page.locator("canvas").boundingBox();
  const scale = Math.min(box.width / 960, box.height / 540);
  const x = box.x + (box.width - 960 * scale) / 2;
  const y = box.y + (box.height - 540 * scale) / 2;
  const clip = { x: x + 330 * scale, y: y + 150 * scale, width: 280 * scale, height: 130 * scale };
  const snapshot = async () =>
    createHash("sha256")
      .update(await page.screenshot({ clip }))
      .digest("hex");
  const click = (px, py) => page.mouse.click(x + px * scale, y + py * scale);
  const before = await snapshot();
  await click(400, 220);
  await page.waitForTimeout(800);
  const first = await snapshot();
  await page.waitForTimeout(800);
  assert.notEqual(await snapshot(), first, "video frames must change");
  assert.notEqual(first, before, "video must render over the scene");
  await click(550, 220);
  await page.waitForTimeout(300);
  assert.equal(await snapshot(), before, "stop must restore the scene");
  await click(400, 220);
  await page.waitForTimeout(4500);
  const ended = await snapshot();
  assert.notEqual(ended, before, "replay must leave a video frame visible");
  await page.waitForTimeout(300);
  assert.equal(await snapshot(), ended, "EOF must retain the final frame");
  await click(550, 220);
  await page.waitForTimeout(300);
  assert.equal(await snapshot(), before, "stop after EOF must restore the scene");
  assert.equal(requests.filter((value) => value.endsWith(".kvid")).length, 2, "replay must open a fresh stream");
  assert.deepEqual(errors, []);
  console.log("PASS: actual WASM, HTTP video, changing frames, stop, replay, EOF; no browser errors");
} finally {
  await browser?.close();
  await new Promise((resolve) => server.close(resolve));
}
