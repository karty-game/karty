// Measure the same preview used for visual review on a verified hardware GPU.
import assert from "node:assert/strict";
import { mkdir, writeFile } from "node:fs/promises";
import { launchEffectLabBrowser } from "./effect-lab-browser.mjs";

const target = process.env.KARTY_EFFECT_LAB_URL ?? "http://127.0.0.1:4587/?effects=1";
await mkdir("tmp", { recursive: true });
const browser = await launchEffectLabBrowser();
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
  page.setDefaultTimeout(5000);
  page.setDefaultNavigationTimeout(5000);
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
    else if (message.text().startsWith("Effect measurement:")) console.log(message.text());
  });
  await page.goto(target);
  await page.waitForFunction(
    () =>
      document.documentElement.dataset.kartyState === "running" &&
      typeof globalThis.kartyEffectLabBenchmark === "function",
    null,
    { timeout: 5000 },
  );
  // Explicit medium presentation profile; preserve it across all comparisons.
  await page.locator("#karty-debug-setting-profile").evaluate((input) => {
    input.value = "medium";
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await page.waitForFunction(() => globalThis.kartyWorldRenderer.metadata().qualityProfile.active === "medium");
  const cdp = await browser.newBrowserCDPSession();
  const { gpu } = await cdp.send("SystemInfo.getInfo");
  assert.equal(gpu.featureStatus.webgl, "enabled", "WebGL hardware acceleration is unavailable");
  const report = await page.evaluate(async () => {
    const original = globalThis.kartyWorldRenderer.metadata();
    const report = await globalThis.kartyEffectLabBenchmark({
      warmup: 12,
      frames: 60,
      repeats: 1,
      progress: (text) => console.log("Effect measurement: " + text),
    });
    return {
      ...report,
      optionsBefore: original.options,
      optionsAfter: globalThis.kartyWorldRenderer.metadata().options,
    };
  });
  assert.equal(report.rows.length, 20);
  assert.deepEqual(report.optionsBefore, report.optionsAfter);
  for (const row of report.rows) {
    assert.ok(row.cpu.samples >= 60, "CPU distribution missing frames");
    assert.equal(row.resourceRevisions, 0, "Warmed measurement rebuilt GPU resources");
  }
  assert.deepEqual(errors, []);
  report.scope +=
    " This automated run uses hardware-accelerated Chromium at 1280×720 with the medium presentation profile; it does not establish mobile-device FPS.";
  report.browserGPU = gpu;
  await writeFile("tmp/effect-lab-performance-report.json", JSON.stringify(report, null, 2) + "\n");
  for (const row of report.rows)
    console.log(
      `${row.name}: ${row.frameBudget.status}, GPU p95 ${row.frameBudget.gpuP95Ms?.toFixed(2) ?? "n/a"} ms, cadence p95 ${row.frameBudget.cadenceP95Ms?.toFixed(2) ?? "n/a"} ms`,
    );
  console.log("Effect measurements saved: tmp/effect-lab-performance-report.json");
} finally {
  await browser.close();
}
