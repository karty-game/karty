# Image and sound assets

Builds convert images to lossless QOI textures and WAV audio to lossy QOA.
Profiles/capabilities come from the project's pinned SDK, not an automatic upgrade.
Image/audio conversion needs no FFmpeg, ImageMagick, CGO or separate tools;
players need only the matching host. See [SDK installation](sdk-bundles.md).

## Add assets

Files under `assets/textures/` with a `.png`, `.jpg`, `.jpeg`, or `.webp`
extension are discovered as textures. WAV files under `assets/sounds/` are
discovered as game-scoped, one-shot sounds. A relative path becomes a dotted
logical name: `assets/sounds/ui/open.wav` becomes `ui.open`.

Use `karty.toml` entries when you need a different name, profile, or transform:

```toml
[[assets.texture]]
name = "ui.panel"
source = "assets/textures/panel.webp"
profile = "interface"

[assets.texture.transform]
max_width = 2048
max_height = 1024
filter = "smooth-lanczos3"
bit_depth = 8

[[assets.sound]]
name = "ui.open"
source = "assets/sounds/ui/open.wav"
profile = "effect"

[assets.sound.transform]
sample_rate = 24000
channels = "mono"
```

Omitted fields inherit the SDK profile. The CLI's selected SDK defines:

| Profile | Default processing |
| --- | --- |
| `sprite` | QOI, fit within 4096×4096, `nearest`, 8-bit channels |
| `interface` | QOI, fit within 4096×4096, `smooth-lanczos3`, 8-bit channels |
| `environment` | QOI, fit within 4096×4096, `smooth-lanczos3`, 8-bit channels |
| `effect` | QOA at 48000 Hz, preserve mono or stereo |
| `music` | Streaming QOA at 48000 Hz, preserve mono or stereo |
| `environment` (audio) | Streaming QOA at 48000 Hz, preserve mono or stereo |

Image resizing preserves the aspect ratio and never upscales. `max_width` and
`max_height` replace the inherited bounds when set. The only filters are
`nearest` and `smooth-lanczos3`; QOI output always uses 8-bit straight-alpha
RGBA. A 16-bit PNG therefore loses channel precision. Animated PNG and WebP
are rejected. QOI preserves the normalized pixels, but it is not guaranteed to
be smaller than the source PNG or WebP.

Project `[assets.texture-profiles.*]` tables and `filter = "smooth"` are unsupported.
Use an SDK profile and sparse overrides. Resizing can change UI intrinsic sizes
or invalidate pixel-coordinate sprite regions; no automatic remapping is applied.

The WAV importer accepts little-endian RIFF/WAVE mono or stereo integer PCM at
8, 16, 24, or 32 bits and IEEE float32. Matching PCM and float extensible
headers are supported. Compressed WAV, RF64/RIFX, unsupported extensible
formats, and non-finite float samples are rejected. Valid target rates are
`22050`, `24000`, `44100`, and `48000`; `channels` is `preserve` or `mono`.
Stereo is downmixed only when `mono` is explicit. QOA always normalizes to
PCM16 and has a fixed encoder policy without a quality setting.

Sources are limited to 64 MiB. An image source is limited to 16384 pixels per
dimension and 64 million pixels; processed textures are limited to 8192 pixels
per dimension, 16 million pixels, and 64 MiB decoded RGBA. The host limits all
textures to 256 MiB decoded memory. Sounds are limited to 30 seconds and 6 MiB
of decoded PCM, with a 64 MiB decoded catalog limit. Music/environment stream
metadata allows up to four hours; the 64 MiB WAV source limit still applies.
Seeking and level-scoped audio remain outside the pipeline.

## Play a sound

A build sorts logical sound names and writes nonzero `engine.SoundID` constants
to `.karty/assets/sounds.go`. For a sound named `ui.open`, use the generated
constant from your embedded `engine.Game`:

```go
func (game *Game) openMenu() {
    game.PlaySound(assets.SoundUiOpen)
}
```

Use the generated spelling rather than guessing how punctuation is converted.
IDs are local to one build and must not be stored in save data. Missing IDs are
ignored safely by the host. One-shots are limited to 32 simultaneous voices.

## Stream music and environment audio

Declare long-running WAV sources explicitly. They are encoded to QOA, wrapped
in a masked Karty media envelope, and staged as opaque
`content/<sha256>.kaud` sidecars rather than embedded in `game.kart`:

