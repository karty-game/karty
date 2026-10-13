# Materialize authoring v1

Status: implemented. Owner: public CLI. Date: 2026-10-09.

## Decision

Add optional `[textures.materialize]` with `preset`, `normal_strength`,
`height_contrast`, `metallic_scale` and `ao_depth_scale` to `level.toml`.
Omitted values preserve preset defaults; explicit zero is meaningful. Numeric
values must be finite in [0,16]. Unknown fields and presets are rejected.
There is no seamless switch: Materialize handles this automatically.

Each referenced world surface is normalized to the existing 256×256 working
resolution, processed and cached independently, then assembled into the existing
atlas and mip format. The processing recipe and pinned executable identity are
part of each asset cache key. This replaces whole-atlas material generation and
requires a processor cache revision; generated output can differ with unchanged
source/defaults because analysis is now confined to each asset.

## Compatibility

An omitted table remains valid. Controls require a world, an atlas-capable SDK
and a used surface texture. Runtime `material_strengths` remain independent and
do not invalidate generated maps. No runtime capability or atlas schema changes
are introduced. The initial implementation packaged normal/height/AO. Candidate SDK 0.0.11
subsequently adds optional `world/material-reflectance@1` metallic/smoothness
companions; matching hosts use `metallic_scale` through direct specular shading.

## Tradeoffs

Warm builds reuse unaffected material maps. Cold builds run more native process
invocations. Shared encoding/resampling rules provide technical consistency;
albedo brightness, color and authored surface differences are not equalized.
Actual GPU execution and visual comparisons require separate smoke checks.

## Explicit nonmetal correction (2026-10-13)

Real sample atlas inspection found that the pinned tool adds specular evidence
after multiplying its metallic detector by `metallic_scale`, leaving nonzero
metallic pixels even with an explicit zero. The CLI clears only the generated
metallic channel for explicit zero after validating all tool maps. Omitted and
positive scales retain the tool result, and smoothness remains unchanged.
Material asset processor revision 3 and atlas processor revision 7 invalidate
both cache layers. The existing authoring/wire format and pinned tool remain
unchanged. Regression coverage checks packed tiles, gutters, mips, warm reuse
and regeneration when the explicit zero is removed.

## Validation (2026-10-09)

- `mise run build`: passed; CLI compilation only.
- `GOCACHE=/tmp/karty-go-cache mise run check-materialize-pipeline`: passed;
  per-asset regeneration, atlas reorder/cache reuse, coverage, mipmaps,
  cancellation, malformed-output rejection and authoring validation.
- `mise run check-materialize-software`: passed with network-enabled execution;
  checksum-pinned Materialize generated actual 256×256 normal/height/AO maps
  with `metal` and all four overrides, and the warm build reused its cache.
- `mise run test`: stopped at the existing selected SDK 0.0.11 download's HTTP 404. `GOCACHE=/tmp/karty-go-cache mise run test-unit` also encounters that
  missing SDK and sandbox socket restrictions in unrelated package checks.

No new guest WASM execution, browser graphics or Hangar visual review was
performed. Cold-build timing for a full level remains unmeasured; the existing
90-second aggregate generation deadline remains in force.
