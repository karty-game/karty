// Runs the staged Go/Ebiten host and TinyGo cartridge in a real browser.
// This is automated runtime evidence, not a manual cross-device visual review.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile, readdir, stat } from "node:fs/promises";
import { extname, normalize, relative, resolve } from "node:path";
import { chromium } from "playwright";

const directory = resolve(process.argv[2] || "samples/pong/dist/web");
await stat(resolve(directory, "index.html"));
const levelArtifacts = await readdir(resolve(directory, "content"));

const prefixes = ["/proxy/4242/", "/missing-script/", "/missing-asset/", "/"];
const mimeTypes = new Map([
  [".html", "text/html; charset=utf-8"],
  [".js", "text/javascript; charset=utf-8"],
  [".json", "application/json"],
  [".png", "image/png"],
  [".wasm", "application/wasm"],
  [".kart", "application/wasm"],
  [".kld", "application/wasm"],
]);
const requests = [];

function respond(response, status, body) {
  response.writeHead(status, { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" });
  response.end(body);
}

const server = createServer(async (request, response) => {
  try {
    const url = new URL(request.url, "http://karty.test");
    const prefix = prefixes.find(candidate => url.pathname.startsWith(candidate));
    if (!prefix) return respond(response, 404, "not found");

    let name = url.pathname.slice(prefix.length) || "index.html";
    requests.push(prefix + name + url.search);
    if (prefix === "/missing-script/" && name === "karty.js") return respond(response, 404, "missing launcher");
    if (prefix === "/missing-asset/" && name === "game.kart") {
      response.writeHead(200, { "Content-Type": "application/wasm", "Cache-Control": "no-store" });
      return response.end(Buffer.from([0, 97, 115, 109, 1, 0, 0, 0]));
    }

    name = normalize(name);
    const path = resolve(directory, name);
    if (relative(directory, path).startsWith("..")) return respond(response, 403, "forbidden");

    const contents = await readFile(path);
    response.writeHead(200, {
      "Content-Type": mimeTypes.get(extname(path)) || "application/octet-stream",
      "Cache-Control": url.searchParams.has("v") ? "public, max-age=31536000, immutable" : "no-cache",
    });
    response.end(contents);
  } catch (error) {
    if (error?.code === "ENOENT") return respond(response, 404, "not found");
    respond(response, 500, String(error));
  }
});

await new Promise((resolveReady, reject) => {
  server.once("error", reject);
  server.listen(0, "127.0.0.1", resolveReady);
});
const address = server.address();
const origin = `http://127.0.0.1:${address.port}`;
const browser = await chromium.launch({ headless: true });

async function checkRuntime(prefix, options = {}) {
  const context = await browser.newContext(options);
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));
  page.on("console", message => { if (message.type() === "error") errors.push(message.text()); });
  await page.goto(origin + prefix, { waitUntil: "domcontentloaded" });
  try {
    await page.waitForFunction(() => document.documentElement.dataset.kartyState === "running", null, { timeout: 30000 });
  } catch (error) {
    const state = await page.locator("html").getAttribute("data-karty-state");
    const detail = await page.locator("#error-detail").textContent();
    throw new Error(`browser runtime did not start (state=${state}, detail=${detail}, pageErrors=${errors.join("; ")}): ${error.message}`);
  }
  const firstFrame = BigInt(await page.locator("html").getAttribute("data-karty-frame"));
  await page.waitForFunction(frame => BigInt(document.documentElement.dataset.kartyFrame) > BigInt(frame), String(firstFrame));
  assert.equal(await page.title(), "pong");
  assert.equal(await page.locator("#loading").isHidden(), true);

  const canvas = page.locator("canvas");
  await canvas.waitFor({ state: "visible" });
  const box = await canvas.boundingBox();
  assert.ok(box && box.width > 0 && box.height > 0, "Ebiten canvas has no visible area");
  assert.ok((await canvas.screenshot()).length > 1024, "Ebiten canvas did not produce a useful screenshot");
  assert.deepEqual(errors, [], `browser page errors: ${errors.join("; ")}`);

  const launcher = prefix + "karty.js?v=";
  const launcherRequests = requests.filter(path => path.startsWith(launcher)).length;
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForFunction(() => document.documentElement.dataset.kartyState === "running", null, { timeout: 30000 });
  assert.equal(await page.locator("#loading").isHidden(), true);
  assert.equal(requests.filter(path => path.startsWith(launcher)).length, launcherRequests, "versioned launcher missed browser cache");
  await context.close();
}

async function checkFailure(prefix, expected) {
  const page = await browser.newPage();
  await page.goto(origin + prefix, { waitUntil: "domcontentloaded" });
  await page.waitForFunction(() => document.documentElement.dataset.kartyState === "error", null, { timeout: 10000 });
  assert.match(await page.locator("#error-detail").textContent(), expected);
  await page.close();
}

try {
  await checkRuntime("/", { viewport: { width: 1280, height: 720 } });
  await checkRuntime("/proxy/4242/", { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
  await checkFailure("/missing-script/", /Could not load script/);
  await checkFailure("/missing-asset/", /asset section is missing or duplicated/);
  assert.ok(requests.some(path => path.startsWith("/proxy/4242/karty.js?v=")), "subpath launcher URL escaped its prefix");
  for (const artifact of levelArtifacts) {
    assert.ok(requests.some(path => path.includes(artifact)), `browser did not fetch level artifact ${artifact}`);
  }
  console.log("Browser checks passed (actual Go/Ebiten host and TinyGo cartridge in Chromium).");
} finally {
  await browser.close();
  await new Promise(resolveClose => server.close(resolveClose));
}