```toml
[[assets.music]]
name = "battle"
source = "assets/music/battle.wav"
profile = "music"

[[assets.environment]]
name = "rain"
source = "assets/environment/rain.wav"
profile = "environment"
```

Builds generate independently name-sorted `engine.MusicID` and
`engine.EnvironmentID` constants in `.karty/assets`. Music has one lane and environment audio has
two lanes; replacing a lane crossfades its previous stream:

```go
game.PlayMusic(assets.MusicBattle, engine.StreamOptions{Loop: true, CrossfadeMS: 800})
game.PlayEnvironment(engine.EnvironmentLane0, assets.EnvironmentRain,
    engine.StreamOptions{Loop: true, CrossfadeMS: 500})
```

## Cache and build output

Verified entries live under `.karty/cache/assets/v2/<cache-key>/`. Keys include
source bytes, resolved transforms and processor/importer/codec/resampler revisions,
not timestamps or logical names. Changed recipes rebuild only affected entries;
corrupt/incomplete entries rebuild automatically. Deleting `.karty/cache/assets/`
is a safe cold rebuild.

`dist/raw/asset-report.json` records recipes, cache hits, hashes, source/output
sizes, original/processed properties and decoded costs. Game `textures` and
per-level `levelTextures` retain separate costs. Unused game textures may be
stripped; `keep = true` retains them. Declared sounds are retained.

Build/dev use identical processing; dev watches supported image and WAV sources.

## Build-time world materials (SDK 0.0.8)

The released default SDK is **0.0.8**. A selected SDK must explicitly
advertise `world/material-atlas@1` and provide complete Materialize tool pins to
enable this pipeline for authored worlds. Atlas capability without tool pins is
an invalid SDK; a tool pin alone does not opt in.

The Go build/dev asset workflow packs surface materials in first-use order
(floor, ceiling, then walls per sector), resamples opaque albedo to 256×256
using nearest sampling to preserve authored retro pixels and palette,
and extrudes 16-pixel gutters. ID zero uses the contract's opaque default albedo;
nonzero IDs require valid, opaque level textures. At most 196 materials fit.
The native, checksum-pinned Materialize executable runs **once per cold atlas**,
after all tiles and borders are packed, generating OpenGL normal, height and AO
maps with seamless processing disabled. There is no Python orchestration,
PATH/global installation, source build or substitute normal-map fallback.
Executables remain in the managed `~/.karty` tool directory.

The CLI merges normal XY into RG (midpoint 127/128 becomes exact neutral 128),
height into B and AO into A. AO is data, **not opacity**; the second texture is
straight RGBA. Generated gutters are re-extruded. Opaque sRGB albedo and linear
material data are QOI-encoded and packaged as `@world/material-albedo` and
`@world/material-data` level data entries. Canonical JSON in
`@world/material-layout` maps level-local material/texture IDs to placements;
`kartyWorldMaterialAtlas` marks the versioned schema in level metadata. Runtime
loads this pair/layout directly rather than repacking it. Original level textures
remain available for sprites and UI. The optional generated `@world/material-mips`
tail carries L1–L8 albedo/data regions in one additional raw RGBA QOI; its
canonical `mips` records accompany the unchanged L0 placements. All four
reserved entries are generated-only;
authored `[[data]]` declarations cannot replace them, even in non-world levels.

Atlas cache keys include full referenced source QOI hashes, completed atlas
bytes, layout/IDs, packer/merge/mip/codec revisions, platform, tool version/revision,
archive checksum and native options. Build/dev share the same verified atomic
cache; warm entries do not invoke or install Materialize. The L0 pair and padded mip-tail allocation
are included in the level asset report and decoded allocation budget before
source decoding; the shared 256 MiB limit includes all original level textures. Missing,
malformed, transparent, oversized, 16-bit or wrong-dimension generated maps fail
the build without publishing a cache result. Native processing has a 90-second
deadline and bounded diagnostic output. Materialize needs a working GPU context;
fixture-executor unit tests do not establish actual Materialize GPU execution.

The complete mip chain has 256×256 L0 through 1×1 L8. Each material's interiors
are filtered independently, then its own shrinking gutter is extruded. Albedo
averages in linear RGB and is encoded back to sRGB. Normals average decoded 3D
unit vectors and are normalized before XY encoding; neutral `(128,128)` remains
exact. Height/AO average linearly, preserving zero AO as data. The combined tail
interleaves albedo/data cells by material in canonical shelf bands, with each
dimension at most 4096. Its linear QOI tag describes raw storage; albedo cells
still contain sRGB bytes. Runtime selects fractional LOD and blends adjacent
levels with shared relief UV/channel filtering. Older L0-only payloads remain
valid and keep their previous behavior.

