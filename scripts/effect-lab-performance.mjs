// Sample-owned diagnostics, copied into an immutable preview by prepare-effect-lab-preview.mjs.
// GPU timestamps come from the host's asynchronous query pool; never gl.finish/readPixels.
export function effectVariants(defaults) {
  const active = { ...defaults, effectsEnabled: true };
  const color = { saturationAmount: 1.3, contrastAmount: 1.2, gradeExposure: 0.2, gradeSlopeBlue: 0.94 };
  const names = [
    "vignette",
    "scanlines",
    "chromatic",
    "quantize",
    "dither",
    "grain",
    "fog",
    "saturation",
    "contrast",
    "colorGrade",
    "outline",
    "lensDistortion",
    "sharpen",
    "lensSurface",
    "bloom",
  ];
  const variants = [{ name: "clean", options: active }];
  for (const name of names) {
    const options = { ...active, [name]: true, ...color };
    if (name === "dither") options.quantize = true;
    variants.push({ name, options });
  }
  const surface = variants.find((v) => v.name === "lensSurface").options;
  variants.push(
    { name: "lens-dirt", options: { ...surface, lensSurfaceMode: 1 } },
    { name: "lens-blood", options: { ...surface, lensSurfaceMode: 2 } },
  );
  const bloom = variants.find((v) => v.name === "bloom").options;
  variants.push(
    { name: "bloom-compact", options: { ...bloom, bloomQuality: 0 } },
    { name: "bloom-balanced", options: { ...bloom, bloomQuality: 1 } },
    { name: "bloom-rgba8", options: { ...bloom, bloomFormat: 1 } },
  );
  const stack = { ...active, ...color, ...Object.fromEntries(names.map((name) => [name, true])) };
  variants.push({ name: "stack-no-outline", options: { ...stack, outline: false } }, { name: "stack", options: stack });
  return variants;
}

export function summarize(values) {
  if (!values.length) return null;
  const sorted = [...values].sort((a, b) => a - b);
  return {
    samples: values.length,
    meanMs: values.reduce((a, b) => a + b, 0) / values.length,
    p50Ms: sorted[Math.ceil(sorted.length * 0.5) - 1],
    p95Ms: sorted[Math.ceil(sorted.length * 0.95) - 1],
    over16ms: values.filter((v) => v > 1000 / 60).length,
    over33ms: values.filter((v) => v > 1000 / 30).length,
  };
}

// This is the total frame budget. Postprocessing must leave time for the rest
// of the game; a GPU-only result can never establish complete 60 FPS delivery.
export function frameBudget(row) {
  const totalMs = 1000 / 60;
  const gpu = row.gpu?.available && !row.gpu.invalid ? row.gpu.frame : null;
  const cadence = row.browserCadence;
  const over = gpu?.samples > 0 && gpu.p95Ms > totalMs;
  const completeGPU = gpuMean(row) != null;
  return {
    totalMs,
    gpuP95Ms: gpu?.samples > 0 ? gpu.p95Ms : null,
    cadenceP95Ms: cadence?.p95Ms ?? null,
    lateFrames: cadence?.over16ms ?? null,
    observedFrames: cadence?.samples ?? 0,
    status: over ? "gpu-over-budget" : completeGPU ? "gpu-within-budget" : "insufficient-evidence",
  };
}

export function stableIdentity(value) {
  if (Array.isArray(value)) return "[" + value.map(stableIdentity).join(",") + "]";
  if (value && typeof value === "object")
    return (
      "{" +
      Object.keys(value)
        .sort()
        .map((key) => JSON.stringify(key) + ":" + stableIdentity(value[key]))
        .join(",") +
      "}"
    );
  return JSON.stringify(value);
}

function nextFrame(signal, deadline) {
  return new Promise((resolve, reject) => {
    let frame, timer;
    const cleanup = () => {
      clearTimeout(timer);
      cancelAnimationFrame(frame);
      signal.removeEventListener("abort", abort);
    };
    const abort = () => {
      cleanup();
      reject(new Error("Measurement cancelled or invalidated"));
    };
    signal.addEventListener("abort", abort, { once: true });
    timer = setTimeout(
      () => {
        cleanup();
        reject(
          new Error("Measurement exceeded 5 seconds; this is not meeting the 60 FPS target. Reduce render settings."),
        );
      },
      Math.max(1, deadline - performance.now()),
    );
    frame = requestAnimationFrame((stamp) => {
      cleanup();
      resolve(stamp);
    });
    if (signal.aborted) abort();
  });
}

