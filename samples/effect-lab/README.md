# Retro effect lab

A small FPS room for comparing rendering treatments in motion. Five warm, cool,
neutral and colored lights illuminate painted blocks, wooden and brick objects,
metal columns, grayscale/color ramps and fine checker patterns. White depth
markers provide a distance-fog comparison.

All artwork is procedural. From the CLI root, regenerate it with
`mise run generate-effect-lab`. Materialize builds the normal, height,
AO and reflectance maps through the ordinary SDK toolchain.

## Play

Build the CLI from its repository root, then run `../../dist/karty dev` here.
Use WASD to walk, Shift to run, mouse or arrows to look and Escape to release
the cursor. Touch drags in the lower left/right halves move/look. Press 1 for
the overview, 2 for the charts, 3 for the material specimens, and R to restore
the current viewpoint. Movement uses host-constrained camera proposals.

The room and Effects panel run with its pinned public SDK and matching public
host. Build normally to use that host. For development, select a matching host
using the CLI's `--host` or `KARTY_HOST_WEB` override; its `wasm_exec.js` must be
beside the host. Use the exact Go compiler required by the selected SDK.

## Mobile target

The user’s established test phone is a **Google Pixel 6**. Retain this target
when investigating mobile issues; ask for OS/browser versions only when needed.
Desktop AMD GPU checks do not establish Pixel 6 performance or stability.

## Compare effects

Open **Debug → Effects**, or append `?effects=1` to the preview URL. Release
mouse look before using the panel. Individual toggles retain their strengths;
Enable effects provides a master before/after comparison. Clean resets all
settings. CRT, Handheld, Hazy, Warm, Cool and Ink provide starting points for experimentation.
Use Jump to effect to reach a section directly.

- Warp the lens with bounded barrel/pincushion distortion, optical center and
  crop zoom. Automatic crop compensation keeps the frame inside the source.
  Chromatic aberration shares this optical center, including while lens
  distortion is disabled; vignette position is independent.
- Restore detail with adaptive RCAS sharpening. Compare native resolution and
  the lower-resolution render profiles; it cannot recover lost detail.
- Add a lens surface: refracting water beads, nonuniform grime or translucent
  blood deposition. Adjust strength, coverage, size and water refraction. These
  are static lens treatments; they do not simulate flowing or coalescing liquid.
  Water specular controls bright glints and rims independently: 0 disables them,
  1 retains their previous brightness, and the restrained default is 0.35.
  Refraction, dark rims and surface coverage remain visible at zero specular.
  Dirt and blood use one nonrepeating, aspect-preserving cover plate with
  reauthored deposits, using the existing atlas memory and one field sample.
  Their size setting 0.5…2 means plate zoom 1…4. Water keeps its original bead
  field and spacing interpretation.
- Compare Oklab chroma and linear-luminance saturation. Neutral saturation,
  contrast and grading controls bypass exactly.
- Adjust contrast with an 18% linear-gray pivot and preserved black/white endpoints.
- Grade exposure and RGB slope/offset/power with a mix control. Warm and Cool
  provide restrained looks; this is linear SDR grading, not film-stock emulation.
  **Preparing color treatment…** indicates a replacement lookup is building.
  Color edits normally bake the full-quality 33³ lookup on the GPU when settings
  change. Preparing applies to the bounded CPU fallback: the previous complete
  treatment remains visible until its replacement is published; first use stays
  neutral until ready. Further edits replace unfinished work, and neutral
  settings bypass immediately.
- Add geometry outlines with depth, crease or combined modes, source-pixel
  radius, ink RGB, opacity and softness. Normal maps and lighting do not produce
  edges. Blended glass/water/particles are excluded from the geometry pass; opaque
  geometry behind them can remain outlined.
- Compare uniform RGB, RGB565, EGA16 and a four-level green luminance palette.
- Compare Bayer 2×2/4×4/8×8 with static blue noise on the same ramp. Dither only
  operates when color quantization is enabled. EGA uses a precomputed search over all color pairs and discrete coverage
  values; its color lookup remains an approximation.
  EGA dither strength blends coverage from the nearest palette color toward the
  selected pair, with exact nearest/full-pair endpoints.
- Compare artistic and lens-inspired vignette falloff; adjust feather,
  roundness, center and strength.
