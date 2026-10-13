# Build-generated lightmap coordinates

SDK 0.0.8 accepts `world/lightmaps@1`. Add an optional
`[lightmap]` table to a world level's `level.toml` to prepare receiver coordinates
while building YAML into KLD. This path uses public SDK helpers and does not
need private engine source, Materialize or GPU access.

```toml
[lightmap]
enabled = true
density = 16
page_size = 1024
padding = 4
max_pages = 1
max_texels = 1048576
light = "court-light"
shadow_size = 512
```

These numeric values are the defaults. Omit both `light` and `lights` to build
coordinates only. Set it to an ID in this level's world YAML `lighting.lights`
list to request a `point-visibility@1` startup bake for that specific authored
point light. This does not freeze the light or choose the first authored light
implicitly. Authored lighting continues to require its own existing
`world/lighting@1` capability. Source YAML and compiled world versions are
unchanged by the new build table.

Each level owns its light definitions, ambient settings and bake recipe. Put
`lighting` in the world source selected by that level's `[world].source`, and
`[lightmap]` in its `level.toml`. Compiled lighting is embedded in that level's
KLD `@world/main` entry; light IDs are local to the level. There is no global
lighting configuration in `karty.toml` or a project-root `lighting.yaml`.

For directional static direct diffuse from several authored lights, replace the
`light` line with:

```toml
lights = ["brazier-north", "brazier-south"]
```

This selects the explicitly versioned `direct-rnm3@1` encoding. `light` and
`lights` are mutually exclusive. Candidate SDK 0.0.11 accepts up to 4,096
unique authored IDs, preserves selection order, and rejects an explicitly empty list. `lights = []` and
`light = ""` are invalid. Every selected light must exist and be inside a sector
volume. The complete list is validated before publishing any level artifact.

Density must be finite and positive, at most 64 texels per world unit. Page size
is 512 or 1024, padding is 1–16, `max_pages` must be 1, and `max_texels` must
cover the selected page while remaining at most 1,048,576. Shadow size is
32–512. Explicit zero numeric values are invalid. Oversized receiver charts and
exhausted page budgets stop the build with a diagnostic; the builder does not
reduce density silently. A disabled or absent table emits no layout.

The builder generates orthonormal planar coordinates for sloped caps and opaque
wall spans, with independent bindings for every solid side. Coplanar floor and
ceiling charts merge across ordinary reciprocal connections. Unique lightmap
UVs remain separate from repeating material UVs. Chart gutters and a minimum
two-texel span protect narrow receivers such as the Roman basin steps.

The KLD includes canonical `@world/lightmaps/layout` data and the
`kartyWorldLightmaps` metadata marker. Its cartridge requires
`world/lightmaps@1`, so an SDK that does not advertise the capability fails
before processing textures. Earlier SDK contracts remain unchanged. The geometry digest
lets the host reject stale layouts after geometry changes while reusing them
when camera, actor, material or light state changes.

