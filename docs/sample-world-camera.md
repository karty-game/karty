# World camera contributor notes

The [sample README](../samples/world-camera/README.md) covers the experience and controls.
This page keeps the editing and validation commands separate.

## Source files

- [World YAML](../samples/world-camera/levels/showcase/world.yaml): geometry, prefabs, actors and lights.
- [Level manifest](../samples/world-camera/levels/showcase/level.toml): materials and bake settings.
- [Camera controller](../samples/world-camera/src/cameras.go): walking, turning and orbiting.
- [Actor components](../samples/world-camera/src/actors.go): accepted-pose followers and animation.
- [HUD](../samples/world-camera/src/hud.go) and [controls](../samples/world-camera/ui/views/controls.kui): camera feedback and interaction.

From the sample directory, run `../../dist/karty schema` to refresh YAML editor
completions, or `../../dist/karty schema --check` to validate without processing
images or lighting. See [world YAML schemas](level-yaml-schema-v1.md).

## Artwork and lighting

Regenerate the checked-in textures from the CLI root:

```sh
mise run generate-world-camera-materials
# Replace only the court wall, preserving other custom artwork:
mise run generate-world-camera-materials -- --only z-court-plaster
```

The level enables an offline bake with four samples, one indirect bounce and
medium denoising. From the sample directory, run:

```sh
../../dist/karty bake --level showcase
../../dist/karty dev
```

Output is cached under the project's `.karty/bakes/showcase/`. Geometry, static
lights, materials and bake quality changes invalidate it. Bake again before
starting or restarting the dev session; the watcher skips hidden cache files.
Without a matching cache, builds report a fallback to runtime lighting baking.
The moving corridor light is evaluated live and stays out of `[lightmap].lights`.
See [lightmaps](lightmaps.md) for quality settings and supported lighting behavior.

## Camera and renderer settings

Both cameras read resolution and projection settings from
[the project manifest](../samples/world-camera/karty.toml). KartUI stays at display
density. Camera render sizes are bounded to 1280×720; perspective uses vertical
FOV and isometric uses `ortho_height` in world units.

The sample opts into browser renderer controls with `[project.debug] renderer = true`.
**Lightmap** shows a snapshot of the completed atlas; **AA: On/Off** toggles SMAA
for Full output. **Inspect** shows accepted host poses, useful for reproducing
camera or portal artifacts. Include it in diagnostic screenshots.

## Focused behavior checks

From the CLI root:

```sh
mise run check-world-camera-components
mise run check-world-camera-hooks
```

These source/CPU fixtures cover typed hooks, mount/remount, camera movement,
accepted host poses and UI refresh without loading world assets, lighting bakes,
graphics or a browser. Each test process is capped at nine seconds; compilation
and SDK setup are separate. See [sample validation](sample-development.md#focused-validation).

## Optional full-scene diagnostics

Build and serve the actual web sample from the CLI root:

```sh
mise run build-world-camera-web
mise run serve-world-camera -- --addr 127.0.0.1:4242
```

For the explicit full-scene check, supply matching checker/tool artifacts and
Playwright's pinned Chromium beside the browser checker:

```sh
KARTY_HOST_NATIVE=/path/to/karty-host \
KARTY_HOST_WEB=/path/to/karty-host.wasm \
KARTY_HOST_CHECK=/path/to/karty-host-check \
KARTY_WORLD_CAMERA_WEB_CHECK=/path/to/check-world-camera-web.mjs \
KARTY_TINYGO=/path/to/sdk-pinned/tinygo \
KARTY_WASM_TOOLS=/path/to/sdk-pinned/wasm-tools \
mise run check-world-camera
```

This builds clean authored inputs twice, compares staged bytes and executes the
packaged guest/level through native Wazero and the web host. It checks cameras,
movement, actors, remounts, corrupted-level containment and primary touch actions
at three viewport sizes. Web host overrides need their matching `wasm_exec.js`.
This is an explicit graphics diagnostic; manual device review remains separate.
