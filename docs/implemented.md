# Implemented behavior

Current CLI capabilities; future work is listed in [Proposals](proposals.md).
Projects use their exact SDK pin and its generated authoring docs.

## Projects and distribution

- Scaffold Go/TinyGo games, compile KartUI and mountable levels, and generate
  project asset references without importing private engine source.
- Install signed SDK bundles into an external cache; verify cached bytes on use.
  There is no embedded SDK or fallback. See [SDK bundles](sdk-bundles.md).
- Build native and browser distributions with SDK-pinned tools and hosts.
  Dev mode serves browser builds and rebuilds source, UI, levels and assets.
- Install optional SDK-pinned released Materialize binaries with full-archive
  verification and warm-cache integrity checks through `karty toolchain install`.
  The default SDK 0.0.8 pins Materialize 2.0.0. Atlas-enabled world builds generate
  material maps; projects without world atlases do not require GPU execution.
  See [SDK bundles](sdk-bundles.md).
- Stage levels and media sidecars alongside the cartridge; ship the whole
  distribution directory. See [Distribution](distribution.md).

## Assets and media

- Convert PNG/JPEG/WebP to QOI with bounded, no-upscale resizing and 8-bit RGBA.
- Convert supported PCM/float WAV to lossy QOA with rate/channel overrides.
- Resolve SDK profiles plus sparse per-asset transforms; share verified recipe
  caches across game/level textures, sounds and audio streams.
- Report source, packaged and decoded costs; rebuild corrupt cache entries.
- Generate typed IDs for one-shots, music, environment audio and MPEG-1 video.
  Music/environment streams support host-owned loops and crossfades.
- Stage audio/video sidecars without video transcoding or automatic discovery.
  See [Assets](assets.md) for configuration, playback APIs and resource bounds.

## Worlds and samples

- Compile YAML rooms, slopes, prefab instances, actor sprites/tags and directed
  portal references into validated packaged world data.
- The [world-camera sample](../samples/world-camera/README.md) demonstrates
  perspective/isometric views, movement/orbit, tag queries and release/remount.
  Camera and portal correction execution belongs to the host.
- [Samples](../samples/README.md) cover games, KartUI, media and authored worlds.