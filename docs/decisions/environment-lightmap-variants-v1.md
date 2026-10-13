# Environment lightmap variants v1

Status: implemented. Owners: public SDK formats/baker, CLI offline
building, engine resource loading and rendering. Date: 2026-10-13. Target:
SDK 0.0.12; earlier manifests and protocol commands are unchanged.

## Decision

Add optional `[lightmap.environment]` with ascending unique `times` and an
optional `default_time` (15 by default). It requires enabled offline lightmaps,
render chunks and the new `world/lightmaps-environment-variants@1` capability.
`weather_variants` defaults false: at most 32 clocks each produce one clear image.
Explicit `weather_variants=true` retains optional clear/overcast pairs, at most 16
clocks and 32 images. Unknown options,
mixed point/import recipes and finite sky-ray distances are rejected.

The separate public `cpu-environment-rnm3@1` producer bakes directional sun and
sky diffuse with opaque BVH visibility and optional diffuse reflection. Existing
point-light producers, manifests and golden pixels retain their semantics.
Time-only maps retain their shadow pattern and direction as cloud cover changes
a cheap runtime brightness multiplier. This does not select, blend or bake
weather maps. The opt-in paired producer linearly suppresses sun from clear to
overcast. Translucent liquid
materials transmit rather than block opaque transport; this first producer does
not model tint absorption or refraction. Actor, point-light, authored ambient and
specular contributions remain live.

Each ordinary receiver chunk owns one immutable layout, a typed bounded `KLV1`
catalog, an embedded clear default and content-addressed passive condition
modules. Catalog and image identities bind complete geometry, surface opacity,
layout, used reflectance, sampling and exact sizes/hashes. Complete binary/QOI
validation precedes allocation or residency replacement. The usual build bakes
all endpoints ahead of execution, using the existing confined dependency-keyed
pixel cache. Small catalogs and condition modules are rebuilt deterministically
on warm builds; the old complete-chunk cache is bypassed for these resources so
no condition child can disappear from staging.

The existing environment control command selects a nearest authored clock.
Time-only catalogs contain distinct increasing clock records with cloud 0; the
selector returns that index twice and blend 0 for all cloud amounts. Existing
paired catalogs retain clear/overcast ordering and return a continuous weather
blend. This extends candidate KLV1 validation without adding wire fields.
Consumers decode RGBM to linear coefficients before optional paired blending
and upload one active RNM map per resident chunk. They retain the
current map during asynchronous fetch, reject stale completions and bound global
transition scratch independently of resident chunk bytes. Conditions are child
resources, never additional game levels or actor instances. Existing shell
content addressing/staging is reused; no protocol or WIT change is introduced.

## Optional receiver ownership

Build-only SDK `Options.SolidOwners` supplies a complete validated owner per
solid. Nil preserves default first-overlap ownership. The opt-in
`BalancedSolidOwners` policy places largest chart sets first into least occupied
eligible sectors only after exact page fitting. Conservative XY intersection,
ordinary-component confinement, complete ownership and all existing allocation,
mesh, texel and encoded bounds remain in force. The CLI enables this only for
environment variant builds; it never changes geometry or reduces density.

SDK 0.0.12 adds a geometric post-pass within that opt-in owner policy.
Narrow grounded solids prefer the first overlapping sector whose exact receiver
layout fits, while every other balanced owner is retained. Longest XY extent
must be at least 2 units and four times the shortest; shortest extent is at most
0.6, maximum height 1.25, top at most 1 above the receiver floor, and bottom at
or below that floor. Names and materials do not select the policy. Each trial
preserves the existing page limits and final complete physical confinement
validation. Inability to concentrate the selected set retains the original
balanced list. Traversal and selection remain bounded by validated sector and
solid counts. CLI complete-chunk cache revision 4 prevents reuse of plans from
the earlier owner heuristic; pixel identity still follows the exact layout.
No new format, authoring field, capability or released manifest is introduced.

Environment builds also opt into `CapsWithinSourceRoom`, `CompactPacking` and
`CompactLightmaps`. Cap charts retain authored room and prefab instance
boundaries. Bounded free-rectangle packing can turn charts by 90 degrees while
updating their right-handed RNM basis and affine UVs together. Compaction merges
complete valid receiver groups only after exact layout, mesh and encoded-budget
checks. It preserves authored grouping, complete ownership and physical
component confinement; it never increases total decoded map bytes. Default
planning, shelf packing and point-light golden layouts remain unchanged.

SDK 0.0.12 widens mesh/catalog receiver membership to permit nonadjacent
selected sectors within one complete ordinary component. Validators traverse
that component using buffers bounded by the validated sector count and reject
every selected sector outside it, including transformed-only connections and
disconnected spaces. No field or binary version changes. Producers of these
environment cartridges require the new variants capability, which older hosts
reject before loading. Released manifests are retained unchanged.

The candidate chunk producer also retains finite upper sky-mask geometry at
outdoor portals with unequal ceiling heights, including compiled frame regions.
Masks have sky metadata and no lightmap chart; lower opaque spans and indoor
lintels retain their receivers. Complete mesh validation rejects missing masks
or incorrect flags. This matches the ordinary renderer without changing static
transport surfaces, layout/image identities or the environment bake producer.
CLI complete-chunk cache revision 3 prevents reuse of the earlier mesh assets;
existing condition pixel caches remain valid.

## Sample budget and tradeoffs

Open Air uses exactly four time states: Night 0, Morning 9, Noon 12 (default),
Evening 17. Cloud cover changes intensity while retaining those baked shadows.
It uses 24 texels per world unit, sixteen samples, one bounce and medium denoising.
Its six render chunks use 512/1024 pages with padding 2 and produce 24 time-image
modules, with identical content
deduplicated by SHA256. The empty field and courtyard use ordinary connected
receiver cells that retain their visible geometry and world-anchored grass UVs.
The fake upper storey uses thin structural sections rather than filled volumes.
Existing page bounds remain 512/1024. Density 32 exceeds base receiver capacity
and is rejected. Large decorations are distributed across their valid XY
receiver sectors. Packing is checked against actual visible residency before
baking; the host's 192 MiB residency limit is retained. Reducing sample padding
from 4 to 2 improves packing without changing density or geometry. Denoising is
chart-isolated; two texels retain the existing linear RNM sampling margin.

A 512 RNM map costs 3 MiB decoded (1024 costs 12 MiB) even when QOI downloads are small. Build
time and compressed storage scale with conditions; steady GPU map count does
not. Transitions require bounded CPU decode/blend/re-encode work and uploads.
Four sky samples prioritize initial build cost; higher final sampling reduces
sky/bounce noise. Time selection is discrete and intermediate clocks use the
nearest authored map; cloud brightness changes continuously without a map
transition. Physical phone and browser backend
qualification remain separate from public CPU baker and WASM module tests.