Optional per-texture material controls live in `level.toml`:

```toml
[[textures]]
name = "wall"
source = "wall.png"
profile = "environment"
[textures.material_strengths]
normal = 1.0
height = 0.5
# omitted ao and rim default to 1
```

Each `normal`, `height`, `ao`, `rim` value must be finite and in [0,4]; 0 disables
that effect. Omitted channels default to 1, including an empty controls table.
Controls require an atlas-capable selected SDK and an authored world surface
using that texture; a worldless level, SDK 0.0.7 or an unused surface control
fails before material generation. They are packaged as the complete optional
`strengths` object in that material's canonical placement. Changing strengths
updates level metadata while reusing generated image bytes and the Materialize
cache. The canonical layout, including mip records and controls, remains bounded
to 32 KiB.

The runtime stores `min(254,round(value*64))` below 3.984375 and reserved byte
255 at or above that midpoint; bytes 0–254 decode on the 1/64 grid and byte
255 means exactly 4. The final representable interval is 1/32 and maximum
nearest-value error is 1/64. Defaults retain the
original metadata representation. A nondefault rim strength causes all materials'
height/rim masks in that atlas to be stored as eight-bit `height*rim/4` and scaled
back by four during lighting resolve; this trades rim-mask precision for the
bounded control without adding a sampler or changing geometric depth.

## Compiler-authored material projection (SDK 0.0.8)

World source version 5 enables `world/material-mapping@1`, including when no
`uv` controls are specified. Its default is world-space triplanar mapping at
one world unit per repeat. The selected SDK must advertise that capability;
SDK 0.0.7 rejects source v5 mapping. Source versions 1–4 retain their exact
previous mapping and packaging. Projection mapping works independently of
static lighting and generated material atlases.

```yaml
version: 5
uv:
  mode: triplanar
  scale: {x: 2, y: 2}  # world units per repeat, independent of geometry size
  offset: {x: 0, y: 0} # texture repeats
  rotation_degrees: 0
rooms:
  - id: room
    floor_uv: {mode: planar}
    ceiling_uv: {mode: planar}
    wall_uv: {mode: wrap, anchor: bottom}
    boundary:
      - id: south
        start: {x: 0, y: 0}
        end: {x: 4, y: 0}
        material: brick
        uv: {mode: planar, anchor: top, rotation_degrees: 0}
    # remaining boundary edges, floor/ceiling, materials and contents follow
    # the normal room contract
```

Each control inherits by field from root `uv` to room `floor_uv`, `ceiling_uv`
and `wall_uv`, then from `wall_uv` to each edge's `uv`. Omitted mode defaults
to `triplanar`, anchor to `world`, scale to `(1,1)`, offset to `(0,0)` and
rotation to zero. Explicit zero rotation resets an inherited rotation. Scale
and offset overrides replace their complete two-component vectors. Scale must
be finite in [0.001,1,000,000] per component; offset and rotation are finite
and bounded to ±1,000,000. The compiler applies scale first, then the positive
rotation matrix, then offset. These controls remain independent of prefab
translation, rotation and scale; geometry expansion does not stretch textures.

`planar` uses global X/Y on floors and ceilings. A planar wall uses its original
authored edge's unit tangent dotted with global X/Y for U, and world Z for V.
`wrap` applies to walls: U follows cumulative world-space distance around the
original authored boundary, starting at its first edge. Decomposed sectors and
clipped portal spans retain that phase. `bottom` means V is world Z minus the
original room floor height; `top` means original room ceiling height minus
world Z. These anchors use the room's original affine planes, including slopes,
and retain them across portal clipping. Floors and ceilings always use a world
anchor. An explicit floor/ceiling wrap or top/bottom anchor is invalid; inherited
root wrap falls back to planar on those surfaces, and a root anchor applies
only to walls.

`triplanar` uses global XY, YZ and XZ projections, with constant per-surface
weights proportional to the fourth power of the geometric normal's absolute
Z, X and Y components, respectively. Negative normals preserve the same global
coordinate signs. The host derives the corresponding material tangent basis.
All material channels use the baked projections and weights; setup belongs to
the editor/compiler rather than camera clipping or per-frame runtime inference.

