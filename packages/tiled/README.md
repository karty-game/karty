# Tiled maps for Karty 2D

A portable Go package for finite orthogonal Tiled JSON maps, reusable image
importing, compiled binary maps and ordinary Karty 2D sprite presentation. The module is
`github.com/karty-game/tiled`; the local package identity is `tiled`, version
`0.1.0`. It contains no generated `.karty` bindings or private engine imports.

From a game, import the working package with ordinary Karty package commands:

```sh
karty package import ../../packages/tiled
```

Import Go code using its ordinary module path. Folder links support development;
exported `.kpkg` archives contain the source, license and tests. There is no
special registry, install hook or engine version override.

## Compile maps during builds

Select the processor in your game's `karty.toml`:

```toml
[[build.processors]]
use = "tiled.compile"
inputs = ["maps/town.tmj", "art/tiles.tsj"]
```

Declare every external tileset needed by the maps. The processor reads only
those confined metadata inputs and emits `maps/town.bin`. Runtime code reads
it with `processed.Read("tiled.compile", "maps/town.bin")`, then calls
`tiled.Decode(data)`. Keep editable JSON out of cartridge embeds.

## Parse and author assets

Build tools import `github.com/karty-game/tiled/source`:

```go
level, err := source.Load(projectFS, "maps/town.tmj")
```

`projectFS` is an `fs.FS` whose root contains the map, external `.tsj` tilesets
and referenced PNG files. Paths resolve relative to their owning map/tileset,
with portable slash separators. Escaping the FS root, absolute paths, drive
prefixes, URLs and backslashes are rejected. The caller must provide a confined
FS when physical filesystem or symlink confinement matters; `os.DirFS` alone
does not prevent following symlinks outside its root. Loading never writes files.

The parsed `Map` includes placed `Sprites`, raw authored `Objects`, named layer
order and the source document/properties. It supports image collections and
atlas tilesets, margins/spacing, nested group offsets/visibility/opacity, tile
object bottom/bottom-left alignment, rotation, horizontal/vertical/diagonal
flips, object index/top-down ordering and millisecond tile animations. Hidden
non-image object layers remain available as gameplay metadata. Collision rules
and responses belong to the game; the parser does not instantiate physics.
Tile collision object groups remain in tileset metadata.

`ImageKey(sprite)` identifies a source path and source rectangle. `Transform`
returns the image scale and clockwise rotation, and `Center` returns the
matching world position. Use both helpers together: diagonal flags exchange the
scaled image dimensions and shift its center to preserve Tiled's bottom-left
placement, including authored object rotation. Tileset offsets scale with the
image before rotation. The parser's `Sprite.X`/`Y` describe the nominal placement,
so `X + Width/2` is insufficient for a diagonal non-square image. `FrameAt` selects the authored animation frame using an absolute
millisecond clock. Custom properties retain explicit scalar values, with convenience
`Number` and `String` accessors. Compiling rejects nested/class values.

The `assets` subpackage resolves all base/animation images, decodes PNGs once,
validates their rectangles and emits independent zero-origin crops through a
consumer-supplied callback. See [the importer reference](assets/README.md).
An ordinary game-owned author generator writes these images and its usual
`level.toml`, then assigns compiled texture IDs and records
`ImageKey → texture ID` aliases. The importer never rewrites authored Tiled files.
Karty processors currently publish embedded `.karty/processed` data; importing
this code package does not automatically register PNGs as level textures.

The initial subset intentionally rejects infinite maps/chunks, non-orthogonal
orientation, render orders other than `right-down`, encoded/compressed tile
data, image layers, external object templates, unsupported object alignments,
and rotated/ellipse/polyline non-image geometry. Those cases return errors
instead of approximated rendering. Unsupported map features require explicit
future support rather than silently changing the editor result.

Input limits are 4 MiB per JSON document, 16 MiB total JSON and 65 documents;
256×256 map/layer cells, 256 layers with 16 levels of nesting; 65,536 flattened
sprites, objects and polygon points; 64 frames per animation and 262,144 placed
animation frames. Coordinates are finite within ±1,000,000 and declared image
dimensions are at most 16,384. JSON reads are bounded before decoding. PNG
decoded-memory and crop checks have their own importer limits.

## Present with a game-owned Sprite2D backend

`github.com/karty-game/tiled/karty2d` depends only on this portable module. The
game implements this small backend using its own generated SDK:

```go
type Backend interface {
    Create(karty2d.SpriteState) (karty2d.Handle, error)
    Update(karty2d.Handle, karty2d.SpriteState) error
    Destroy(karty2d.Handle) error
}
```

`SpriteState` contains `AssetID uint32`, centered `X`, `Y`, `ScaleX`, `ScaleY`,
clockwise-radian `Rotation`, `Opacity` and `Layer int32`. Handles are opaque
`uint64` values; creation must return a unique nonzero live handle. A game
adapter owns its `EntitySprite2D` pointers and level handle, turns `AssetID`
into its `engine.LevelTexture`, and updates changed state with ordinary sprite
setters. Each failed backend operation must leave that operation's old state
intact. Previously successful operations are retained if a later call fails.

```go
view, err := karty2d.New(level, backend, func(key string) (uint32, bool) {
    id, ok := textureIDs[key] // Produced by the game's ordinary author generator.
    return id, ok
}, karty2d.Options{Wind: true, MaxLiveSprites: engine.MaxEntityRefs - 64})

ready, err := view.Update(karty2d.Bounds{
    MinX: left, MinY: top, MaxX: right, MaxY: bottom,
}, elapsedMilliseconds)
```

