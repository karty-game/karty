// The shared sample shell leaves renderer controls to the host debug screen.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";

const script = await readFile(resolve("internal/build/templates/renderer-debug.js"), "utf8");
const browser = await chromium.launch({ headless: true });
try {
  for (const viewport of [
    { width: 960, height: 540 },
    { width: 320, height: 740 },
  ]) {
    const page = await browser.newPage({ viewport });
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.setContent("<!doctype html><html><head></head><body><canvas></canvas></body></html>");
    await page.addScriptTag({ content: script });
    assert.equal(await page.evaluate(() => kartyRendererDebugEnabled), true);
    assert.equal(await page.locator("button,nav,dialog").count(), 0);
    assert.deepEqual(errors, []);
    await page.close();
  }
  console.log("Sample shell has no Lightmap or AA buttons; host debug opt-in preserved.");
} finally {
  await browser.close();
}
