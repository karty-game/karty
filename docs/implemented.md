# Implemented CLI behavior

Projects use their exact SDK pin and generated `.karty/docs`. The CLI's released
default and candidate sample selections are owned by
[internal/release/version.go](../internal/release/version.go); see
[release pins](releases.md#sdk-pins) and the
[current candidate](releases.md#sky-physics-and-particles-candidate).
The inventory below depends on the selected bundle's capabilities and matching
host. Unresolved work is listed in [proposals](proposals.md).

## Projects and distribution

- With supporting SDK bundles, derive typed action catalogs,
  per-level editor schemas and staged invocation adapters from Go declarations.
  Validate optional authored sequences and their actor targets before compilation.
  See [client hooks and actions](client-hooks.md).
- Scaffold Go/TinyGo games, compile KartUI and mountable levels, and generate
  typed project assets/configuration without importing private engine source.
- Discover and package `.kui` single-file components and layouts, with
  conservative unused-view stripping. See
  [UI sources](ui-source-discovery-v1.md).
- Install signed SDK bundles into an external cache and verify cached bytes on
  use. There is no embedded SDK fallback; see [SDK bundles](sdk-bundles.md).
- Build native/browser distributions with SDK-pinned tools and hosts. Dev mode
  serves browser builds and watches source, UI, levels, manifests and assets.
- Capture one world level through a local native host using `karty screenshot`,
  with explicit camera placement, bounded PNG dimensions and a capture timeout.
  See [native screenshots](screenshot.md).
- Resolve optional project camera defaults into typed constants; games apply
  those constants to camera settings. See [development](development.md).
- Stage levels and media sidecars alongside the cartridge. Ship the complete
  distribution directory; see [distribution](distribution.md).

## Assets and media

- Convert PNG/JPEG/WebP to QOI with bounded, no-upscale resizing and 8-bit RGBA.
- Convert supported PCM/float WAV to QOA with rate/channel overrides.
- Resolve SDK profiles and per-asset transforms through verified shared recipe
  caches; report source, packaged and decoded costs and rebuild corrupt entries.
- Generate typed IDs for one-shots, music, environment audio and MPEG-1 video.
  Stage streaming sidecars; host playback owns loops, fades and stream lifetime.
- Install SDK-pinned released Materialize binaries with archive/warm-cache
  verification. Native tool builds and releases belong to public karty-tools;
  the selected SDK owns executable pins.
- Build paired world albedo/normal/height/AO atlases, per-material L0–L8 mips
  and bounded strength metadata. Warm atlas caches avoid tool execution.
- Per-world-surface `[textures.materialize]` presets and optional generation
  overrides, with finite bounds, capability checks and unknown-key rejection.
  Materialize runs on independently normalized/cached 256×256 assets before
  atlas assembly. Automatic seamless processing has no author-facing switch.
  Existing normal/height/AO packaging and runtime strengths remain supported;
  SDKs advertising `world/material-reflectance@1` add optional metallic/smoothness
  companions for direct specular rendering in the matching host.

See [assets](assets.md) for configuration, mapping, cache identities and bounds.
Generated world materials need a supported graphics backend; levels without
atlas generation do not. Linux headless generation has an automatic software
OpenGL fallback described in [development](development.md).

## Worlds and lighting

- Package source-v9 environment presets and room/boundary sky masks behind the
  pinned `world/environment@1` SDK capability; see [environments](environments.md).

- Compile rooms, slopes, prefabs, actor sprites/tags and directed portals into
  validated world data. Bake planar/triplanar material mappings after expansion.
- Compile static extrusions and room-free detail prefabs without adding synthetic
  sectors; see [static solids](static-solids.md).
- Preserve authored ambient cubes, opt-in actor lighting and bounded periodic
  point-light motion. Rendering/collision execution belongs to the host.
- Generate independent lightmap receiver charts with explicit visibility or
  directional static-light recipes, and strictly validate completed imports.
- Automatically select every eligible steady source for offline lighting when
  selectors are omitted. Up to 4,096 sources can collapse into the same three
  coefficient tiles; the separate live table remains bounded to 50 lights.
- Run headless `karty bake` for deterministic direct lighting plus sampled
  diffuse bounces under project `.karty/bakes/<level-directory>/`. Each level's
  world source owns its lights, embedded in that level's KLD. Package matching
  automatic output or fall back to live lighting with a diagnostic when the
  automatic pair is missing/stale/corrupt and the sources fit the live bound.
  Explicit manual imports remain strict. See [lightmaps](lightmaps.md).

[Samples](../samples/README.md) cover game/UI/media and authored worlds. The
[world-camera sample](../samples/world-camera/README.md) exercises both cameras,
keyboard/touch HUDs, tags, static solids, moving lights, offline bakes and reloads.
