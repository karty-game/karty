# WebGPU material charts

A validated WebGPU host bundle selects an offline material profile. Ordinary
single-projection surfaces keep their authored mapping. Exact one-hot triplanar
mappings reduce to their selected affine projection without changing sampling.
Blended mappings, secondary color layers and masked wall bands are rasterized
into ordinary material tiles before lighting and chunk preparation. Sloped
static-solid and reusable-prefab caps use the same conversion. Portal links,
sector planes, solid footprints, collision and actor attachment remain CPU data.

Each finite surface footprint normally receives a 120 by 120 pixel chart, with
four independent charts packed in one 256 pixel material tile. Four pixels of
extruded gutter separate each chart from its neighbors. If any encoded atlas
entry exceeds the existing 8 MiB level-entry bound, the profile restarts from
the original compiled document and uses sixteen 56 by 56 pixel charts per tile.
Original material generation is reused. Atlas-entry bounds are checked before
lighting. After sprite-atlas, character, UI, actions, world, chunk-catalog and
optional saved-reflection packaging, the build also checks the exact complete
16 MiB level envelope, including metadata and entry tables. If that completed
package still overflows, a bounded rebuild selects denser charts, with sixty-four
24 by 24 pixel charts per tile as the last fallback. Unchanged cached inputs are
reused; the final package must satisfy all original bounds or fail explicitly.
The generated `kartyWebGPUChartGrid` metadata records the selected grid.

This is a bounded texture approximation. Large footprints and the denser
fallback can visibly reduce detail. Filtering and low mip levels also limit
small features; these outputs need visual review at the intended game scale.
Chart interiors use nearest periodic source samples, linear-light layer
compositing, tangent-frame normal conversion and independent gutter extrusion.
Normal/AO/height gains are baked once into generated chart pixels. Existing
material-data and reflectance channel meanings remain unchanged.

Animated composite UVs and emitting composite surfaces require separate baking
policies and currently fail explicitly. Original material images that were replaced by charts are omitted from the
standalone/sprite atlas unless actors, animations, particles, vegetation or UI
still consume them. This removes duplicate payloads without discarding a known
runtime consumer. A genuinely blended solid side mapping
requires per-edge render geometry; the authored solid compiler currently allows
planar side mappings. The profile does not silently discard unsupported layers.
Its private producer revision participates in chunk cache identity, keeping
legacy material outputs and cache keys separate from WebGPU chart outputs.

## Optional self-shadowed detail

SDK 0.1.0 adds `world/material-ssbump@1`. When both the SDK capability
and WebGPU profile are selected, builds automatically generate a separate
self-shadowed RNM companion from each original material's height channel.
Generation runs before chart packing; unrelated chart cells never shadow each
other. Height bytes, bounded generation controls and algorithm version identify
the per-material cache. Source normal, height, AO and reflectance meanings stay
unchanged, and models and terrain keep their ordinary normal path initially.

Chart projection rotates or mirrors each directional coefficient field into
the destination tangent frame. Bands composite coefficient energy with their
authored alpha, while secondary color layers leave occlusion alone. Mips average
coefficients linearly without renormalization. Descriptor binding, per-level
metadata and the cartridge feature make this an explicit optional resource.
Encoded entry and shared decoded-texture budgets are checked before lighting;
the companion participates in adaptive chart density selection. SDKs without
the capability retain the same chart outputs and cache identity.

## Prepared GPU materials

SDK 0.1.0 advertises `world/material-gpu@1` and selects a complete prepared
material path for the WebGPU profile. Every world material receives SSBump;
constant height or zero relief produces the flat response. Materialize and
height tracing remain build work. Each original height tile is processed before
chart packing, then its directional coefficients are transferred into the
destination tangent basis.

The final material ID order becomes the layer order of three 256 by 256 texture
arrays, each with all nine mip levels:

| Array  | RGB                         | Alpha                             |
| ------ | --------------------------- | --------------------------------- |
| Albedo | sRGB surface color          | Linear opacity or cutout coverage |
| SSBump | Linear directional response | Linear emission mask              |
| Detail | Linear normal XY, metallic  | Linear smoothness                 |

Normal strength is resolved before mip generation. Color mips average in linear
light with coverage weighting, normals average as decoded vectors, and
metallic/smoothness average as scalars. SSBump coefficients and emission masks
average linearly without normalization. Current scalar-emission authoring uses
a full emission mask; intensity still comes from the world material setting.
Height and scalar AO are intermediate build data and are absent from these
runtime arrays.

The builder supplies the final layers and mip levels to the Khronos KTX encoder,
using UASTC 4x4 and Zstandard compression. It does not select normal-map presets,
premultiply data channels, or ask the encoder to generate generic mipmaps.
The finished level contains the canonical material layout, a descriptor binding
its digest and complete material order, and the three KTX2 arrays. Legacy QOI
material images and the optional QOI SSBump companion are omitted. Other texture
classes and published SDK artifacts keep their existing contracts.

