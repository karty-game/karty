import assert from "node:assert/strict";
import { test } from "node:test";
import {
  compare,
  effectVariants,
  frameBudget,
  qualifyComparisons,
  runEffectBenchmark,
  stableIdentity,
  summarize,
} from "./effect-lab-performance.mjs";

test("all effects exercise actual work, with matched baseline and outline stacks", () => {
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
  const defaults = {
    ...Object.fromEntries(names.map((n) => [n, false])),
    effectsEnabled: true,
    saturationAmount: 1,
    contrastAmount: 1,
    gradeExposure: 0,
  };
  const variants = effectVariants(defaults);
  assert.equal(variants.length, 23);
  assert.deepEqual(variants[0].options, defaults);
  for (const name of names) assert.equal(variants.find((v) => v.name === name).options[name], true);
  assert.equal(variants.find((v) => v.name === "bloom-compact").options.bloomQuality, 0);
  assert.equal(variants.find((v) => v.name === "bloom-balanced").options.bloomQuality, 1);
  assert.equal(variants.find((v) => v.name === "bloom-rgba8").options.bloomFormat, 1);
  assert.equal(variants.find((v) => v.name === "dither").options.quantize, true);
  assert.notEqual(variants.find((v) => v.name === "saturation").options.saturationAmount, 1);
  assert.notEqual(variants.find((v) => v.name === "contrast").options.contrastAmount, 1);
  assert.notEqual(variants.find((v) => v.name === "colorGrade").options.gradeExposure, 0);
  assert.deepEqual(variants.at(-2).options, { ...variants.at(-1).options, outline: false });
  assert.deepEqual(defaults, variants[0].options, "input defaults mutated");
});

test("GPU deltas require enough valid completed samples", () => {
  const row = (meanMs) => ({
    gpu: { available: true, invalid: 0, dropped: 0, pending: 0, frame: { samples: 32, meanMs } },
    engineCadence: { meanMs },
  });
  assert.deepEqual(compare(row(2), row(3)), { gpuDeltaMs: 1, gpuPercent: 50, cadenceDeltaMs: 1 });
  for (const change of [
    { invalid: 1 },
    { dropped: 1 },
    { pending: 1 },
    { available: false },
    { frame: { samples: 15, meanMs: 3 } },
  ]) {
    const candidate = row(3);
    Object.assign(candidate.gpu, change);
    assert.equal(compare(row(2), candidate).gpuDeltaMs, null);
    assert.equal(compare(candidate, row(2)).gpuPercent, null);
  }
  assert.equal(compare(row(0), row(3)).gpuPercent, null);
});

test("drifting clean baselines cannot produce misleading effect speedups", () => {
  const row = (meanMs) => ({
    gpu: { available: true, pending: 0, dropped: 0, invalid: 0, frame: { samples: 32, meanMs } },
    comparison: { gpuDeltaMs: -1, gpuPercent: -20 },
  });
  const rows = [row(5), row(4.5), row(4)];
  qualifyComparisons(rows);
  for (const result of rows) {
    assert.equal(result.comparison.gpuDeltaMs, null);
    assert.equal(result.comparison.gpuPercent, null);
    assert.match(result.comparisonIssue, /baseline drifted/);
    assert.ok(Math.abs(result.baselineDriftPercent + 20) < 1e-10);
  }
  const stable = [row(5), row(6), row(5.1)];
  qualifyComparisons(stable);
  assert.equal(stable[1].comparison.gpuDeltaMs, -1);
  assert.equal(stable[1].comparisonIssue, null);
});

test("cadence reports tail latency and both mobile frame budgets", () => {
  assert.deepEqual(summarize([10, 20, 40, 10]), {
    samples: 4,
    meanMs: 20,
    p50Ms: 10,
    p95Ms: 40,
    over16ms: 2,
    over33ms: 1,
  });
  assert.equal(summarize([]), null);
});

