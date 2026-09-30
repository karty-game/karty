# World camera and actor gallery

This unlit renderer sample now targets the unreleased SDK 0.0.7 candidate for A/D keyboard input. It packages a high-level YAML world with
a concave entry hall, three sloped galleries, a 90-degree corner, and a steeper
ascending corridor. Floor planes change direction and grade along the route;
the ceiling stays level across portal seams so the isometric cutaway remains
readable. The mounted contents become host ECS actors and render through the
same `Camera3D` entity in perspective and isometric projections.

The gallery uses distinct adventurer, ghost, tree, direction-sign and rune
artwork to make camera-facing and upright billboards, crossed geometry,
a one-sided fixed wall sprite, a blended sprite, and a fixed planar decal on
the sloped floor. The first gallery's crossed sprite is discovered through its
`spinner` tag and rotated every frame with a typed ECS transform update. Gallery
instances replace the prefab actor's tags. The client also queries the authored
`interactive` tag after mounting and changes matching actors to
`interactive,verified`, exercising authored identity, tag lookup, and mutation.

- **Hold left/right:** move the FPS camera along the route in either view
- **Hold A/D:** orbit the isometric camera
- **Click/tap:** switch between FPS and isometric cameras
- **Right-click:** release and remount the packaged level

A tiny adventurer marks the FPS camera position in isometric view. It follows
the host-corrected FPS position, just behind the eye so it stays out of the FPS
view. Orbiting the isometric camera leaves this marker at the saved FPS position.

Build it with the matching engine SDK artifact:

\`\`\`sh
../../dist/karty build --target native
./dist/native/karty-host --cartridge ./dist/native/game.kart
\`\`\`

The repository integration check starts from a clean copy of these authored
inputs, builds it twice, compares every staged byte, then executes the guest and
mounted level through the native host checker:

\`\`\`sh
KARTY_HOST_NATIVE=/path/to/karty-host \
KARTY_HOST_WEB=/path/to/karty-host.wasm \
KARTY_HOST_CHECK=/path/to/karty-host-check \
KARTY_WORLD_CAMERA_WEB_CHECK=/path/to/check-world-camera-web.mjs \
KARTY_TINYGO=/path/to/sdk-pinned/tinygo \
KARTY_WASM_TOOLS=/path/to/sdk-pinned/wasm-tools \
mise run check-world-camera
\`\`\`

The check requires Playwright's pinned Chromium beside the browser checker. It
executes the guest and packaged level through native Wazero and the actual web
host. It verifies perspective/isometric switching, continuous FPS panning,
isometric orbit, authored actor motion and near passes, release/remount, and
contained failure for a corrupted level artifact.

The TinyGo sample can also be built for the browser with the matching web host:

\`\`\`sh
../../dist/karty build --target web --host /path/to/karty-host.wasm
\`\`\`

Lighting and material maps remain outside this slice.