- Adjust correlated grain's size, luminance response, color and cadence. Zero
  cadence freezes its pattern. This is a realtime approximation of film grain.
  Colored grain balances channel variance while retaining the fine/coarse mix.
- Adjust scanlines, radial chromatic separation and distance fog independently.
  The minimum scanline size now alternates rows instead of dimming every row
  equally. Fog retains encoded-SDR blending; a linear-light, coverage-aware fog
  treatment remains a separate feature.
- Compare quantization before display effects with final quantization, which
  constrains vignette, scanlines and grain to the selected output colors.

UI and the crosshair stay clear. Dither patterns are screen-relative; this lab
does not yet implement surface-stable fractal dithering. Revisit the same camera
poses and inspect stationary and moving views, not just screenshots. Scanlines
are beam-inspired modulation, not a complete CRT simulation.

From the CLI root, after building this sample with the intended host:

```sh
mise run check-effect-lab
```

The browser check snapshots the complete distribution, records host/cartridge
hashes, exercises controls, checks before/after restoration and FPS movement,
and saves comparison images and a report under ignored `tmp/`. It refreshes
`preview.jpg`. The checker uses hardware-accelerated Chromium and records its
actual GPU. Synthesized input does not replace manual review.

For a focused preset capture after changing their tuning, run
`mise run check-effect-lab-presets`. Its report records the narrower scope;
it does not replace the full movement and control check above.

For the grading and outline controls specifically, run
`mise run check-effect-lab-color`. Captures and the report distinguish this
focused scope from the complete room/control check.

For outline modes and thresholds alone, run `mise run check-effect-lab-outline`.
The geometry pass retains RGBA8 data and its scalar mask uses R8: five bytes
per scene pixel instead of eight, about 9.89 MiB at 1920×1080. A 16 MiB preflight
limit bounds these two targets; it does not reserve device memory. The shader
uses explicit high-precision samplers/integers and four explicit neighbor calls,
preserving the existing edge equations while simplifying mobile compiler input.
Eligible Y-down single-context outline views write directly into the geometry
target, removing one fullscreen copy; other orientations, portals and unsupported
attachments keep the composition fallback. Reported draw counts follow the
selected path. Depth-only mode skips normal decoding, crease-only mode skips
depth decoding, and zero softness skips four composition neighbor samples.

If optional outline setup fails, the scene and other effects keep rendering,
partial outline resources are released, and an inline message appears beside
the outline controls. Turn outlines off and on to retry. Failed setup does not
retry allocation every frame. This handles errors returned by shader, texture
or framebuffer setup; recovery from a driver reset or browser tab termination is
not guaranteed.

Run `mise run check-effect-lab-outline-failure` to inject one mask-allocation
out-of-memory result in the real browser. The focused fixture checks that WASM
frames advance, vignette pixels remain identical, the warning is visible,
resource counts stay stable and the off/on retry restores outlines. The hook
is installed only for this scope. It does not reproduce a physical phone's
driver or prove mobile performance.

To reuse a complete cartridge baseline with a candidate host, override only
the checker's copied inputs:

```sh
KARTY_EFFECT_LAB_DISTRIBUTION=/absolute/path/to/baseline/web \
KARTY_EFFECT_LAB_HOST=/absolute/path/to/candidate/karty-host.wasm \
mise run check-effect-lab-outline-failure
```

The host override also copies its adjacent `wasm_exec.js`. The baseline stays
unchanged; the report records the exact cartridge, host and glue hashes.

## Measure effects

The immutable preview created by the browser check also includes **Measure on
this device** in the same Effects panel. Quick runs are exploratory; Sustained
uses three rounds with reversed candidate order and clean baselines at both ends.
Keep the camera still and the tab visible. The active presentation profile is
pinned for the comparison and restored afterward, along with the original
effects. Resize, context loss, hidden tabs and camera/scene changes invalidate
the capture. Cancel stops the capture and restores settings.

An enabled outline with positive strength must report active without a setup
error. Otherwise the benchmark stops and restores the original effects/profile,
instead of recording an unavailable outline as a cheap successful result.
Rows include outline activity, target bytes and setup error when the host
provides those diagnostics; older hosts report them as unknown.

