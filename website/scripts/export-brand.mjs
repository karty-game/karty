import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { chromium } from "playwright";
const artwork = await readFile("internal/brand/favicon.svg", "utf8");
const icon = "data:image/svg+xml;base64," + Buffer.from(artwork).toString("base64");
const font = await readFile("website/assets/fonts/baloo2/Baloo2.ttf");
const regular = await readFile(process.env.KARTY_BRAND_BODY_FONT ?? "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf");
const banner = `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="320" viewBox="0 0 640 320">
<defs>
 <radialGradient id="glow"><stop stop-color="#304a70" stop-opacity=".55"/><stop offset="1" stop-color="#111827" stop-opacity="0"/></radialGradient>
 <style>@font-face{font-family:KartyDisplay;src:url(data:font/ttf;base64,${font.toString("base64")});font-weight:400 800} @font-face{font-family:KartyBody;src:url(data:font/ttf;base64,${regular.toString("base64")})}.wordmark{font-family:KartyDisplay,sans-serif;font-size:108px;font-weight:800;letter-spacing:-2px}.slogan{font-family:KartyBody,sans-serif;font-size:17px;letter-spacing:-.2px}</style>
</defs>
<rect width="640" height="320" fill="#111827"/>
<ellipse cx="470" cy="90" rx="360" ry="300" fill="url(#glow)"/>
<path d="M-28 254C46 292 150 274 198 222S244 99 213 45" fill="none" stroke="#30405a" stroke-opacity=".48"/>
<path d="M-36 273C60 320 174 293 220 232S267 88 234 29" fill="none" stroke="#30405a" stroke-opacity=".3"/>
<image href="${icon}" x="47" y="62" width="202" height="202"/>
<text class="wordmark" x="272" y="162" fill="#bc612d">Karty</text>
<text class="wordmark" x="270" y="158" fill="#fff4e7">Karty</text>
<rect x="274" y="181" width="40" height="4" rx="2" fill="#ffad48"/>
<text class="slogan" x="274" y="217" fill="#cfd8e8">Retro looks, modern tools.</text>
<path d="M582 250v12m-6-6h12" fill="none" stroke="#ffad48" stroke-width="2" stroke-linecap="round" opacity=".65"/>
</svg>`;
const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 256, height: 256 }, deviceScaleFactor: 1 });
  await page.setContent(
    `<style>html,body{margin:0;background:transparent}svg{display:block;width:256px;height:256px}</style>${artwork}`,
  );
  await page.screenshot({ path: "website/assets/logo.png", omitBackground: true });
  await page.setViewportSize({ width: 640, height: 320 });
  await page.setContent(`<style>html,body{margin:0;background:#111827}svg{display:block}</style>${banner}`);
  await page.evaluate(() => document.fonts.ready);
  assert.ok(await page.evaluate(() => document.fonts.check("800 108px KartyDisplay")), "Wordmark font did not load");
  await page.locator("image").evaluate(async (element) => {
    const img = new Image();
    img.src = element.getAttribute("href");
    await img.decode();
  });
  const wordmark = await page
    .locator("text.wordmark")
    .last()
    .evaluate((el) => {
      const r = el.getBBox();
      return { x: r.x, y: r.y, width: r.width, height: r.height };
    });
  const slogan = await page.locator("text.slogan").evaluate((el) => {
    const r = el.getBBox();
    return { x: r.x, y: r.y, width: r.width, height: r.height };
  });
  assert.ok(wordmark.x + wordmark.width < 620 && slogan.x + slogan.width < 620, "Text does not fit the banner");
  await page.screenshot({ path: "website/assets/banner.png" });
  for (const [name, w, h] of [
    ["logo.png", 256, 256],
    ["banner.png", 640, 320],
  ]) {
    const bytes = await readFile(`website/assets/${name}`);
    assert.equal(bytes.readUInt32BE(16), w);
    assert.equal(bytes.readUInt32BE(20), h);
  }
  const alpha = await page.evaluate(
    async (src) => {
      const img = new Image();
      img.src = src;
      await img.decode();
      const canvas = document.createElement("canvas");
      canvas.width = canvas.height = 256;
      const ctx = canvas.getContext("2d");
      ctx.drawImage(img, 0, 0);
      return ctx.getImageData(0, 0, 1, 1).data[3];
    },
    "data:image/png;base64," + (await readFile("website/assets/logo.png")).toString("base64"),
  );
  assert.equal(alpha, 0, "Logo background is not transparent");
  console.log(JSON.stringify({ logo: "256x256, transparent PNG", banner: "640x320, opaque PNG", wordmark, slogan }));
} finally {
  await browser.close();
}
