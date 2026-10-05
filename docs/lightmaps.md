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
coordinates only. Set it to an ID in the YAML root's existing `lighting.lights`
list to request a `point-visibility@1` startup bake for that specific authored
point light. This does not freeze the light or choose the first authored light
implicitly. Authored lighting continues to require its own existing
`world/lighting@1` capability. Source YAML and compiled world versions are
unchanged by the new build table.

For directional static direct diffuse from several authored lights, replace the
`light` line with:

```toml
lights = ["brazier-north", "brazier-south"]
```

This selects the explicitly versioned `direct-rnm3@1` encoding. `light` and
`lights` are mutually exclusive. The list contains one to eight unique authored
IDs, preserves selection order, and cannot be empty. `lights = []` and
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
before processing textures. Released SDKs are unchanged. The geometry digest
lets the host reject stale layouts after geometry changes while reusing them
when camera, actor, material or light state changes.

The first host implementation consumes one ordinary physical component and one
page. A runtime recipe fails at build time if sectors remain disconnected
through ordinary reciprocal portals, any selected light is outside the sector
volumes, or a solid has no conservative XY receiver association. Layout-only
builds permit those cases. The visibility encoding stores direct static shadow data. Direct RNM stores
three horizontal RGBM coefficient tiles per logical page, preserving runtime
normal-map response with a Source-inspired directional approximation. Its
range derives automatically from selected linear light colours. A 1024 logical
page retains a 3072×1024 RGBA8 image (12 MiB), before scratch or other cached
recipes; `max_texels` bounds logical coordinates rather than all GPU memory.
After complete publication, selected environment diffuse is removed
from runtime lighting. Front-facing rim remains live. Those lights still illuminate
actors at runtime. Ambient cube and material AO remain live and applied once.
Changing selected colour, position or radius rebakes coefficients without
changing charts. Runtime recipes supply direct lighting. Dynamic actor shadows
and transformed-portal light transport remain outside this implementation.
See the
[public format contract](https://github.com/karty-game/karty-sdk/blob/main/format/worldlightmap/README.md)
for exact identities, bounds and versioning.

## Import a completed prebake

SDK 0.0.8 additionally advertises `world/lightmaps-prebaked@1`.
Export a completed direct RNM atlas using a host preview/tool, then add both
explicit paths to the same `[lightmap]` table:

```toml
prebake = "bakes/roman.prebake.json"
prebake_image = "bakes/roman.prebake.qoi"
```

These controls require an enabled direct recipe using `lights = [...]`. Both
paths must be nonempty, relative to the directory containing `level.toml`, and
confined there even through symlinks. Neither is inferred from the other.
Omitting both retains runtime baking. Providing only one, using a visibility
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
lights = ["brazier-north", "brazier-south"]
offline = true
# Optional persisted quality expectations; otherwise command options are kept.
bake_samples = 16
bake_bounces = 1
```

`offline` is mutually exclusive with explicit `prebake`/`prebake_image` paths.
Defaults are 16 hemisphere samples, one diffuse bounce and the available CPU
worker count. Samples are 1–256; bounces are 0–4. Zero bounces requests direct
lighting only. Workers affect throughput, not pixels. The initial deterministic
seed is 1. `--level` accepts a level directory or its logical name; omitting it
bakes every enabled directional recipe.

The command writes only generated files below each level's `.karty/bakes/`:
content-addressed `lightmap-<sha256>.qoi` images and an atomically published
`lightmap-prebake.json`. It does not overwrite explicit authored prebakes or
modify the level manifest. Previous generated images can remain cached.

Normal `karty build` and `karty dev` package a matching offline pair. If no pair
exists, it is corrupt, or geometry, light, surface UV/material assignment,
original albedo or configured quality changed, the builder reports the fallback
and packages the existing runtime bake recipe. Explicit manual imports retain
strict errors for corrupt or stale data. Command quality overrides differing
from explicit `bake_samples`/`bake_bounces` expectations require updating those
settings before packaging the generated pair.

Offline output uses algorithm 2 of the same separately versioned prebake
manifest and records its producer, reflectance/surface identities, sample count,
bounce count and seed. Older algorithm 1 direct imports remain valid. The same
page, chart, QOI and cartridge budgets still apply. This first baker handles
opaque static geometry in one ordinary physical component; transformed portal
transport, emissive textures, moving shadows and indirect actor probes remain
future work. Increasing samples reduces stochastic noise; this first slice
does not include an automatic denoiser.