function matched(api) {
  const metadata = api.metadata(),
    stats = api.stats(),
    camera = stats.cameras[0];
  if (stats.error || metadata.cameras.length !== 1 || !camera?.device) throw new Error("Expected one healthy camera");
  if (camera.device.gradePending === true) throw new Error("Color grading is still building during measurement");
  return {
    cameras: metadata.cameras,
    world: [camera.worldWidth, camera.worldHeight],
    presentation: [
      innerWidth,
      innerHeight,
      devicePixelRatio,
      document.querySelector("canvas").width,
      document.querySelector("canvas").height,
    ],
    materials: metadata.materials,
    lights: metadata.lights,
    profile: metadata.qualityProfile,
    options: metadata.options,
    timingScope: metadata.timingScope,
  };
}

async function measure(api, { warmup, frames, signal }) {
  const deadline = performance.now() + 5000;
  const wait = () => nextFrame(signal, deadline);
  const advance = async (count) => {
    const target = api.stats().frames + count;
    while (api.stats().frames < target) await wait();
  };
  await advance(warmup);
  const gradeStart = performance.now(),
    gradeStartFrame = api.stats().frames,
    gradeSettlement = { pendingAfterWarmup: false, waitFrames: 0, waitMs: 0, observedSlices: [] };
  let gradeStats = api.stats(),
    lastGradeFrame;
  gradeSettlement.pendingAfterWarmup = gradeStats.cameras[0]?.device?.gradePending === true;
  for (;;) {
    const device = gradeStats.cameras[0]?.device;
    if (!device || gradeStats.error) throw new Error("Expected one healthy camera while waiting for color grading");
    const frame = device.frames ?? gradeStats.frames;
    if (frame !== lastGradeFrame && (device.gradeBuildNodes > 0 || device.gradeBuildMs > 0)) {
      gradeSettlement.observedSlices.push({
        frame,
        nodes: device.gradeBuildNodes ?? null,
        cpuMs: device.gradeBuildMs ?? null,
      });
      lastGradeFrame = frame;
    }
    if (device.gradePending !== true) break;
    await wait();
    gradeStats = api.stats();
  }
  gradeSettlement.waitFrames = gradeStats.frames - gradeStartFrame;
  gradeSettlement.waitMs = performance.now() - gradeStart;
  const metadata = matched(api),
    identity = stableIdentity(metadata);
  const checkedDevice = () => {
    const device = api.stats().cameras[0].device;
    if (
      metadata.options.effectsEnabled &&
      metadata.options.outline &&
      metadata.options.outlineStrength > 0 &&
      (device.outlineActive === false || device.outlineError)
    )
      throw new Error(
        "Geometry outlines unavailable during measurement: " + (device.outlineError || "requested effect is inactive"),
      );
    if (
      metadata.options.effectsEnabled &&
      metadata.options.bloom &&
      metadata.options.bloomIntensity > 0 &&
      (device.bloomActive === false || device.bloomError)
    )
      throw new Error("Bloom unavailable during measurement: " + (device.bloomError || "requested effect is inactive"));
    return device;
  };
  const before = checkedDevice().resourceRevisions;
  api.startTiming();
  try {
    let stamp = await wait();
    const initial = api.stats().frames,
      target = initial + frames,
      intervals = [];
    while (api.stats().frames < target) {
      const next = await wait();
      intervals.push(next - stamp);
      stamp = next;
      if (stableIdentity(matched(api)) !== identity)
        throw new Error("Camera, resolution, lights or settings changed during measurement");
    }
    const submittedFrames = api.stats().frames - initial;
    api.stopTiming();
    let timing = api.readTiming();
    for (let i = 0; i < 60 && timing.gpu?.available && timing.gpu.pending > 0; i++) {
      await wait();
      timing = api.readTiming();
    }
    if (stableIdentity(matched(api)) !== identity) throw new Error("Measurement changed while draining GPU queries");
    const device = checkedDevice();
    return {
      metadata,
      gradeSettlement,
      submittedFrames,
      cpu: timing.cpuFrame,
      gpu: timing.gpu,
      engineCadence: timing.frameCadence,
      browserCadence: summarize(intervals),
      resourceRevisions: device.resourceRevisions - before,
      renderStats: {
        drawCalls: device.drawCalls,
        opaqueDrawCalls: device.opaqueDrawCalls,
        shadowCaptures: device.shadowCaptures,
        uploadBytes: device.uploadBytes,
        bloomActive: device.bloomActive ?? null,
        bloomBytes: device.bloomBytes ?? null,
        bloomPasses: device.bloomPasses ?? null,
        bloomError: device.bloomError ?? null,
        outlineActive: device.outlineActive ?? null,
        outlineBytes: device.outlineBytes ?? null,
        outlineError: device.outlineError ?? null,
        gradePending: device.gradePending ?? null,
        gradeBuildMs: device.gradeBuildMs ?? null,
        gradeBuildNodes: device.gradeBuildNodes ?? null,
        gradeGPU: device.gradeGPU ?? null,
        gradeBakeError: device.gradeBakeError ?? null,
      },
    };
  } finally {
    api.stopTiming();
  }
}

