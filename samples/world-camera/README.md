# World camera and actor gallery

This is the S2 unlit renderer sample. It packages a high-level YAML world with
one concave sloped hall and two transformed instances of the same room prefab.
The mounted contents become host ECS actors and render through the same
`Camera3D` entity in perspective and isometric projections.

The gallery covers camera-facing and upright billboards, crossed geometry,
a one-sided fixed wall sprite, a blended sprite, and a fixed planar decal on
the sloped floor. Both prefab instances replace the prefab actor's tags. The
client queries the authored `interactive` tag after mounting and changes the
matching actors to `interactive,verified`, exercising authored identity, tag
lookup, and typed ECS mutation.

- **Left arrow:** perspective/FPS camera
- **Right arrow:** orthographic/isometric camera
- **Right-click:** step the FPS camera through generated room/portal seams
- **Click/tap:** release and remount the packaged level

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
host. It verifies perspective/isometric switching, generated-sector seam
routes, authored actor pixels, release/remount, and contained failure for a
corrupted level artifact.

The TinyGo sample can also be built for the browser with the matching web host:

\`\`\`sh
../../dist/karty build --target web --host /path/to/karty-host.wasm
\`\`\`

Lighting and material maps remain outside this slice.
