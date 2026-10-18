import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile, writeFile, mkdir, realpath } from "node:fs/promises";
import { createServer } from "node:http";
import { extname, resolve, sep } from "node:path";
import { chromium } from "playwright";

const directory = await realpath(process.env.KARTY_DOCS_SITE ?? "dist/samples-site");
const captures = resolve("dist/validation/docs-web");
await mkdir(captures, { recursive: true });
const mime = {
  ".html": "text/html; charset=utf-8",
  ".md": "text/markdown; charset=utf-8",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".svg": "image/svg+xml",
};
const server = createServer(async (request, response) => {
  try {
    let pathname = new URL(request.url, "http://localhost").pathname;
    if (pathname.endsWith("/")) pathname += "index.html";
    const file = await realpath(resolve(directory, "." + decodeURIComponent(pathname)));
    if (!file.startsWith(directory + sep)) throw new Error("Outside docs root");
    const body = await readFile(file);
    response.writeHead(200, {
      "Content-Type": mime[extname(file)] ?? "application/octet-stream",
      "Content-Length": body.length,
    });
    response.end(body);
  } catch {
    response.writeHead(404).end();
  }
});
await new Promise((done, reject) => {
  server.once("error", reject);
  server.listen(0, "127.0.0.1", done);
});
const base = `http://127.0.0.1:${server.address().port}`;
const report = { pages: [], redirects: [], errors: [], failedResponses: [], artifacts: [] };
const sections = {
  technical: ["index", "runtime", "input", "transforms", "cameras", "graphics", "audio"],
  design: ["index", "why", "architecture", "sdk-and-toolchain", "asset-pipeline", "packages"],
  guide: ["index", "2d", "fps"],
};
let browser;
try {
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  page.on("pageerror", (error) => report.errors.push(error.message));
  page.on("response", (response) => {
    if (response.status() >= 400) report.failedResponses.push({ url: response.url(), status: response.status() });
  });
  await page.route("**/*", (route) =>
    new URL(route.request().url()).origin === base ? route.continue() : route.abort(),
  );
  for (const width of [1440, 390, 320]) {
    await page.setViewportSize({ width, height: width > 800 ? 1000 : 844 });
    for (const [section, pages] of Object.entries(sections)) {
      for (const name of pages) {
        const response = await page.goto(`${base}/${section}/${name}.html`, { waitUntil: "networkidle" });
        assert.equal(response.status(), 200);
        assert.equal(await page.locator("h1").count(), 1);
        const active = await page.locator(".site-nav a[aria-current=page]").innerText();
        assert.equal(active, section === "guide" ? "Guides" : section === "design" ? "Design" : "API");
        assert.deepEqual(
          (await page.locator(".site-nav a").allTextContents()).map((label) => label.trim()),
          ["Samples", "Guides", "API", "Design", "GitHub"],
        );
        const dimensions = await page.evaluate(() => ({
          viewport: innerWidth,
          document: document.documentElement.scrollWidth,
        }));
        assert.ok(dimensions.document <= width, `Overflow on ${section}/${name} at ${width}`);
        if (width <= 560) {
          const navRows = await page
            .locator(".site-nav a")
            .evaluateAll((links) => links.map((link) => Math.round(link.getBoundingClientRect().top)));
          assert.equal(new Set(navRows).size, 1, `Wrapped main navigation at ${width}`);
        }
        for (const image of await page.locator(".guide-body img").all()) {
          await image.scrollIntoViewIfNeeded();
          await image.evaluate((element) => element.decode());
        }
        assert.ok(
          await page
            .locator(".guide-body img")
            .evaluateAll((images) => images.every((image) => image.complete && image.naturalWidth > 0)),
        );
        const download = await page.locator(".page-tools a[download]").getAttribute("href");
        assert.equal((await page.request.get(`${base}/${section}/${download}`)).status(), 200);
        if (section === "technical" && width <= 800) {
          const tablesFit = await page
            .locator(".table-wrap")
            .evaluateAll((tables) => tables.every((table) => table.scrollWidth <= table.clientWidth));
          assert.ok(tablesFit, `Hidden reference descriptions on ${name} at ${width}`);
          await page.locator(".mobile-toc summary").click();
          assert.equal(await page.locator(".mobile-links ul a").count(), 6);
          await page.locator(".mobile-links a[href='graphics.html']").waitFor({ state: "visible" });
          await page.locator(".mobile-toc summary").click();
        }
        await page.screenshot({ path: resolve(captures, `${width}-${section}-${name}.png`), fullPage: true });
        report.pages.push({ width, section, name, dimensions });
      }
    }
  }
  await page.goto(`${base}/guide/why.html`, { waitUntil: "networkidle" });
  await page.waitForURL("**/design/why.html");
  await page.locator(".site-nav a", { hasText: "API" }).click();
  await page.waitForURL("**/technical/");
  await page.locator(".mobile-toc summary").click();
  await page.locator(".mobile-links a[href='cameras.html']").click();
  await page.waitForURL("**/technical/cameras.html");
  await page.locator(".mobile-toc summary").click();
  await page.locator(".mobile-links a[href='#camera-3d']").click();
  await page.waitForURL("**/technical/cameras.html#camera-3d");
  await page.waitForFunction(() => {
    const top = document.getElementById("camera-3d").getBoundingClientRect().top;
    return top >= 0 && top < innerHeight;
  });
  for (const [oldPage, target] of Object.entries({
    "game-lifecycle": "runtime.html#game-lifecycle",
    entities: "runtime.html#entities-and-components",
    levels: "runtime.html#levels-and-resources",
    "transform-2d": "transforms.html#transform-2d",
    "transform-3d": "transforms.html#transform-3d",
    "camera-2d": "cameras.html#camera-2d",
    "camera-3d": "cameras.html#camera-3d",
    sprites: "graphics.html#sprites",
    particles: "graphics.html#particles",
  })) {
    await page.goto(`${base}/technical/${oldPage}.html`, { waitUntil: "networkidle" });
    await page.waitForURL(`${base}/technical/${target}`);
    assert.equal(await page.locator(new URL(page.url()).hash).count(), 1);
    report.redirects.push({ oldPage, target });
  }
  await page.goto(`${base}/`, { waitUntil: "networkidle" });
  assert.deepEqual(
    (await page.locator(".site-nav a").allTextContents()).map((label) => label.trim()),
    ["Samples", "Guides", "API", "Design", "GitHub"],
  );
  await page.locator(".site-nav a", { hasText: "API" }).click();
  await page.waitForURL("**/technical/");
  assert.deepEqual(report.errors, []);
  assert.deepEqual(report.failedResponses, []);
  for (const [section, pages] of Object.entries(sections)) {
    for (const name of pages) {
      const file = `${section}/${name}.html`;
      const sha256 = createHash("sha256")
        .update(await readFile(resolve(directory, file)))
        .digest("hex");
      report.artifacts.push({ file, sha256 });
    }
  }
  console.log(`Passed ${report.pages.length} documentation page/viewport checks; captures: ${captures}`);
} catch (error) {
  report.failure = String(error.stack ?? error);
  throw error;
} finally {
  await writeFile(resolve(captures, "report.json"), JSON.stringify(report, null, 2) + "\n");
  await browser?.close();
  await new Promise((done) => server.close(done));
}