function gpuMean(row) {
  const gpu = row.gpu;
  return gpu?.available && !gpu.invalid && gpu.frame?.samples >= 16 && !gpu.pending && !gpu.dropped
    ? gpu.frame.meanMs
    : null;
}

export function compare(baseline, row) {
  const before = gpuMean(baseline),
    after = gpuMean(row);
  return {
    gpuDeltaMs: before != null && after != null ? after - before : null,
    gpuPercent: before > 0 && after != null ? (after / before - 1) * 100 : null,
    cadenceDeltaMs:
      row.engineCadence?.meanMs != null && baseline.engineCadence?.meanMs != null
        ? row.engineCadence.meanMs - baseline.engineCadence.meanMs
        : null,
  };
}

// A single clean baseline can give misleading negative effect costs while
// GPU clocks/load settle. Preserve raw timings but withhold those comparisons.
export function qualifyComparisons(rows) {
  const before = gpuMean(rows[0]),
    after = gpuMean(rows.at(-1));
  const drift = before > 0 && after != null ? (after / before - 1) * 100 : null;
  const issue =
    drift == null
      ? "Incomplete clean baseline GPU samples"
      : Math.abs(drift) > 5
        ? `Clean baseline drifted ${drift.toFixed(1)}%; repeat with stable GPU load`
        : null;
  for (const row of rows) {
    row.baselineDriftPercent = drift;
    row.comparisonIssue = issue;
    if (issue && row.comparison) {
      row.comparison.gpuDeltaMs = null;
      row.comparison.gpuPercent = null;
    }
  }
}