`New` validates every base image, animation frame and layer before making any
backend calls. Keep the map and its animation timing immutable while the view
exists. Bounds are world coordinates, usually including overscan around the
camera's visible region. Time is **absolute elapsed milliseconds**, not a
delta; every animated tile follows the same clock even after culling and
recreation. A 60 Hz game can pass `int64(frameNumber*1000/60)`.

`Update` returns whether residency for the current bounds has caught up. It
retains its rolling scan cursor if the camera moves every frame, so distant
entries cannot starve behind repeated prefix rebuilding. It uses full rotated
image bounds, including every animation frame and optional wind margin. Static
sprites do not receive state updates when the camera moves; only residency or
changed animation/effect states trigger backend calls.

Offscreen sprites are retired before replacements are admitted, within the
same lifecycle budget. Set `MaxLiveSprites` to the backend's available capacity;
the example reserves 64 SDK entity references for the hero, HUD and other game
objects. The lifecycle call budget does not limit the total live population.

Options default to 128 lifecycle calls and 256 changed animation calls per
update, with at most 8,192 live sprites. Both call budgets accept values from
1 through 256; the live cap accepts 1 through 65,536. Zero selects a default.
`ErrLiveLimit` reports an overfull visible region; use smaller bounds, a
different zoom or a deliberately larger cap. The readiness flag covers
residency, not whether every budgeted animation update has caught up. `LiveCount`
reports the current residency. Backend creation/destruction can allocate;
steady camera updates and prepared animation-state calculations do not.

`LayerFor(layer, footY)` gives the same sorting key to dynamic game actors:
`LayerBase + layer*LayerStride + trunc(footY*SortScale)`. Defaults are 0,
100,000 and 2. Choose a stride greater than the complete Y-sort span of one
layer, and keep the resulting interval inside the SDK camera's world layer
range. Overflow is rejected. Equal-footY keys retain the backend's entity
ordering, which can change after eviction/recreation; this initial API does
not add a second stable tie key. Game HUD layers stay outside the world range.

The game calls the SDK's `SetCamera2D` and current-camera `ScreenToWorld` helper
directly. This module does not create 3D worlds/cameras, dispatch input, choose
camera follow behavior or own HUD layout. Portrait-safe HUD positioning stays
with the game or KartUI. Existing SDK Camera2D support is sufficient; this
package introduces no engine API or wire changes.

Call `Close()` until it returns ready before releasing the backend's textures;
it destroys at most the residency budget per call and can retry errors.
Further updates are rejected once closing begins. Closing again is harmless.

## Optional wind extension

Wind is disabled by default. `Options.Wind` opts into `windAmplitude` (degrees,
clamped to ±2) and `windPeriod` (seconds, default 5) custom sprite properties.
`karty2d.WindTransform` rotates around the rendered bottom center so roots do
not slide. The view quantizes breeze changes to 15 Hz while preserving exact
authored animation holds. This is an optional Karty effect, not a Tiled format
requirement. Other sample-specific behavior belongs to the consuming game.

Tests cover parser/path/read limits, Tiled transforms and timing, importer
rectangles and output independence, bounded view residency/animation, rotating
full-image culling, error retry, neutral static camera updates, shared cull
phase, fixed wind roots and allocation-free steady view updates. Run the pinned
Tiled package tasks from the CLI repository root; full game builds and actual
browser/device rendering are separate integration checks.

The [compiled map v1 decision](docs/binary-map-v1.md) records the binary layout,
allocation bounds and source/runtime API boundary. The root package and
`karty2d` have no JSON dependency; `source` is for build tools.

## Static background chunks

`assets.BakeStaticLayers` optionally composes selected background tile layers
into separate transparent images of at most 1024×1024 pixels. `ChunkOptions`
selects logical chunk size and integer source-pixel density. Downsampling is
rejected; existing high-resolution tile detail survives. The callback stores
each unique image and the returned placements can replace those layers in a
render-only map. Repeated patterns and identical pixel content share images.
Chunks are processed sequentially, so a repeated map does not allocate one
large atlas. Original gameplay/collision metadata stays unchanged.

Never select an actor/Y-sorted layer: keep scenery, foreground masks, animated
tiles and wind-driven sprites separate. Objects, animation, wind, fractional
geometry, rotation and layer opacity in selected layers return errors. PNG
alpha, painter order and all orthogonal tile flips are retained.

Import's source/pixel bounds apply; output is limited to 1,024 placements and
256 MiB of unique RGBA pixels, with a separate 1 GiB cumulative raster-work bound
and bounded spatial-plan references. Chunking reduces ordinary sprite entities
by trading reusable small textures for bounded composed images; it does not add
a renderer batching API. [Willowbrook](../../samples/willowbrook/README.md) shows
authoring, generated chunk metadata, stale-placement detection and an unchanged
dynamic sprite backend.

## Build static background layers

To merge static tile backgrounds during `karty build`, add a `tiled-build.json`
input beside the map:

```json
{
  "version": 1,
  "map": "internal/village/village.tmj",
  "level": "village",
  "staticLayers": ["Ground", "Paths"],
  "chunkSize": 256,
  "pixelScale": 4
}
```

Include that file, the map/tilesets and the selected layers' original PNGs in
`build.processors.inputs`. Paths are relative to the project. Chunk size is in
world pixels; `pixelScale` retains source detail rather than downsampling.
Select only integer-aligned, opaque, nonanimated background tile layers.

The processor returns the usual `village.bin` plus `village.render.bin` with
merged backgrounds and unchanged dynamic scenery/gameplay metadata. Load the
render map for drawing and resolve its image keys through
`processed.TextureID("tiled.compile", key)` after checking authored texture IDs.
The CLI packages generated images as level textures under ignored `.karty/`
outputs. Keep only the original PNGs, map, tilesets and configuration in Git.
See the [version 1 build contract](../../website/content/design/processor-textures-v1.md)
for bounds, identity and transaction rules.
