# Compiled Tiled maps v1

Decision: 2026-10-18, for the unreleased `tiled` 0.1.0 candidate. Editable maps
remain Tiled JSON. The `source` package owns JSON parsing, document expansion,
path confinement and source validation. The root package and `karty2d` own
compiled game data and presentation; they have no JSON dependency.

Consumers explicitly select the `tiled.compile` build processor and declare
both map and external tileset inputs. Each `path/map.tmj` produces
`path/map.bin` through the existing bounded KPR1 processor transaction. Games
read that resource from `.karty/processed` and call `tiled.Decode`. Source edits
therefore rebuild compiled data through ordinary build/development watching.
PNG authoring and registration remain with the existing asset importer.

`source.Load(fs, name)` replaces the old JSON `tiled.Load` call in authoring
tools. Runtime `tiled.Load` reads compiled records; `Decode` accepts bytes and
`Encode` compiles an expanded source map. The cartridge never embeds the TMJ or
TSJ documents. This changes the unpublished package API, without an engine,
SDK, level-envelope or command-protocol change.

## KTM1 record

The 16-byte little-endian header contains `KTM1`, u16 version 1, u16 zero flags,
u32 payload byte length and u32 reserved zero. The payload contains:

1. Map width, height, tile width and tile height as u32 values, then properties.
2. A counted array of layer names.
3. A counted array of flattened sprites, each followed by its animation frames.
4. A counted array of gameplay regions, polygon points and properties.

Counts and UTF-8 string lengths use u32. Sprite image keys retain their confined
relative path and source rectangle. Numeric transforms use finite float64;
layer/object IDs and rectangle coordinates use bounded u32 values. Three flag
bits encode horizontal, vertical and diagonal flips. Frame durations use u32
milliseconds. Nested animation frames are rejected. Raw layers, tilesets and
unexpanded source documents are omitted.

Properties retain their ordered names and Tiled type labels, with explicit
value tags: null 0, false 1, true 2, float64 3 and UTF-8 string 4. Numbers retain
the existing float64 accessor semantics. Nested/class-valued properties are
rejected explicitly; they are not interpreted as an implicit JSON contract.

## Bounds and publication

Records are at most 64 MiB, with a conservative 128 MiB allocation budget.
A complete structural pass checks counts, lengths, finite values, paths and
aggregate budgets before allocating decoded slices or copied strings. A second
pass constructs the map; semantic validation completes before returning it.
Malformed headers, unsupported versions, trailing bytes and JSON are rejected.
Empty arrays have a canonical empty representation.

Existing map, layer, sprite, animation and polygon limits remain. Property lists
are limited to 256 entries, names to 1024 UTF-8 bytes, type labels to 256 and
string values to 65536. The build processor additionally retains the CLI's
256-file / 16 MiB aggregate input and output bounds. Runtime records may be
larger when produced outside that processor, within the codec's own limits.

Tests retain the official Tiled raster oracle and parser/importer bounds, and
cover compiled placement, animation, properties and collision equivalence,
complete-record rejection, dependency closures and transactional processor
output. Real Go/TinyGo consumers qualify the build boundary separately.