let running = false;
export async function runEffectBenchmark({ warmup = 60, frames = 180, repeats = 3, signal, progress = () => {} } = {}) {
  if (running) throw new Error("An effect benchmark is already running");
  for (const [name, value, min, max] of [
    ["warmup", warmup, 1, 600],
    ["frames", frames, 16, 600],
    ["repeats", repeats, 1, 5],
  ]) {
    if (!Number.isInteger(value) || value < min || value > max) throw new Error("Invalid " + name);
  }
  const api = globalThis.kartyWorldRenderer;
  if (!api || document.hidden) throw new Error("Keep the running lab visible");
  running = true;
  const original = api.metadata(),
    controller = new AbortController(),
    rows = [];
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  if (signal?.aborted) abort();
  const hidden = () => {
    if (document.hidden) abort();
  };
  document.addEventListener("visibilitychange", hidden);
  window.addEventListener("resize", abort);
  document.addEventListener("webglcontextlost", abort, true);
  const apply = (options) => {
    const error = api.setOptions(options);
    if (error) throw new Error(error);
  };
  try {
    document.getElementById("karty-effects-preset-Clean").click();
    const defaults = Object.fromEntries(
      [...document.querySelectorAll('input[id^="karty-effect-"],select[id^="karty-effect-"]')].map((e) => {
        const name = e.id.slice("karty-effect-".length);
        return [name, api.metadata().options[name]];
      }),
    );
    // Pin the active presentation profile without changing its render scale.
    // Setting worldScale would switch to Custom and reset presentation scale.
    if (original.qualityProfile.requested === "auto") {
      const profile = document.getElementById("karty-debug-setting-profile");
      profile.value = original.qualityProfile.active;
      profile.dispatchEvent(new Event("change", { bubbles: true }));
    }
    const variants = effectVariants(defaults),
      base = variants[0];
    let matchedScope;
    const effectNames = new Set(Object.keys(defaults));
    const gl = document.querySelector("canvas").getContext("webgl2"),
      debug = gl.getExtension("WEBGL_debug_renderer_info");
    const device = {
      userAgent: navigator.userAgent,
      renderer: gl.getParameter(debug ? debug.UNMASKED_RENDERER_WEBGL : gl.RENDERER),
      vendor: gl.getParameter(debug ? debug.UNMASKED_VENDOR_WEBGL : gl.VENDOR),
    };
    if (/swiftshader|llvmpipe|softpipe|lavapipe|software/i.test(device.renderer))
      throw new Error("Software renderer detected. Enable the device GPU before measuring effect performance.");
    for (let repeat = 0; repeat < repeats; repeat++) {
      // Reverse candidate order every other round to expose ordering/thermal effects.
      const candidates = repeat % 2 ? variants.slice(1).reverse() : variants.slice(1);
      let baseline;
      const roundStart = rows.length;
      for (const variant of [base, ...candidates, base]) {
        progress(`${repeat + 1}/${repeats}: ${variant.name}`);
        apply(variant.options);
        const warmupFrames = rows.length === 0 ? Math.max(warmup, 60) : warmup;
        const row = {
          name: variant.name,
          warmupFrames,
          repeat,
          ...(await measure(api, { warmup: warmupFrames, frames, signal: controller.signal })),
        };
        const candidateScope = {
          ...row.metadata,
          options: Object.fromEntries(Object.entries(row.metadata.options).filter(([name]) => !effectNames.has(name))),
        };
        matchedScope ??= candidateScope;
        if (stableIdentity(candidateScope) !== stableIdentity(matchedScope))
          throw new Error("Comparison scene or resolution changed");
        row.comparison = baseline ? compare(baseline, row) : null;
        row.frameBudget = frameBudget(row);
        rows.push(row);
        baseline ??= row;
      }
      qualifyComparisons(rows.slice(roundStart));
    }
    return {
      version: 1,
      measuredAt: new Date().toISOString(),
      device,
      assets: globalThis.kartyRomanEvidenceAssets ?? null,
      warmup,
      frames,
      repeats,
      rows,
      scope:
        "16.67 ms is the total 60 FPS frame budget, not an effect allowance. Shader/resource warm-up is excluded. Grading must finish before timing starts; its settlement shares the same five-second window deadline. gradeSettlement contains wait duration and observed per-frame build slices at/after warm-up, not a complete build total. renderStats grading fields describe the final frame slice and reset to zero when warm. Actual current browser/device. CPU is camera enqueue; GPU is world rendering plus camera composition; cadence includes frame scheduling. No display latency or physical-phone claim from software renderers. Dither includes quantization; neutral color controls are excluded. GPU deltas require at least 16 valid samples with no pending/dropped/disjoint queries and clean baseline drift no greater than 5%. Raw RAF intervals above 16.67 ms include timestamp quantization; GPU budget status does not certify full frame delivery. Repeated clean rows reveal drift.",
    };
  } finally {
    api.stopTiming();
    try {
      const restoreOptions = { ...original.options };
      // Reapplying an unchanged worldScale would pin Custom and reset presentation scale.
      if (api.metadata().options.worldScale === restoreOptions.worldScale) delete restoreOptions.worldScale;
      apply(restoreOptions);
      for (const input of document.querySelectorAll('input[id^="karty-effect-"],select[id^="karty-effect-"]')) {
        const name = input.id.slice("karty-effect-".length),
          value = original.options[name];
        if (input.type === "checkbox") input.checked = value;
        else input.value = String(value);
        const output = document.getElementById("karty-effect-value-" + name);
        if (output) output.textContent = String(value);
      }
      const profile = document.getElementById("karty-debug-setting-profile");
      if (profile && original.qualityProfile.requested !== "custom") {
        profile.value = original.qualityProfile.requested;
        profile.dispatchEvent(new Event("change", { bubbles: true }));
      }
    } finally {
      signal?.removeEventListener("abort", abort);
      document.removeEventListener("visibilitychange", hidden);
      window.removeEventListener("resize", abort);
      document.removeEventListener("webglcontextlost", abort, true);
      running = false;
    }
  }
}