Compiled world v3 carries the optional `material_mapping: {version: 1}` marker
and complete affine U/V plane records for every floor, ceiling and wall,
including generated internal boundaries. Coefficients and plane offsets must
be finite and bounded to ±10^12. Invalid final settings reject the entire build.
The cartridge records `world/material-mapping@1` alongside `world/sectors@1`;
the marker and all per-surface records must be present together. Geometry,
geometric depth, authored identities and portal connectivity remain unchanged.

## Authored static world lighting (SDK 0.0.8)

World source version 4 may add a top-level `lighting` payload. The selected SDK
must advertise `world/lighting@1`; SDK 0.0.7 rejects authored lighting. Lighting
and material atlas capabilities are independent, so a lighting-capable SDK can
use ordinary surface textures without invoking Materialize.

```yaml
version: 4
lighting:
  version: 1
  ambient: {x: 0.1, y: 0.1, z: 0.1}
  lights:
    - id: hall-lamp
      position: {x: 2, y: 2, z: 3}
      color: {x: 1, y: 0.8, z: 0.6}
      radius: 8
# rooms, prefabs and instances follow the normal world source contract
```

Ambient and light colors are linear RGB intensities in [0,1]. At most 50 lights
are allowed, with unique nonempty IDs and finite radii of at least 0.001.
Positions and radii stay within the world coordinate bound (1,000,000). Lights retain their
authored order and global positions; prefab transforms do not move, scale or
duplicate them. Prefab-local lights are unsupported. The complete payload is
validated before packaging; malformed lights fail the build.

Compiled worlds retain this optional payload in `@world/main`. Lighting adds
`world/lighting@1` alongside `world/sectors@1` to the cartridge's required
features. An ambient-only payload still requires lighting capability. Omit
`lighting` to retain the existing rendering and packaging behavior, including
source versions 1–3. Static lighting uses squared Half-Lambert diffuse with
bounded radius falloff. With a material atlas, generated normals affect diffuse
lighting and AO scales only ambient light, once in linear RGB; direct light
remains unoccluded. The lit albedo diagnostic shows albedo without baked AO.
Normal and depth diagnostics retain their meanings; material height never
changes geometric depth, silhouettes, sprite placement or UI.

Generated height represents positive raised relief; zero preserves the original
UVs. Authored lighting enables
two fixed-point parallax shifts using bilinear height samples, limited to
0.025 of a texture repeat per UV component (6.4 texels in a 256×256 tile).
The shift fades at grazing angles, and albedo, normal, height and AO use the
same shifted UVs. A subtle stylized rim uses height and AO masks, a fourth-power
view Fresnel term and only actual front-facing direct lights, with coefficient
0.08; it is not an ambient glow or a PBR/specular material parameter.

Worlds without authored lighting retain their existing AO-baked unlit output
and do not use height parallax or rim lighting. Shadows and portal light
transport remain unsupported. These rendering changes add no wire fields,
material-atlas schema or SDK manifest version.

## MPEG-1 video

Declare already-encoded MPEG-PS files explicitly:

```toml
[[assets.video]]
name = "demo.pattern"
source = "assets/videos/test-pattern.mpg"
```

The CLI generates `assets.VideoDemoPattern`. Play or restart it with
`game.PlayVideo(assets.VideoDemoPattern, x, y, width, height)` and stop with
`game.StopVideo()`. Coordinates describe the top-left and size of a display
rectangle. One video plays at a time, fitted without distortion above world
entities and below UI. The final frame stays visible. No completion event,
seeking, pause, or looping API is provided yet. Host logs report failures.

Source files must be confined project `.mpg` files containing MPEG-1 video
(optionally MP2 audio), at most 1280×720 and 64 MiB each. Up to 64 videos may
be declared. No video transcoding or automatic discovery is performed.
The header must be in the first 128 KiB. MPEG-2 and MP4/H.264 are unsupported.

Video stays in masked `content/<sha256>.kvid` sidecars; only the catalog is embedded.
Ship `content/` for native and web builds. The host streams bounded, verified
128 KiB chunks from disk/HTTP, not Brotli level data. Cartridges supply typed IDs,
never arbitrary paths or URLs.
Browser audio starts after user interaction. Slow downloads can stall video
without blocking the game. Audio/video synchronization is approximate.

For example, encode a clip before building:

```sh
ffmpeg -i input.mp4 -vf 'scale=640:-2' -r 25 -c:v mpeg1video -q:v 5 \
  -c:a mp2 -ar 48000 -ac 2 -f mpeg output.mpg
```