The current host consumes one ordinary physical component and one
page. A runtime recipe fails at build time if sectors remain disconnected
through ordinary reciprocal portals, any selected light is outside the sector
volumes, a selected light has motion, or a solid has no conservative XY receiver
association. Transformed/one-way physical connections are rejected. Layout-only
builds permit those cases. The visibility encoding stores direct static shadow data. Direct RNM stores
three horizontal RGBM coefficient tiles per logical page, preserving runtime
normal-map response with a Source-inspired directional approximation. Its
range derives automatically from selected linear light colours. A 1024 logical
page retains a 3072×1024 RGBA8 image (12 MiB), before scratch or other cached
recipes; `max_texels` bounds logical coordinates rather than all GPU memory.
After a completed offline bake is validated, its selected sources are removed
from the runtime point-light table. Their diffuse contribution comes from the
coefficient atlas; source count does not increase per-pixel light iterations.
Dynamic and unselected lights remain on the bounded live path. Ambient cube and material AO remain live and applied once.
Changing selected colour, position or radius rebakes coefficients without
changing charts. Runtime recipes supply direct lighting. Dynamic actor shadows
and transformed-portal light transport remain outside this implementation.
See the
[public format contract](https://github.com/karty-game/karty-sdk/blob/main/format/worldlightmap/README.md)
for exact identities, bounds and versioning.

## Import a completed prebake

SDK 0.0.8 additionally advertises `world/lightmaps-prebaked@1`.

`karty bake` checks this advertised capability rather than requiring that exact
SDK version. Later SDKs that retain it can use the same offline bake workflow.
Export a completed direct RNM atlas using a host preview/tool, then add both
explicit paths to the same `[lightmap]` table:

```toml
prebake = ".karty/imports/roman.prebake.bin"
prebake_image = ".karty/imports/roman.prebake.qoi"
```

These controls require an enabled direct recipe using `lights = [...]`. Both
paths must be nonempty, relative to the directory containing `level.toml`, and
confined there even through symlinks. Neither is inferred from the other.
Omitting both packages no completed prebake. The current forward host uses live
lighting in that case and has no runtime lightmap baker. Providing only one, using a visibility
recipe, or selecting an SDK without the new capability fails before publishing
artifacts.

The builder freshly compiles the world/layout and checks the exact canonical
prebake manifest, selected light IDs/order/position/colour/radius, chart/layout
identity, derived RGBM range, image digest/size and complete bounded QOI stream.
Changing those baked inputs makes the imported artifact stale and fails the
build; ambient, material AO, actors and unselected lights remain runtime terms.
The image must be straight linear RGBA RGBM with three horizontal coefficient
tiles. The CLI packages two fixed data entries and their metadata marker;
there are no author paths or external URLs in the level wire format. A matching
host loads the completed image and submits no startup bake stages.

The separately versioned manifest is `karty.world-lightmap-prebake@1` with
algorithm 1 and encoding `direct-rnm3@1`. Legacy layout/recipe bytes are unchanged.
The manifest is bounded to 16 KiB and encoded QOI to 8 MiB; the total existing
KLD envelope remains bounded to 16 MiB. QOI that cannot fit fails explicitly;
limits are never raised silently. A logical 1024 page still costs 12 MiB of
raw coefficient pixels. This is reuse of a completed bake, not radiosity or a
new quality setting. The public CLI needs no private baker source to import it.

## Bake offline with radiosity

SDK 0.0.8 adds a deterministic public CPU baker. From
the project directory, run:

```sh
karty bake
karty bake --level showcase --samples 64 --bounces 2 --workers 8
```

This compiles authored world YAML, builds its lightmap layout, traces direct
visibility and diffuse light bounces, and writes a completed directional RGBM
atlas. It does not compile a cartridge, install an SDK bundle, run Materialize
or require a host, GPU, browser or private engine source. Selected static lights
and original decoded albedo textures are the transport inputs. Existing normal
maps continue to affect the directional coefficients at runtime.

To package the generated output automatically, use:

```toml
[lightmap]
enabled = true
offline = true
# Omit light/lights to bake every eligible steady source automatically.
# Optional persisted quality expectations; otherwise command options are kept.
bake_samples = 16
bake_bounces = 1
# Optional finite transport segment length in world units; absent is unbounded.
# bake_max_ray_distance = 16
```

`offline` is mutually exclusive with explicit `prebake`/`prebake_image` paths
and the legacy single `light` visibility recipe. With candidate SDK 0.0.11,
omitting `lights` automatically selects every fixed source without a nonzero
linked emission pulse, in authored order. Moving and pulsing sources remain
live. An explicit `lights` list remains available for a partial bake; no light
is inferred for layout-only builds without `offline`. A world with only dynamic
sources gets an empty, black static coefficient atlas.

The world can author up to 4,096 sources, while the runtime point-light table
remains capped at 50. Levels exceeding that live bound need a complete matching
bake before packaging or previewing; an unavailable bake reports an error
instead of dropping sources. Static source count changes offline work and
identity data, not the three coefficient tiles or their chosen resolution.
See [complete static lighting bake v1](decisions/static-light-bake-v1.md).

Defaults are 16 hemisphere samples, one diffuse bounce and the available CPU
worker count. Samples are 1–256; bounces are 0–4. Zero bounces requests direct
lighting only. Workers affect throughput, not pixels. The initial deterministic
seed is 1. `--level` accepts a level directory or its logical name; omitting it
bakes every enabled directional recipe.

`bake_max_ray_distance` is optional, finite and positive, at most 1,000,000
world units. It limits each direct visibility or diffuse bounce segment,
so distant lighting transport is deliberately truncated. Omitting it preserves
the existing unbounded producer and output. A positive value selects the
version-3 offline producer and binds the exact distance into the binary manifest;
automatic reuse and explicit imports must match the configured value. This
setting applies equally to `karty bake` and offline chunk builds. See
[bounded incremental chunk bake v1](decisions/bounded-incremental-bake-v1.md).

The command writes generated files under the **project root's**
`.karty/bakes/<level-directory>/`: content-addressed `lightmap-<sha256>.qoi`
images and an atomically published `lightmap-prebake.bin`. For example:

```text
karty.toml
levels/showcase/level.toml       # [world] source and [lightmap] recipe
levels/showcase/world.yaml       # authored lighting and geometry
.karty/bakes/showcase/lightmap-prebake.bin
.karty/bakes/showcase/lightmap-<sha256>.qoi
```

The cache key is the directory under `levels/`, regardless of the logical
`[level].name`. Baking does not create output in level source or asset
directories, overwrite explicit imported prebakes, or modify manifests.
Previous generated images can remain cached. After upgrading from the old
`levels/<level-directory>/.karty/bakes/` location, rerun `karty bake`; old caches
are ignored and can be removed. See [bake storage v1](bake-storage-v1.md).

Normal `karty build` and `karty dev` package a matching offline pair. If no pair
exists, it is corrupt, or geometry, light, surface UV/material assignment,
original albedo or configured quality changed, the builder reports the fallback
and packages the existing recipe without a completed prebake. The current
forward host renders that fallback with live lighting. Ordinary builds retain
strict errors for corrupt or stale explicit imports; development previews omit
an unavailable explicit prebake. Command quality overrides differing
from explicit `bake_samples`/`bake_bounces` expectations require updating those
settings before packaging the generated pair.

Bake before starting `karty dev`, or restart the dev session after baking.
The existing dev watcher skips hidden `.karty/` directories, so publishing a
cache alone does not trigger a rebuild.

Offline output uses algorithm 2 of the same separately versioned prebake
manifest and records its producer, reflectance/surface identities, sample count,
bounce count and seed. Older algorithm 1 direct imports remain valid. The same
page, chart, QOI and cartridge budgets still apply. This first baker handles
opaque static geometry in one ordinary physical component; transformed portal
transport, emissive textures, moving shadows and indirect lighting for actors
remain future work. Increasing samples reduces stochastic noise; the spatial
denoiser described below filters indirect lighting by default.

## Offline indirect denoising

The CPU baker defaults to a pure Go spatial denoiser at medium strength.
Choose the preset in each level's `level.toml`:

```toml
[lightmap]
# Keep your enabled recipe, density and other settings here.
offline = true
bake_samples = 196
bake_bounces = 2
bake_denoise = "medium" # off, low, medium
```

Then run `karty bake --level showcase`, followed by the usual `karty dev`
or `karty build`. Baking does not hot-replace an already running cartridge.
The generated pair lives in project `.karty/bakes/showcase/`.
The updated CLI and host are required for the new denoised producer identity.

`karty bake --denoise off` or `--denoise low` overrides the level preset for
that invocation. To package an override, set the same `bake_denoise` in the
level config: a differing preset makes the cached bake stale. The command
prints filter time separately from the complete bake duration.

Only noisy indirect bounce lighting is filtered, before RGBM encoding.
Direct shadows and retro material textures remain sharp. The filter observes
chart coverage and geometry barriers, and has no runtime frame cost. Low
uses two passes and medium three; direct-only bakes skip filtering. Very
small indirect features can soften, and residual noise or chart seams can
still require more samples. Preset changes require rebaking.

## CPU bake acceleration

`karty bake` uses Go's portable SIMD triangle kernel with precomputed geometry
and early-exit shadow visibility. Go selects the hardware width and provides
emulation on unsupported processors; there is no user-facing SIMD switch or
runtime requirement for the prebuilt CLI. Sampling, denoising presets and cache
identity remain unchanged. This accelerates offline CPU baking; levels without
an available prebake use the current host's live lighting.

When building the CLI from source with Go 1.27, use the pinned root mise tasks
(they enable `GOEXPERIMENT=simd`) or set that experiment for direct Go commands.

## Bake time and weather variants

Candidate SDK 0.0.12 supports static sun/sky diffuse variants through
`world/lightmaps-environment-variants@1`. Configure the world level:

```toml
[lightmap]
enabled = true
offline = true
density = 24
page_size = 512
padding = 2
bake_samples = 4
bake_bounces = 1
bake_denoise = "medium"

[lightmap.environment]
times = [0, 9, 12, 17]
default_time = 12
weather_variants = false

[chunks]
enabled = true
```

Run the normal native or web build. It bakes one clear image for each authored
clock into content-addressed passive resources before the game runs. Every chunk
retains one shared chart layout and embeds the default clear
map. Later builds reuse matching validated pixel caches; changed geometry,
opacity, original albedo or bake settings produce new identities. There is no
runtime ray tracing or bake step. Editor previews without a completed geometry
cache retain the existing mesh-only live-lighting path and do not bake rays.

Times must be unique ascending hours in `[0,24)`, at most 32 clocks. The default
must belong to that list; its omission selects 15:00. Unknown environment options
are rejected. Environment recipes require chunks and enabled offline lightmaps,
and cannot also select point lights, import a whole-world prebake or truncate
sky rays with `bake_max_ray_distance`. Point lights and authored ambient remain
runtime contributions. Sun and sky visibility use actual opaque geometry;
translucent liquid materials, including static glass, transmit light in this
initial producer. Refraction, tint absorption and moving-object shadows are not
baked. Higher final sample counts can reduce residual sky/bounce noise.

`weather_variants` defaults to false. Cloud coverage changes a cheap brightness
multiplier while retaining the selected state's shadows and direction; it does
not fetch another map or start a bake. Set `weather_variants=true` explicitly
to produce clear/overcast pairs instead, with a maximum 16 clocks (32 images).
Existing paired catalogs remain supported and blend decoded linear RNM
coefficients for weather changes.

The existing environment selector chooses the nearest baked clock. A
matching host fetches condition resources when selected and uploads one active
map per chunk. The complete visible scene uses current live lighting until all
required maps are ready, suppressing old baked shadows during the transition.
Failed conditions retain that live fallback.
The image cache does not imply retaining all decoded conditions. A 1024 map
costs 12 MiB decoded; transitions need an explicitly bounded endpoint pair and
output scratch. Compressed bytes and build time grow with the number of clocks,
while the steady GPU texture count remains one RNM map per resident chunk.

The environment sample uses 24 texels per world unit and six receiver chunks,
with adaptive 512px or 1024px pages. Ordinary connected receiver cells preserve
the visible ground and courtyard; thin sections form the inaccessible
upper-storey architecture.
The opt-in solid ownership policy spreads decorative parts across eligible
sectors, while chart rotation and complete receiver-group compaction improve
page use. All selected sectors must share one full ordinary physical component;
transformed-only and disconnected groups remain invalid. Packing preserves the
authored density and respects the configured 1024px page limit. See
[environment lightmap variants v1](decisions/environment-lightmap-variants-v1.md).
Its four states are Night (0), Morning (9), Noon (12, default), and Evening (17).
They produce four atlases per chunk, 24 image resources total, rather than
additional weather states.
