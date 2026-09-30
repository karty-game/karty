# QOI/QOA asset pipeline implementation plan

Status: implemented in the SDK, CLI, and engine worktrees for the SDK 0.0.4 candidate. Automated codec, cache, native, and web checks are in place; immutable SDK publication, released-dependency integration, performance baselines, and manual audible playback remain release gates.

This file preserves the original delegation plan and acceptance checklist. For the current author-facing behavior, see [Image and sound assets](../assets.md).

## Outcome

Authors keep PNG, JPEG, WebP and WAV source files. `karty build` and `karty dev` discover and process those sources automatically, reuse verified processed output, and embed QOI textures and QOA sounds in cartridges. Hosts consume the embedded output without authoring tools or access to engine source. Native and web builds use the same processed assets.

The CLI owns conversion, caching, packaging and customer integration tests. The public SDK owns shared asset contracts; the engine owns decoding, graphics and playback. KartUI continues to refer to logical texture names and dimensions. No engine-to-CLI dependency, new command entrypoints, copied third_party directories, or tracked generated project assets are needed.

Deliver in two milestones: images plus the reusable cache first; WAV/QOA plus actual sound playback second. The complete task includes both. Long-form streamed music, level-scoped audio and a full mixer API are explicit follow-up work, not prerequisites for initial game-scoped sound effects.

## Starting repository state (historical)

- CLI `internal/assetpipeline/pipeline.go` supports `copy-png@1`. Its cache checks the output against the source hash because output currently equals input. Source and output identity must become separate concepts.
- `internal/project/project.go` discovers only PNG textures. `internal/build/build.go` prunes unused game textures and embeds cached bytes.
- `internal/levelbuild/levelbuild.go` reads and packages level textures separately; it bypasses the asset cache. Both paths must share the new processor.
- `internal/commands/dev/dev.go` watches PNG but needs the additional extensions.
- SDK `format/cartridge/assets.go` stores names and opaque bytes, with an 8 MiB per-asset and 16 MiB aggregate limit. `format/level/envelope.go` accepts data and texture entry kinds. Encoded-size bounds do not bound decoded memory.
- Engine `host/internal/render/scene.go` decodes game textures and explicitly requires PNG for level textures. Both loading paths need the same bounded QOI decoder.
- Engine `host/host.go` and `host/web_js.go` only log `PlaySound` calls. There is no completed sound asset/playback pipeline to switch to QOA.

## Contract checkpoint: finish before parallel implementation

The integrating agent owns a short contract change specifying the following. Other tasks consume that change rather than independently inventing interfaces.

1. SDK processor identifiers: proposed `qoi@1` and `qoa@1`, alongside existing `copy-png@1`. Define capabilities and required CLI support in the selected SDK manifest; unsupported processors fail before expensive compilation. Readiness must never be inferred from the SDK version number alone.
2. Typed processed-asset result: kind, encoding, source digest, output digest, cache recipe key, output path, source/output sizes, and bounded image/audio metadata. Image metadata includes width/height; audio includes channels, sample rate, sample frames and decoded-byte estimate. Keep project names and source paths outside reusable content identity.
3. Processor interface: inspect bounded input, resolve a versioned transform recipe, then transform and encode deterministically into bytes plus metadata. Freeze the transform option schema, precedence, operation order and rounding rules before parallel work. The cache owns I/O/publication; processors do not write arbitrary paths. Accept an immutable source snapshot or a validated stream so inspection, hashing and conversion cannot observe different source contents.
4. Wire decision: retain the existing opaque texture payloads for QOI, identify supported image encodings by magic bytes, and gate their use through SDK/host compatibility. Add a separately versioned, typed sound custom section to game cartridges, with a shared SDK encoder/decoder and bounded name/ID/payload records. Do not place sound bytes in the texture list. Validate unknown versions, duplicate IDs, unknown codecs, lengths and aggregate limits before mounting.
5. Sound identity: define one deterministic mapping from logical names to nonzero IDs, generate named client constants, and store the same IDs in the sound section. Sorting names is sufficient for IDs local to one build; document that IDs are not persistent save-game identifiers. Specify missing-ID behavior and simultaneous voice limits. Reuse existing `play-sound(u32)` for initial one-shot playback; change WIT only if that contract is actually insufficient.
6. Set concrete resource budgets before merging implementation: source bytes, image dimensions/pixels, decoded texture bytes per cartridge/level, sound duration, decoded PCM bytes, total assets and active voices. Derive defaults from sample measurements and current cartridge bounds; test each boundary. Do not increase existing wire limits merely to fit a fixture.
7. Choose and pin pure-Go codec implementations after checking licenses, reference interoperability, malformed-input handling and supported targets. Prefer maintained dependencies; no external converters or CGO in the installed CLI. If a small owned implementation is needed, place shared codec code under public SDK `codec/qoi` / `codec/qoa`, with attribution and conformance tests. Keep CLI source importers and engine playback adapters in their owning repositories.

