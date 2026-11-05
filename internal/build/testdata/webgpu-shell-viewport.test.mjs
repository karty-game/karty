import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { chromium } from "playwright";

const html = await readFile(new URL("../templates/index-webgpu.html.tmpl", import.meta.url), "utf8");
const startup = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)][0][1];
const shell = html.replace(/<script\b[^>]*>[\s\S]*?<\/script>/g, "");
const browser = await chromium.launch({
  headless: true,
  args: [
    "--use-gl=angle",
    "--use-angle=gl-egl",
    "--enable-gpu",
    "--ignore-gpu-blocklist",
    "--disable-software-rasterizer",
  ],
  env: { ...process.env, EGL_PLATFORM: "surfaceless", LIBGL_ALWAYS_SOFTWARE: "0" },
});
try {
  const page = await browser.newPage();
  const cdp = await browser.newBrowserCDPSession();
  assert.equal((await cdp.send("SystemInfo.getInfo")).gpu.featureStatus.webgl, "enabled");
  await page.setContent(shell);
  const renderer = await page.evaluate(() => {
    const gl = document.createElement("canvas").getContext("webgl2");
    return gl?.getParameter(gl.getExtension("WEBGL_debug_renderer_info").UNMASKED_RENDERER_WEBGL);
  });
  assert.ok(renderer);
  assert.doesNotMatch(renderer, /swiftshader|llvmpipe|softpipe|software/i);
  await page.evaluate(() => {
    const canvas = document.getElementById("world");
    canvas.width = 1280;
    canvas.height = 720;
  });
  await page.addScriptTag({ content: startup });
  await page.evaluate(() => window.dispatchEvent(new Event("karty-host-ready")));
  for (const viewport of [
    { width: 1907, height: 868 },
    { width: 1920, height: 1080 },
    { width: 900, height: 1200 },
    { width: 640, height: 360 },
  ]) {
    await page.setViewportSize(viewport);
    const state = await page.locator("#world").evaluate((canvas) => {
      const rect = canvas.getBoundingClientRect();
      return {
        x: rect.x,
        y: rect.y,
        width: rect.width,
        height: rect.height,
        internal: [canvas.width, canvas.height],
        outline: getComputedStyle(canvas).outlineStyle,
        imageRendering: getComputedStyle(canvas).imageRendering,
      };
    });
    const scale = Math.min(viewport.width / 1280, viewport.height / 720);
    assert.ok(Math.abs(state.width - 1280 * scale) < 1, "canvas must upscale to largest fitting width");
    assert.ok(Math.abs(state.height - 720 * scale) < 1, "canvas must preserve authored aspect");
    assert.ok(Math.abs(state.x - (viewport.width - state.width) / 2) < 1);
    assert.ok(Math.abs(state.y - (viewport.height - state.height) / 2) < 1);
    assert.deepEqual(state.internal, [1280, 720], "CSS upscale must retain bounded rendering");
    assert.equal(state.outline, "none");
    assert.equal(state.imageRendering, "auto", "fractional scaling must retain smooth edges and text");
  }
  await page.locator("#fullscreen").click();
  await page.waitForFunction(() => document.activeElement?.id === "world");
  assert.ok(await page.evaluate(() => Boolean(document.fullscreenElement)));
  await page.locator("#fullscreen").click();
  await page.waitForFunction(() => !document.fullscreenElement && document.activeElement?.id === "world");
  console.log("WebGPU shell largest-fit viewport and fullscreen focus passed: " + renderer);
} finally {
  await browser.close();
}