The CLI installs the SDK-pinned official Khronos KTX-Software release for the
developer's platform, verifying its archive checksum before unpacking it. Karty
does not rebuild or republish the encoder. Encoder development can select an
explicit `KARTY_KTX=/absolute/path/to/ktx` executable; the CLI never searches PATH
or builds a tool while building a game. The encoder cache binds prepared bytes,
array dimensions, color space, settings, the selected executable and libktx
library hashes from both its executable directory and neighboring `lib` directory.
Encoder subprocesses discard inherited `LD_*` and `DYLD_*` loader overrides so
they cannot select libraries outside that recorded layout.
Managed tools also bind the upstream version, source
revision and selected platform archive checksum. Library aliases must resolve
inside the real library directory, with bounded hashing in deterministic order.
KTX-Software 4.4.2 is the qualified encoder, with upstream source commit
`4d6fc70eaf62ad0558e63e8d97eb9766118327a6` recorded in SDK tool metadata.

The official Windows encoder requires the Microsoft Visual C++ v14 runtime
(`MSVCP140.dll` and `VCRUNTIME140.dll`) and the Universal CRT. These are system
prerequisites; the official KTX package does not embed those runtime DLLs.
Install the system components before using the encoder. Karty's tool installer
extracts the verified package files without running an administrator installer.

At runtime, hosts validate the complete resource set, decompress Zstandard,
transcode UASTC into an adapter-supported GPU format and upload the prepared
layers and mips. Transcoding and upload still require load-time work; this does
not establish mobile startup timing or sustained device FPS.

## Portal-selected GPU cells

SDKs advertising `world/gpu-chunks@2` select build-prepared spatial KLDs for
WebGPU worlds. Each render cell owns its final vertex, index, address and material
record buffers, draw bounds and source ownership, plus one lossless linear
RGBA8+Zstd KTX2 with exactly three local RNM layers. Channel packing, chart
placement, constants, tangents and geometry conversion stay in the build. The
runtime verifies and uploads these immutable payloads independently.

The static `karty.webgpu-cell@2` vertex is 96 bytes: position, normal, tangent,
material UV, lightmap UV and basis, and the existing motion parameters. Retained
attributes keep their float32 precision. Joint indices, weights and per-vertex
RNM values belong to other rendering paths and are absent from static cell
vertices. Catalog and payload wire version 2 and `karty.world-gpu-chunks@2`
metadata identify this contract even though the KLD entry names stay unchanged.
The CLI regenerates cell buffers through the selected SDK; it does not reinterpret
old cell bytes or add a runtime expansion step. Install the matching host and SDK
together. Earlier published SDKs and hosts retain their immutable contracts.

Indoor chunks preserve whole authored rooms within a proven identity-portal
space. Optional build-only `room_groups` join connected room fragments into one
loading area. Their enclosing grid bounds do not cut geometry. Reciprocal
coincident portal edges can share that coordinate space; transformed or directed
links do not establish coordinate adjacency. Catalog cells retain explicit sector
and physical-owner references. Portal traversal selects cells and lookahead;
neighboring grid coordinates do not imply reachability.

Liquid materials preserve the existing renderer's 64-pixel sine normal field,
expanded into the shared detail RG layer before semantic mip generation. Their
authored opacity, flow, wave phase and displacement, water Fresnel and specular
parameters are packed offline. Loading creates no procedural normal image.

The optional `[chunks.gpu]` settings select `cell_size` (default 16 metres) and
`liquid_edge_step`. For indoor rooms, `cell_size` defines enclosing catalog grid
coordinates, not cutting planes. For example, `room_groups = [["hall", "hall-stairs"],
["workshop", "workshop-recess"]]` keeps each listed area together; unlisted rooms
remain independent. For this indoor profile, room ownership also replaces
`max_sectors` partitioning for physical payloads; legacy and explicit spatial
profiles retain their sector-count policy. Unknown, duplicate or disconnected
groups fail the build.
Each group must fit the authored lightmap page limit at unchanged density and
padding; exceeding capacity fails the build instead of silently cutting the room.
Moving actors remain separate from static cells. Fixed worlds
retain collision and probe metadata in physical-only KLDs; live-only worlds keep
their existing physics ownership and use a neutral one-pixel RNM per cell.
Completed prebake recipes preserve their selected lights and bake controls;
their existing provenance format retains a minimum 512-pixel page. The catalog
records the baked light indexes before any cell loads.
Environment-variant lighting and runtime-generated lightmap recipes require a
separate prepared contract and are rejected by this profile.

Existing `[chunks.spatial]` recipes retain their authored fine/background density,
geometry limits and loading-unit grouping. Ordinary surfaces and those explicitly
selected by that recipe use separate quality groups; grid span records larger
background cells without merging unrelated portal spaces. Shared material arrays
remain referenced root resources. Per-cell RNM is reported as streamed texture
storage rather than charged as a whole-world preload; total decoded estimates
remain visible, with per-resource and shared-residency bounds still enforced.