Color edits keep the 33³ RGBA8 grading lookup quality. The default path bakes
the complete lookup with one GPU draw on a changed active key, then reuses it;
there is no additional fullscreen pass or HDR buffer. Optional startup
preparation compiles its shader and draws an identity lookup into the retained
1089×33 atlas/framebuffer once. First-use and changed-key timing still need
device evidence; this architecture does not certify a frame or UI-latency budget.
If GPU baking is unavailable, CPU generation advances in batches of 32 nodes
with a one-millisecond time check and a 4096-node cap per frame; one batch can
cross the deadline, and allocation/final upload can add time. The finished
fallback lookup is uploaded atomically. Captures and benchmarks wait for
`gradePending` to clear so a selected treatment cannot accidentally compare the
previous lookup. Settlement stays within the existing five-second window and
is excluded from warmed timings. JSON reports separate wait duration and
observed per-frame build slices; these observations are not a complete build
total. Build-millisecond/node diagnostics reset to zero on warm frames; nodes
count CPU generation only, and milliseconds measure CPU preparation/queueing
rather than GPU completion. `gradeGPU` records the published lookup's origin,
not a recurring bake draw; `gradeBakeError` retains any GPU fallback reason.

Each run measures the fourteen effects individually, all effects together, and the
stack without outlines, including separate water/grime/blood windows. Dithering includes quantization; saturation, contrast and
grading use nonneutral settings. The combination of sharpening and chromatic
aberration evaluates fifteen scene samples; assess that combination separately.
Lens surface data retains one shared 3 MiB CPU atlas decoded at host startup,
with an optional cached 3 MiB GPU atlas per camera and no extra full-screen
render target. Layered uploads reuse browser staging with a 1 MiB requirement
instead of growing it to 4 MiB. Shader compilation and first data upload are first-use costs;
Clean startup attempts optional composition-shader preparation without making
an inactive-effect failure fatal. This moves preparation earlier and does not
establish a cold-frame or first-toggle budget. Vignette and scanlines share one
linear-light decode/encode; zero dither and underwater distortion skip unused
work. Measurements below describe warmed rendering. CPU preparation, asynchronous GPU timestamps,
frame cadence and p95 are separate. The panel reports GPU p95, late frame counts
and the 16.67 ms total frame budget. Postprocessing must leave room for the rest
of the game. A window exceeding five seconds stops immediately; that watchdog
is separate from the per-frame performance target. GPU deltas require at least sixteen completed
samples without disjoint results or missing queries. Deltas are withheld when
bracketed clean GPU means drift by more than 5%; raw timings remain available.
The first clean window warms for at least sixty frames to help GPU clocks settle. Download the JSON to retain
the device, resolution, settings, samples, repeated baseline drift and resource
revision counts. The GPU budget label describes measured world/composition work,
not complete frame delivery. Raw RAF intervals near 16.67 ms include timestamp
rounding; their threshold count alone does not establish missed refreshes.
Measurements include diagnostic polling overhead and do not
measure display latency. They are comparisons at the current settings, not a
worst-case benchmark of every algorithm and slider extreme.

Open the same preview on a physical phone for mobile measurements. Select its
render profile before measuring; compare p95 cadence with 16.67 ms for 60 Hz or
33.33 ms for 30 Hz and inspect sustained baseline drift. Software rendering is
not evidence of phone FPS. For an automated exploratory run against the shared
loopback preview, use `mise run bench-effect-lab`; `KARTY_EFFECT_LAB_URL` can
select its URL. This command uses verified hardware acceleration at 1280×720 with the medium
presentation profile, rejects software fallbacks, records all effects, checks
warmed resource stability and writes `tmp/effect-lab-performance-report.json`.
On Linux it uses surfaceless EGL and needs access to `/dev/dri/renderD128`; a
sandbox that hides GPU devices must be exited for this check. The local AMD
results qualify that development GPU, not phone performance.

For the new lens effects specifically, run `mise run check-effect-lab-lens` after
building with the matching host. Its focused hardware captures cover
radial/offset distortion, native and half-resolution sharpening, the three lens
surfaces, water refraction and the Water specular slider, combined optics, neutral/master bypass and atomic
invalid-setting rejection. All browser actions have five-second bounds.