This checkpoint includes the decision on whether cartridge compatibility metadata needs a new version to declare the sound section as required. An old host must reject an incompatible new cartridge rather than silently ignore its audio. Published contracts/SDKs remain immutable; incompatible wire/API changes need an explicit version decision before implementation.

## Library selection and evaluation

The pipeline runs inside the Go CLI without FFmpeg, ImageMagick, CGO, or separately installed converters. The implementation selected these pinned components:

| Operation | Selected implementation | Version / license |
| --- | --- | --- |
| PNG/JPEG decoding | Go `image/png` and `image/jpeg` | Go 1.27 standard library, BSD-style |
| WebP decoding | [`golang.org/x/image/webp`](https://pkg.go.dev/golang.org/x/image/webp) | v0.46.0, BSD-3-Clause |
| Image resizing | [`github.com/disintegration/imaging`](https://github.com/disintegration/imaging) nearest/Lanczos filters | v1.6.2, MIT |
| QOI codec | Public SDK bounded adapter over [`github.com/hchargois/qoi`](https://pkg.go.dev/github.com/hchargois/qoi) | v1.0.0, MIT |
| WAV decoding/normalization | Karty-owned bounded RIFF parser in the CLI | CLI MIT license; no WAV dependency |
| Audio resampling | [`github.com/cwbudde/algo-dsp`](https://pkg.go.dev/github.com/cwbudde/algo-dsp) `QualityBest` | v0.7.1, MIT |
| QOA codec | Public SDK bounded adapter over [`github.com/braheezy/qoa`](https://github.com/braheezy/qoa) | v1.0.3, MIT |

The adapters validate reference fixtures, lengths, decoded-memory budgets, and complete encoded output. The CLI processor revisions include the importer, transform, normalizer, resampler, and codec choices, so a behavior-changing dependency update invalidates the affected cache recipe. Runtime QOI/QOA adapters stay in the public SDK; source import and transforms stay in the CLI.

Keep library types behind small Karty-owned adapters so a failed candidate can be replaced without changing asset profiles or shared wire contracts. Karty owns profile resolution, source limits, normalization policy, cache publication and reports. Importers and image transformations belong in the CLI; the engine needs runtime QOI/QOA decoding and audio playback adaptation, not the authoring dependency stack. Share codec adapters through the public SDK only where useful, with no copied dependency trees.

Packet A recorded the selected module versions and licenses above. The codec and processor suites cover reference interoperability, malformed input, promised WAV variants, resampling behavior, deterministic output, and CGO-free supported targets. Runtime adapters compile for native hosts and `js/wasm`. Manual listening and checked-in performance thresholds remain acceptance work.

Accepted versions are pinned in their owning modules. Dependency updates that affect decoding, normalization, resampling, or encoded output must run the fixture corpus and advance the matching processor/cache revision when behavior changes.

## Processing policies

### Configurable transformations

Transformation is a first-class step between decoding source assets and encoding runtime assets. Preserve author-owned originals; only derived cache/package output changes. Apply the same recipe in build and dev and to game and level textures.

Resolve settings in this order: SDK profile defaults, project profile overrides, then per-asset overrides. Materialize canonical effective options before calculating the cache key. Unsupported options and incompatible combinations must fail clearly. SDK defaults remain pinned; changes to transform behavior require a processor revision.

Initial image controls: maximum width/height, aspect-preserving fit inside those bounds, no upscaling, explicit nearest-neighbor or smooth filtering, and conversion to 8-bit channels. Nearest-neighbor serves pixel art; smooth filtering serves backgrounds and UI. Pin the smooth filter and alpha/color treatment so output is reproducible. Keep cropping, atlas generation and arbitrary transform scripts out of this milestone.

Proposed TOML shape (contract checkpoint must finalize and validate it):

```toml
[assets.texture-profiles.background]
base = "environment"
max_width = 2048
max_height = 2048
filter = "smooth"
bit_depth = 8

[[assets.texture]]
name = "background"
source = "assets/textures/background.png"
profile = "background"

[assets.texture.transform]
max_width = 1024
max_height = 1024
```

Here an 8192×4096 image becomes 1024×512; a 512×256 image stays that size. Output dimensions use deterministic rounding, remain at least one pixel, and never exceed the requested bounds. A 16-bit PNG can be reduced to 8-bit channels by the configured profile, with that lossy precision reduction recorded. Initial standard profiles declare the required 8-bit conversion but do not silently resize; authors opt into resolution caps through profiles or overrides. Emit actionable diagnostics for excessive dimensions/decoded memory, including a suggested profile setting.

Freeze size semantics before integration: inspect whether sprite defaults, UI intrinsic layout, nine-slice borders and source rectangles use texture pixel dimensions. Retain original and processed dimensions plus the scale mapping. Preserve logical display sizes by carrying logical dimensions where needed and mapping pixel coordinates consistently; if an existing consumer cannot express this safely, reject that transform with an explanation until its contract is extended. Do not silently change UI layout or invalidate sprite regions when downscaling.

Audio controls: target sample rate and preserve/mono channel policy, with required PCM16 normalization before QOA encoding. Proposed supported target rates are 22050, 24000, 44100 and 48000 Hz; default to 48000 Hz. Allow explicit stereo-to-mono conversion for effects, with pinned mixing, clipping and rounding rules. Do not silently downmix. Lower rates must remain lower in packaged QOA, with host playback adapting to its audio context rate; do not immediately upsample in the CLI and lose the storage benefit. PCM bit depth is fixed at 16 for QOA, not an arbitrary output-quality option.

Reports show original → processed dimensions, bit depth, sample rate, channel count, encoded/decoded costs, effective recipe and reasons for each change. Include warnings for precision reduction and quality-affecting transforms. Provide enough data to tune profiles without changing source files.

Source safety limits are separate from target budgets: shrinking a huge image does not make its initial decode safe. Inspect source headers before allocation; bound decoded working memory and concurrent transforms. Reject inputs exceeding safe source limits even if the requested output is small. Streaming/tiled import can be added later where a decoder supports it.

### Images

- Accept `.png`, `.jpg`, `.jpeg`, `.webp` in automatic discovery and explicit declarations, including level textures. Match actual content to an allowed decoder; report mismatches and duplicate inferred names clearly.
- Normalize to 8-bit straight-alpha RGB/RGBA before encoding. Handle paletted/grayscale images and transparency explicitly; test translucent edges through Ebiten to catch premultiplication mistakes.
- QOI preserves normalized pixels, not arbitrary source metadata or original high-bit-depth samples. Accept 16-bit PNG through the explicit profile bit-depth conversion described above; specify deterministic quantization to 8-bit channels and report the precision loss. Reject animated PNG/WebP rather than silently choosing a frame.
- Treat decoded channel values as sRGB; do not introduce an implicit ICC conversion. Initial JPEG policy follows stored pixel orientation (no automatic EXIF rotation), documented and covered by an oriented fixture. These normalization rules are part of the processor revision.
- Use identical encoding policy for dev and release initially. Keep asset names, logical sprite sizes, UI theme references and level-local IDs stable; validate transformed dimension/coordinate mappings.
- Evaluate representative sprites, UI, transparent textures and photographic backgrounds. QOI is a lossless pixel format designed for fast decoding; smaller downloads than PNG/WebP are not guaranteed. Compare actual cartridge sizes and compressed web delivery sizes as well as startup CPU and memory. [QOI specification/reference](https://github.com/phoboslab/qoi/blob/master/qoi.h)

### Audio

- Add discovery under `assets/sounds/` and proposed `[[assets.sound]]` declarations with logical name, source and profile. Validate the TOML spelling at the contract checkpoint, then use it consistently in docs and tests.
- Initial importer supports RIFF/WAVE integer PCM 8/16/24/32-bit and IEEE float32, mono/stereo. Specify conversion to signed PCM16 (rounding, clipping, nonfinite float rejection) and cover it with fixtures. Unsupported compressed WAV, RF64 and unsupported extensible variants fail explicitly. Parse RIFF chunk padding and lengths; do not assume a fixed 44-byte header.
- Normalize to the profile-selected sample rate (default 48 kHz) using a pinned deterministic resampler. Preserve mono/stereo unless explicit downmixing is requested; adapt the encoded channel count and rate to the playback API at the host boundary. Include input rate, target rate, resampler revision and conversion policy in recipe identity. Confirm host resampling and playback-rate handling against the pinned Ebiten audio API during the checkpoint.
- QOA is lossy and encodes PCM16. Expose that fact in SDK docs and build reports. Do not add an imaginary quality slider; the initial processor uses a fixed, versioned encoder policy. [QOA specification/reference](https://github.com/phoboslab/qoa/blob/master/qoa.h)
- Start with bounded game-scoped one-shot effects, decode once at load and reuse PCM for repeated playback. Retain all declared sounds initially, with that reason in the report; introduce static sound stripping only when reference analysis is proven conservative.
- Streaming music, seeking, loop metadata, spatial audio, buses and level-scoped sound lifetimes are follow-ups. Reject oversized effects with an actionable error rather than exhausting memory.

## Cache design

Use the existing project-owned `.karty/cache/assets/` with a new cache schema. No global cache or remote service in the first implementation.

Recipe identity is a hash of source bytes, asset kind, processor revision, canonical effective options, importer/normalizer/resampler revisions and cache schema. SDK identity participates only when it changes those inputs: unrelated SDK changes should not require recompression. Output digest is calculated independently from encoded output. Pin implementation revisions so a dependency update that changes output invalidates the recipe.

Store each recipe as one committed directory containing encoded output and metadata. Write into a unique temporary directory, validate, then publish it as a complete entry. Concurrent writers must converge on a validated winner; never delete a valid entry before replacement. Specify repair/locking behavior for corrupt entries on Windows as well as Unix. Interrupted temporary entries are ignored and can be cleaned safely.

On a hit, validate metadata schema, recipe, lengths and output digest before packaging. Recover from missing, truncated, mismatched or corrupt entries by rebuilding. Cache metadata is derived, untrusted input: recheck metadata bounds/consistency with the encoded header before allocating. A warm hit must avoid source image/audio decoding and encoding; source hashing and output integrity reads still occur. Do not rely only on mtime and file size.

Deduplicate identical recipes across game and level scopes within a project while preserving logical ownership and IDs. Prune game textures before conversion. Keep level retention semantics initially. Use bounded worker concurrency and a decoded-memory budget rather than one goroutine per asset.

Preserve path confinement, including symlink escape rejection and generated-file ownership. Rebuild from a clean cache must yield identical output; packaging must consume the validated processed bytes, never accidentally reread the original source. Atomic cache publication is distinct from publishing final build output: conversion failure must not stage a partial new game.

Extend `asset-report.json` with source/output digests, kind/encoding, recipe key, real cache path, hit/miss reason, dimensions or audio properties, and separate encoded/decoded costs for game and level assets. Keep timings out of deterministic package data. Remove stale references when assets are renamed/deleted; automatic size-based eviction is deferred. Document that deleting the derived cache is safe.

## Delegation packets

Every packet must read applicable AGENTS.md, preserve unrelated work, avoid commits/publication, and return changed files, exact checks/results, outstanding limits and integration instructions. No packet may revise a frozen contract independently; send a proposed adjustment to the integrator.

| Packet | Owned scope | Depends on | Required handoff |
| --- | --- | --- | --- |
| A — shared contracts and compatibility | SDK `format/` and chosen codec adapters; engine SDK manifest schema/capability definitions; contract fixtures | None | Frozen processor/result and transform configuration interfaces, size/coordinate semantics, version decision, sound format, limits, library evaluation record and pinned dependency decision, conformance fixture corpus |
| B — reusable processor cache | CLI `internal/assetpipeline/` cache/result/report files and cache tests | A | Atomic concurrent cache, source/output separation, deterministic recipe keys, corruption recovery, bounded scheduling; fake processors prove behavior |
| C — image import | CLI image processor files within `internal/assetpipeline/`, image fixtures/benchmarks | A; integrate with B | PNG/JPEG/WebP → QOI, resizing/filtering and bit-depth transforms, canonical pixel rules, malformed-input and alpha tests, baseline size/time results |
| D — audio import | CLI audio processor files within `internal/assetpipeline/`, WAV fixtures/benchmarks | A; integrate with B | WAV parsing, configurable resampling/downmixing and PCM normalization, QOA output, deterministic reference fixtures and measured error |
| E — host texture support | Engine `host/internal/render/` and related texture validation tests | A | Same bounded decoder for game/level textures, PNG compatibility, transactional level mounts, native/web graphics validation |
| F — host audio and client names | Engine host sound service, runtime sound loading, `core/internal/codegen/` templates and relevant WIT only if approved | A and D fixtures | Sound catalog, generated names, actual one-shot playback native/web, browser activation, voice limits, rate/channel adaptation and resource cleanup |
| G — CLI integration and authoring | CLI `internal/project/`, `internal/build/`, `internal/levelbuild/`, dev watcher, generation integration, sample assets | B–F | Shared game/level texture processing, audio discovery/packaging, profile/override resolution, report aggregation, logical size/coordinate preservation, sample demonstrating all source types |
| H — acceptance, docs and release preparation | CLI customer integration scripts/tests; engine versioned SDK docs/templates/coverage; release compatibility matrix | G | Public-dependency end-to-end results, performance report, docs, proposed immutable SDK release and CLI rollout |

Scheduling: finish A first. Then B, C and E can run together. D follows C if sharing processor wiring; F follows E if sharing host lifecycle files. G is owned by the integrator after workers hand back their changes, followed by H. With three workers plus an integrator, queue excess tasks rather than letting workers overlap ownership. Assign individual cache/image/audio filenames before launching B/C/D; only B edits common processor interfaces, and only A changes shared SDK contracts.

## Acceptance gates

1. Codec correctness: independent reference fixtures, not just our own encoder/decoder round trips. QOI must reproduce canonical pixels exactly; QOA must match expected decoded reference PCM for fixed bitstreams. Validate audio length/channel order, objective error measurements and listening on representative effects; set corpus-specific quality thresholds before accepting the encoder.
2. Transform correctness: test profile precedence and invalid settings, aspect ratio/rounding/no-upscale, pixel-art and smooth filters, transparent edges, 16-to-8-bit precision conversion, UI/nine-slice/source-rectangle mappings, resampling duration/pitch, channel mixing and clipping. Test source-memory limits independently of output dimensions. Changing one effective transform must invalidate only affected recipes; equivalent resolved options reuse output.
3. Cache behavior: cold build encodes; warm build performs zero conversions; one source change rebuilds only that recipe. Cover same-length/mtime edits, changed options/revisions, unrelated SDK bump, duplicated sources, deletion/rename, corrupt payload/metadata, cancellation, concurrent builds and interrupted publication. Exercise Windows publication semantics on Windows.
4. Robustness: fuzz image/audio/header and sound-section decoders; truncated inputs, overflowing dimensions/sample counts, invalid QOA frames, trailing/malformed chunks, path escapes and aggregate decoded-memory budgets must fail without panic or partial mounts.
5. Packaging: inspect `.kart` and `.kld` payloads to prove retained images are QOI, sounds are QOA, source files are not duplicated, names/IDs match generated bindings, fonts/UI are unaffected, and encoded/decoded limits are respected. Repeated clean builds produce identical processed assets and deterministic asset sections.
6. Compatibility: legacy `copy-png@1` SDK builds still work; unknown processors fail early; new SDK hosts accept declared new assets; old hosts fail clearly on cartridges requiring new features. Test against released public dependencies with `GOWORK=off`, not only sibling replacements.
7. Execution: CLI-owned integration builds a fixture game and runs published/candidate hosts. Test real native and browser texture rendering including translucent UI and level mount/unmount. Audio tests verify PCM delivery through an injectable sink and browser activation/playback progress; actual audible playback needs a separate manual check. Compilation or a logged sound ID does not count as playback validation.
8. Platforms: compile all supported CLI and host targets, execute platform-specific cache tests in CI, and run native/web integration gates. No image/audio converter needs installing on player machines or development machines beyond the CLI.
9. Performance: record cold/warm build time, source/processed/cartridge bytes, web transfer bytes, host startup decode time, peak decoded memory and repeated-play allocations. Require zero processor calls on unchanged warm builds and bounded resource use. Set numerical regression thresholds from a checked-in representative fixture corpus before merging; do not claim faster loading from format choice alone.

Use repository-owned mise tasks: CLI `test`, `build`, `smoke`, `check-web`, `check-browser`, `check-dev` and relevant allocation checks; engine `generate`/`check-generated` when bindings change, `test`, host builds and relevant allocation checks. Add focused processor/codec tests and benchmark tasks where needed. Record exact commands and distinguish automated execution, rendering assertions and manual audio review.

## Rollout

1. Merge and publish any new public SDK Go contract/codec version required by both repositories.
2. Engine publishes a new immutable SDK bundle and native/web hosts advertising the new processors, with matching versioned authoring docs and templates. Choose the next unused version at implementation time; do not overwrite an existing release or silently repurpose `copy-png@1`.
3. CLI consumes the public SDK module and tests the released runtime artifacts before its own release. Existing projects keep their pinned SDK behavior; new defaults move only after these gates pass.
4. Document the source asset formats, WAV restrictions and lossy conversion, profile selection, cache behavior, report fields, troubleshooting and migration. No KartUI release is necessary unless its public schema actually changes.

No release workflow is dispatched by this plan. Implementation is complete only after both milestones pass their gates; release publication remains a separate authorized action.