test("60 FPS budget failures remain visible with incomplete GPU samples", () => {
  const row = {
    gpu: { available: true, invalid: 0, pending: 0, dropped: 0, frame: { samples: 32, meanMs: 2, p95Ms: 3 } },
    browserCadence: summarize(Array(20).fill(16)),
  };
  assert.equal(frameBudget(row).status, "gpu-within-budget");
  row.gpu.frame.samples = 4;
  assert.equal(frameBudget(row).status, "insufficient-evidence");
  row.gpu.frame.p95Ms = 18;
  assert.equal(frameBudget(row).status, "gpu-over-budget");
  row.gpu.available = false;
  row.browserCadence = summarize([16, 16, 35, 16]);
  const budget = frameBudget(row);
  assert.equal(budget.status, "insufficient-evidence");
  assert.equal(budget.lateFrames, 1);
  assert.equal(budget.totalMs, 1000 / 60);
  assert.equal(budget.gpuP95Ms, null);
});

test("Go map property order cannot invalidate otherwise matched captures", () => {
  assert.equal(
    stableIdentity({ options: { blue: 1, red: 2 }, camera: [1, 2] }),
    stableIdentity({ camera: [1, 2], options: { red: 2, blue: 1 } }),
  );
  assert.notEqual(stableIdentity({ camera: [1, 2] }), stableIdentity({ camera: [2, 1] }));
});

