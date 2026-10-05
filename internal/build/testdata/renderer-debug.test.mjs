// Browser controls use the real staged script with a small diagnostic host stub.
// Real GPU atlas capture is validated by the engine's renderer tests.
import assert from 'node:assert/strict';
import { readFile, mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { chromium } from 'playwright';

const script = await readFile(resolve('internal/build/templates/renderer-debug.js'), 'utf8');
const artifacts = resolve('dist/validation/renderer-debug');
await mkdir(artifacts, { recursive: true });
const browser = await chromium.launch({ headless: true });
try {
  for (const viewport of [{ width: 960, height: 540 }, { width: 320, height: 740 }]) {
    const page = await browser.newPage({ viewport });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.setContent('<!doctype html><html data-karty-state="running"><head></head><body><canvas></canvas></body></html>');
    await page.evaluate(() => {
      globalThis.mode = 'smaa';
      globalThis.guestInputs = 0;
      document.addEventListener('keyup', () => guestInputs++);
      document.addEventListener('pointerup', () => guestInputs++);
      globalThis.kartySMAA = next => { if (next) mode = next; return { mode }; };
      globalThis.snapshots = 0;
      globalThis.kartyLightmapDebug = async () => {
        snapshots++;
        return { status: 'ready', width: 3072, height: 1024, tiles: 3, charts: 100, lights: 6, indirect: true,
          encoding: 'direct-rnm3@1', image: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j9mUAAAAASUVORK5CYII=' };
      };
    });
    await page.addScriptTag({ content: script });
    const toggle = page.locator('#karty-aa-toggle');
    await toggle.click();
    assert.equal(await toggle.textContent(), 'AA: Off');
    assert.equal(await page.evaluate(() => mode), 'off');
    await toggle.focus();
    await page.keyboard.press('Enter');
    assert.equal(await toggle.textContent(), 'AA: On');
    assert.equal(await toggle.getAttribute('aria-pressed'), 'true');
    assert.equal(await page.evaluate(() => guestInputs), 0);
    assert.equal(await page.evaluate(() => snapshots), 0, 'ordinary controls must not read the atlas');
    await page.locator('#karty-lightmap-open').click();
    await page.locator('#karty-lightmap-image').waitFor({ state: 'visible' });
    assert.match(await page.locator('#karty-lightmap-status').textContent(), /3 tiles.*6 baked lights.*bounced/);
    await page.locator('#karty-lightmap-zoom').click();
    assert.equal(await page.locator('#karty-lightmap-zoom').textContent(), 'Fit atlas');
    await page.locator('#karty-lightmap-refresh').click();
    await page.waitForFunction(() => snapshots === 2);
    await page.screenshot({ path: resolve(artifacts, `lightmap-${viewport.width}.png`) });
    await page.keyboard.press('Escape');
    assert.equal(await page.locator('#karty-lightmap-image').getAttribute('src'), null);
    assert.equal(await page.evaluate(() => guestInputs), 0);
    await page.screenshot({ path: resolve(artifacts, `controls-${viewport.width}.png`) });
    for (const id of ['karty-lightmap-open', 'karty-aa-toggle']) {
      const box = await page.locator(`#${id}`).boundingBox();
      assert.ok(box && box.height >= 44 && box.x >= 0 && box.x + box.width <= viewport.width);
    }
    assert.deepEqual(errors, []);
    await page.close();
  }
  console.log('Renderer debug controls passed at desktop and 320-pixel mobile widths.');
} finally {
  await browser.close();
}