function attach() {
  const panel = document.getElementById("karty-debug-effects");
  if (!panel || !globalThis.kartyWorldRenderer) return false;
  const section = document.createElement("details");
  section.id = "effect-lab-performance";
  section.style.cssText = "margin:10px 0;padding:8px;border:1px solid #526171;border-radius:8px";
  const title = document.createElement("summary");
  title.style.cssText = "cursor:pointer;font-weight:bold";
  title.textContent = "Measure on this device";
  const help = document.createElement("p");
  help.textContent =
    "Keep this tab visible and the camera still. Measures each effect and combined stacks at the current resolution; clean runs bracket each round. 16.67 ms is the total frame budget at 60 FPS; effects need to leave room for the game. Each window stops after five seconds. Results depend on this device and browser.";
  const mode = document.createElement("select");
  mode.setAttribute("aria-label", "Benchmark duration");
  for (const [value, text] of [
    ["sustained", "Sustained · 3 rounds"],
    ["quick", "Quick · exploratory"],
  ]) {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = text;
    option.style.background = "#0f141b";
    mode.append(option);
  }
  const start = document.createElement("button");
  start.textContent = "Measure all effects";
  const cancel = document.createElement("button");
  cancel.textContent = "Cancel measurement";
  cancel.disabled = true;
  const download = document.createElement("button");
  download.textContent = "Download measurements";
  download.disabled = true;
  const status = document.createElement("p");
  status.setAttribute("role", "status");
  const results = document.createElement("pre");
  results.style.cssText = "white-space:pre;overflow:auto;font-size:11px";
  let controller, report;
  start.onclick = async () => {
    controller = new AbortController();
    start.disabled = mode.disabled = true;
    cancel.disabled = false;
    download.disabled = true;
    results.textContent = "";
    try {
      report = await runEffectBenchmark({
        ...(mode.value === "quick" ? { warmup: 12, frames: 24, repeats: 1 } : {}),
        signal: controller.signal,
        progress: (text) => {
          status.textContent = text;
        },
      });
      results.textContent =
        "Effect              CPU ms  GPU ms  GPU Δ ms  GPU p95  GPU samples  Cad p95  >16.67 ms   GPU budget\n" +
        report.rows
          .map((r) => {
            const number = (v) => (v == null ? "    n/a" : v.toFixed(2).padStart(7));
            return (
              r.name.padEnd(19) +
              number(r.cpu?.meanMs) +
              number(r.gpu?.available && !r.gpu.invalid ? r.gpu.frame?.meanMs : null) +
              number(r.comparison?.gpuDeltaMs) +
              number(r.frameBudget.gpuP95Ms) +
              `${r.gpu?.frame?.samples ?? 0}/${r.cpu?.samples ?? 0}`.padStart(13) +
              number(r.frameBudget.cadenceP95Ms) +
              `${r.frameBudget.lateFrames ?? "?"}/${r.frameBudget.observedFrames}`.padStart(13) +
              "  " +
              r.frameBudget.status
            );
          })
          .join("\n");
      status.textContent =
        "Complete. GPU budget uses the total 16.67 ms target and excludes other game work. RAF interval counts include timestamp rounding near 16.67 ms. Quick runs are exploratory. GPU samples shows query coverage; GPU deltas require 16 valid samples without dropped/pending/disjoint queries. Deltas are withheld above 5% clean baseline drift. Download includes drift, resolution and all timings.";
      download.disabled = false;
    } catch (error) {
      status.textContent = error.message;
    } finally {
      start.disabled = mode.disabled = false;
      cancel.disabled = true;
    }
  };
  cancel.onclick = () => controller?.abort();
  download.onclick = () => {
    const url = URL.createObjectURL(new Blob([JSON.stringify(report, null, 2) + "\n"], { type: "application/json" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = "effect-lab-measurements.json";
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };
  for (const control of [mode, start, cancel, download]) {
    control.style.cssText = document.getElementById("karty-effects-preset-Clean").style.cssText;
    control.style.margin = "4px 2px";
  }
  const style = document.createElement("style");
  style.textContent = "#effect-lab-performance button:disabled{opacity:.45}";
  section.append(style, title, help, mode, start, cancel, download, status, results);
  panel.insertBefore(section, panel.children[2]);
  globalThis.kartyEffectLabBenchmark = runEffectBenchmark;
  return true;
}
if (typeof document !== "undefined") {
  const timer = setInterval(() => {
    if (attach()) clearInterval(timer);
  }, 250);
}