test("complete and cancelled captures restore effects and the original presentation profile", async (t) => {
  const saved = new Map();
  const expose = (name, value) => {
    saved.set(name, Object.getOwnPropertyDescriptor(globalThis, name));
    Object.defineProperty(globalThis, name, { configurable: true, writable: true, value });
  };
  const defaults = {
    ...Object.fromEntries(
      [
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
      ].map((n) => [n, false]),
    ),
    effectsEnabled: true,
    worldScale: 1,
    aa: true,
    saturationAmount: 1,
    contrastAmount: 1,
    gradeExposure: 0,
    gradeSlopeBlue: 1,
    outlineStrength: 0.8,
    lensSurfaceMode: 0,
    lensAmount: 0.18,
    sharpness: 0.35,
    lensSurfaceStrength: 0.65,
    lensCoverage: 0.55,
    lensSize: 1,
    lensRefraction: 8,
    lensSpecular: 0.35,
    bloomIntensity: 1,
    bloomQuality: 2,
    bloomFormat: 0,
  };
  let options = { ...defaults, grain: true },
    profile = "low",
    frame = 0,
    timingStart = 0,
    renderer = "fixture GPU",
    outlineDiagnosticOverride = null,
    gradeDelayFrames = 0,
    gradeRemaining = 0,
    gradeBuildMs = 0,
    gradeBuildNodes = 0,
    timingStarts = 0,
    onGradeFrame = null;
  const original = { ...options };
  const controls = Object.keys(defaults)
    .filter((n) => n !== "worldScale" && n !== "aa")
    .map((n) => ({ id: "karty-effect-" + n, type: "checkbox" }));
  const selector = {
    value: "low",
    dispatchEvent() {
      profile = this.value;
    },
  };
  const canvas = {
    width: 960,
    height: 540,
    getContext: () => ({ getExtension: () => null, getParameter: () => renderer }),
  };
  const document = Object.assign(new EventTarget(), {
    hidden: false,
    querySelector: () => canvas,
    querySelectorAll: () => controls,
    getElementById: (id) =>
      id === "karty-effects-preset-Clean"
        ? {
            click: () => {
              options = { ...defaults };
              gradeRemaining = 0;
            },
          }
        : id === "karty-debug-setting-profile"
          ? selector
          : null,
  });
  const api = {
    metadata: () => ({
      options: { ...options },
      qualityProfile: { requested: profile, active: profile === "auto" ? "low" : profile, renderScale: 0.75 },
      cameras: [{ position: [0, 0, 0] }],
      materials: [1],
      lights: [1],
      timingScope: { cpu: "fixture CPU", gpu: "fixture GPU" },
    }),
    stats: () => {
      const outlineRequested = options.effectsEnabled && options.outline && options.outlineStrength > 0;
      return {
        frames: frame,
        error: null,
        cameras: [
          {
            worldWidth: 960,
            worldHeight: 540,
            device: {
              frames: frame,
              resourceRevisions: 2,
              bloomActive: options.bloom,
              bloomBytes: options.bloom ? 2048 : 0,
              bloomPasses: options.bloom ? 2 * (options.bloomQuality + 1) : 0,
              bloomError: "",
              gradePending: gradeRemaining > 0,
              gradeBuildMs,
              gradeBuildNodes,
              gradeGPU: gradeDelayFrames === 0,
              gradeBakeError: gradeDelayFrames > 0 ? "fixture GPU bake unavailable" : "",
              ...(outlineRequested && outlineDiagnosticOverride !== null
                ? outlineDiagnosticOverride
                : { outlineActive: outlineRequested, outlineBytes: outlineRequested ? 4096 : 0, outlineError: "" }),
            },
          },
        ],
      };
    },
    setOptions: (values) => {
      Object.assign(options, values);
      gradeRemaining =
        options.effectsEnabled && (options.saturation || options.contrast || options.colorGrade) ? gradeDelayFrames : 0;
      if ("worldScale" in values) profile = "custom";
      return null;
    },
    startTiming: () => {
      assert.equal(gradeRemaining, 0, "timing started while the previous grading LUT was still displayed");
      timingStarts++;
      timingStart = frame;
    },
    stopTiming: () => {},
    readTiming: () => ({
      cpuFrame: { samples: frame - timingStart, meanMs: 1 },
      frameCadence: { meanMs: 16 },
      gpu: {
        available: true,
        pending: 0,
        dropped: 0,
        invalid: 0,
        frame: { samples: 32, meanMs: options.outline ? 3 : 2 },
      },
    }),
  };
  for (const [key, value] of Object.entries({
    document,
    window: new EventTarget(),
    innerWidth: 960,
    innerHeight: 540,
    devicePixelRatio: 1,
    navigator: { userAgent: "fixture" },
    kartyWorldRenderer: api,
    requestAnimationFrame: (cb) =>
      setImmediate(() => {
        frame++;
        gradeBuildMs = gradeBuildNodes = 0;
        if (gradeRemaining > 0) {
          gradeRemaining--;
          gradeBuildMs = 0.2;
          gradeBuildNodes = 64;
          onGradeFrame?.();
        }
        cb(performance.now());
      }),
    cancelAnimationFrame: clearImmediate,
  }))
    expose(key, value);
  try {
    const report = await runEffectBenchmark({ warmup: 1, frames: 16, repeats: 1 });
    assert.equal(report.rows.length, 24);
    assert.equal(report.rows.find((r) => r.name === "outline").comparison.gpuDeltaMs, 1);
    const outlineStats = report.rows.find((r) => r.name === "outline").renderStats;
    assert.equal(outlineStats.outlineActive, true);
    assert.ok(outlineStats.outlineBytes > 0);
    assert.equal(outlineStats.outlineError, "");
    assert.equal(report.rows[0].renderStats.outlineActive, false);
    assert.equal(report.rows[0].renderStats.gradeGPU, true);
    assert.equal(report.rows[0].renderStats.gradeBakeError, "");
    assert.ok(report.rows.every((r) => r.resourceRevisions === 0));
    assert.deepEqual(options, original);
    assert.equal(profile, "low");
    const controller = new AbortController();
    await assert.rejects(
      runEffectBenchmark({
        warmup: 1,
        frames: 16,
        repeats: 1,
        signal: controller.signal,
        progress: () => controller.abort(),
      }),
      /cancelled/,
    );
    assert.deepEqual(options, original);
    assert.equal(profile, "low");
    profile = "auto";
    await runEffectBenchmark({ warmup: 1, frames: 16, repeats: 1 });
    assert.equal(profile, "auto");
    assert.deepEqual(options, original);
    await assert.rejects(
      runEffectBenchmark({
        warmup: 1,
        frames: 16,
        repeats: 1,
        progress: (label) => {
          if (label.endsWith("vignette")) api.setOptions({ aa: false });
        },
      }),
      /Comparison scene or resolution changed/,
    );
    assert.deepEqual(options, original);
    assert.equal(profile, "auto");
    await t.test("incremental grading finishes before timing and reports excluded build slices", async () => {
      gradeDelayFrames = 6;
      try {
        const delayed = await runEffectBenchmark({ warmup: 1, frames: 16, repeats: 1 });
        const gradedRows = delayed.rows.filter(
          (row) => row.metadata.options.saturation || row.metadata.options.contrast || row.metadata.options.colorGrade,
        );
        assert.ok(gradedRows.length >= 3);
        for (const row of gradedRows) {
          assert.equal(row.gradeSettlement.pendingAfterWarmup, true);
          assert.equal(row.gradeSettlement.waitFrames, 5);
          assert.equal(row.gradeSettlement.observedSlices.length, 6);
          assert.equal(
            row.gradeSettlement.observedSlices.reduce((sum, slice) => sum + slice.nodes, 0),
            384,
          );
          assert.ok(row.gradeSettlement.observedSlices.every((slice) => slice.cpuMs === 0.2));
          assert.equal(row.renderStats.gradePending, false);
          assert.equal(row.renderStats.gradeBuildNodes, 0);
          assert.equal(row.renderStats.gradeBuildMs, 0);
          assert.equal(row.renderStats.gradeGPU, false);
          assert.equal(row.renderStats.gradeBakeError, "fixture GPU bake unavailable");
          assert.equal(row.resourceRevisions, 0);
        }
        assert.deepEqual(options, original);
        assert.equal(profile, "auto");
      } finally {
        gradeDelayFrames = 0;
      }
    });
    await t.test("cancellation while waiting for grading never starts an old-LUT timing window", async () => {
      gradeDelayFrames = 8;
      const cancellation = new AbortController();
      let startsBeforeGrading,
        cancelledWhileSettling = false;
      onGradeFrame = () => {
        if (gradeRemaining === 6) {
          cancelledWhileSettling = true;
          cancellation.abort();
        }
      };
      try {
        await assert.rejects(
          runEffectBenchmark({
            warmup: 1,
            frames: 16,
            repeats: 1,
            signal: cancellation.signal,
            progress: (label) => {
              if (label.endsWith("saturation")) startsBeforeGrading = timingStarts;
            },
          }),
          /cancelled/,
        );
        assert.equal(cancelledWhileSettling, true);
        assert.equal(timingStarts, startsBeforeGrading);
        assert.deepEqual(options, original);
        assert.equal(profile, "auto");
      } finally {
        onGradeFrame = null;
        gradeDelayFrames = 0;
      }
    });
    await t.test("unavailable outlines reject measurements and restore options and Auto profile", async () => {
      for (const diagnostic of [
        { outlineActive: false, outlineBytes: 0, outlineError: "outline mask GL 0x505" },
        { outlineActive: false, outlineBytes: 0, outlineError: "" },
        { outlineActive: true, outlineBytes: 4096, outlineError: "outline mask setup failed" },
      ]) {
        outlineDiagnosticOverride = diagnostic;
        await assert.rejects(runEffectBenchmark({ warmup: 1, frames: 16, repeats: 1 }), (error) => {
          assert.match(error.message, /Geometry outlines unavailable during measurement/);
          assert.ok(error.message.includes(diagnostic.outlineError || "requested effect is inactive"));
          return true;
        });
        assert.deepEqual(options, original);
        assert.equal(profile, "auto");
      }
      outlineDiagnosticOverride = {};
      const legacy = await runEffectBenchmark({ warmup: 1, frames: 16, repeats: 1 });
      assert.equal(legacy.rows.find((r) => r.name === "outline").renderStats.outlineActive, null);
      assert.deepEqual(options, original);
      assert.equal(profile, "auto");
      outlineDiagnosticOverride = null;
    });
    renderer = "ANGLE (SwiftShader Device)";
    await assert.rejects(runEffectBenchmark({ warmup: 1, frames: 16, repeats: 1 }), /Software renderer detected/);
    assert.deepEqual(options, original);
    assert.equal(profile, "auto");
    document.hidden = true;
    await assert.rejects(runEffectBenchmark({ warmup: 1, frames: 16, repeats: 1 }), /visible/);
  } finally {
    for (const [name, descriptor] of saved) {
      if (descriptor) Object.defineProperty(globalThis, name, descriptor);
      else delete globalThis[name];
    }
  }
});
