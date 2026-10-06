# World camera, actor gallery, and Roman court

Run `karty schema` from this sample's directory before editing the world YAML.
Its schema header provides field diagnostics and material/texture completions;
`karty schema --check` validates the YAML without processing images or lighting.
See [world YAML schemas](../../docs/level-yaml-schema-v1.md).

This renderer sample packages a high-level YAML world with
a concave entry hall, three sloped galleries, a 90-degree corner, and a steeper
ascending corridor, plus a Roman-inspired peristyle connected to the hall's
west wall. Floor planes change direction and grade along the gallery route;
the ceiling stays level across portal seams so the isometric cutaway remains
readable. The mounted contents become host ECS actors and render through the
shared `Camera3D` renderer type using separate FPS and isometric camera entities.

Environment surfaces use three distinct opaque albedo materials: blue-gray floor
tiles, warm brick walls, and pale ceiling panels. The level manifest declares
their PNG textures with the SDK's `environment` profile; `floor_material`,
`ceiling_material`, and boundary `material` names in the YAML resolve to those
level-local textures. The same materials cover prefab instances and portal steps.
Regenerate the checked-in artwork from the CLI repository root with
`mise run generate-world-camera-materials`. To replace only the court wall and
preserve custom artwork:

```sh
mise run generate-world-camera-materials -- --only z-court-plaster
```

The court has sixteen connected bays around an eight-metre inner court, eight
solid decagonal marble columns, mosaic porticoes, pale paving, white
limestone walls with shallow joints and pores, and coffered ceilings. The perimeter ceiling is 4.6 metres high; the
central volume rises to a blue ceiling at 12 metres. This represents a skylight
within the renderer's closed-sector geometry, rather than an actual open sky.
The columns reach the upper ceiling at 12 metres and have regular ten-sided
sections with a 50-centimetre radius. Their
ten facets belong to a reusable room-free `roman-column` prefab; eight nested
instances supply opaque meshes and collision volumes without cutting the bay
boundaries. Normals and depth still describe their real surfaces. The migrated
complete fixture has 69 sectors and 236 directed portals, versus 125 and 348
when the same columns were holes in the bay boundaries. The `roman-court`
instance alone has 61 sectors and 219 outgoing directed portals (218 within the
court and one exit to the hall); the hall/gallery route supplies the remaining
eight sectors and 17 outgoing portals. Basin steps retain their
room geometry because they change walkable elevation.
A hexagonal basin occupies the centre: three 12-centimetre marble steps rise
from the paving to a 64-centimetre rim, with a turquoise water surface 50
centimetres above the court floor and 14 centimetres below the rim. The steps, risers and inner walls are real
connected-sector geometry, visible in normal and depth views. The water is an
opaque decorative texture with a submerged tile pattern, without fluid
simulation or reflections. The court's walking loop skirts the basin, and its
centred isometric viewpoint shows the whole stepped footprint.

The [authored YAML](levels/showcase/world.yaml) uses upright adventurers/ghosts,
crossed trees, fixed wall signs and a blended planar slope decal. The player
marker is camera-facing; actor names do not determine sprite facing modes.
The first gallery's crossed sprite is discovered through its
`spinner` tag and rotated every frame with a typed ECS transform update. Gallery
instances replace the prefab actor's tags. The client also queries the authored
`interactive` tag after mounting and changes matching actors to
`interactive,verified`, exercising authored identity, tag lookup, and mutation.

Controls are authored in [KartUI](ui/views/controls.kui), rendered at display
density, and reflow at 600 CSS pixels. Buttons are at least 44 pixels tall, and
the bottom space keeps them clear of the browser shell's fullscreen button.

- **Movement row ← / ↓ / ↑ / →:** start continuous, free FPS movement in
  the camera's current facing direction, in either view. **■** ends movement
  and turning. Walls, solids and portals are resolved by the host.
- **Look row ← / ↓ / ■ / ↑ / →:** turn left/right, look down/up in FPS,
  or stop motion. Left/right orbit the isometric view; pitch buttons apply to
  FPS only. Pitch stops short of the vertical poles.
- **FPS / Isometric**, **Roman court / Gallery**, and the four render-channel
  buttons select the viewpoint and output directly. The render channel persists across
  movement, view changes, viewpoint jumps, and remounts.
- **Reload:** release and remount the packaged level; all motion stops.
- **Inspect:** stop motion and show the retained camera diagnostics. Starting
  motion closes the inspector, keeping touch controls stable during movement.

Tab or arrow keys move focus; Enter activates a KartUI control. **Hide** or
Escape closes the controls and stops motion. Movement keys also work while the
controls are visible. Hold **W/S** to move forward/back;
**A/D** strafes in FPS and retains orbit control in isometric view. **Q/E** turns
in FPS or orbits in isometric view. Diagonal walking has the same speed as
walking along one axis. With controls hidden, left/right arrows retain the
authored diagnostic tour through the Gallery. Press V to switch cameras,
G to cycle channels, and right-click to reload. Press C or tap the scene to
restore the controls.

A tiny adventurer marks the FPS camera position in isometric view. It follows
the host-corrected FPS position, just behind the eye so it stays out of the FPS
view. Orbiting the isometric camera leaves this marker at the saved FPS position.

The optional scrollable inspector shows the live FPS pose (including host portal
corrections), the isometric pose when active, and the active projection settings.
Angles are in radians; position and angle values retain float32 precision. Include
this panel in screenshots of portal edge artifacts to identify the affected view.

The sample now requires SDK 0.0.9 / API 0.0.7 for its typed hooks and authored
actions, plus the existing static solids/material mapping extensions. SDK 0.0.9
is currently a candidate: install a matching candidate bundle with
`karty sdk install --archive /path/to/sdk-0.0.9.zip --sha256 CHECKSUM 0.0.9` and
supply its matching host. Once published, use the normal installation below.
From the CLI root:

```sh
mise run build
./dist/karty sdk install 0.0.9
cd samples/world-camera
../../dist/karty dev --port 4242
```

Open `http://localhost:4242/`. An optional `--host` selects a local renderer;
it does not install the matching SDK. This sample now pins SDK 0.0.9.

Build and serve the browser sample from the CLI repository root:

```sh
mise run build-world-camera-web
mise run serve-world-camera --addr 127.0.0.1:8094
```

The optional full-scene diagnostic check starts from a clean copy of these authored
inputs, builds it twice, compares every staged byte, then executes the guest and
mounted level through the native host checker:

```sh
KARTY_HOST_NATIVE=/path/to/karty-host \
KARTY_HOST_WEB=/path/to/karty-host.wasm \
KARTY_HOST_CHECK=/path/to/karty-host-check \
KARTY_WORLD_CAMERA_WEB_CHECK=/path/to/check-world-camera-web.mjs \
KARTY_TINYGO=/path/to/sdk-pinned/tinygo \
KARTY_WASM_TOOLS=/path/to/sdk-pinned/wasm-tools \
mise run check-world-camera
```

The check requires Playwright's pinned Chromium beside the browser checker. It
executes the guest and packaged level through native Wazero and the actual web
host. It verifies perspective/isometric switching, continuous FPS panning,
isometric orbit, authored actor motion and near passes, release/remount, and
contained failure for a corrupted level artifact.
It also checks KartUI actions with real primary touch input at 390x844, 320x740,
and 844x390, including movement, Stop, orbit, viewpoint jumps and channels.

The TinyGo sample can also be built for the browser with the matching web host:

```sh
../../dist/karty build --target web --host /path/to/karty-host.wasm
```

Web overrides require their matching `wasm_exec.js` beside the host WASM.

## Moving lighting

The corridor also has a warm orange `corridor-wanderer` point light. It travels
from (3, 1, 2.2) to (15, 1, 3) and back every 12 seconds, passing the hall actors
and gallery trees. Select **Gallery** and **Full**, then stop camera movement to
watch its light sweep across brick walls, floor tiles and actors. Its radius is
four metres. **Albedo**, **Normal** and **Depth** provide stable comparisons.
Reload restarts the motion. The path is authored with `motion` in
[world.yaml](levels/showcase/world.yaml) in the normal sample.
It is an unbaked direct light and does not cast moving shadows; keep it out of
`[lightmap].lights`. The published SDK 0.0.8 host supports the motion extension.

The normal sample now packages a six-light directional lightmap. Static walls
and columns cast shadows, while material normals, texture AO and height relief
remain active. Physical phone GPU performance and manual device interaction remain separate checks.

## Packaged baked lighting

The normal sample includes authored ambient/point lighting, compiled
world-space material UVs, nearest-sampled retro materials and an automatic
offline RNM3 bake with diffuse radiosity. Its level manifest enables
`offline = true`, with 196 samples and one indirect bounce. Generate its ignored
cache before building:

```sh
cd samples/world-camera
../../dist/karty bake --level showcase
```

The result lives in the project's `.karty/bakes/showcase/`. Light definitions remain
in `levels/showcase/world.yaml`, and bake settings in `levels/showcase/level.toml`.
A matching build loads the atlas immediately with
zero runtime bake stages; changing the camera does not rebake lighting.

SDK 0.0.9 retains SDK 0.0.8's
`world/lightmaps-prebaked@1` support. Build and serve from the CLI root:

```sh
mise run build-world-camera-web
mise run serve-world-camera -- --addr 127.0.0.1:4242
```

A local unreleased host can be supplied with the existing build task's `--host`
argument. The published host is selected by default.
Geometry, selected lights, material UVs, source albedo or requested quality edits
invalidate the automatic bake. A missing, stale or corrupt automatic cache
falls back to direct runtime baking with a diagnostic; rerun `karty bake` to
restore bounced lighting and skip startup baking. Explicit manual pairs still
fail validation when stale.
The first offline baker has hard point-light shadows and finite-sample noise;
it does not add moving-object shadows, actor GI probes or transformed-portal
light transport. Higher sample counts reduce indirect noise; configure them
in `[lightmap]` or supply `--samples`/`--bounces` to the bake command.

Retro presentation preserves nearest-sampled source pixels. Runtime material
mip selection follows the atlas texel footprint with zero LOD bias and blends
progressively filtered distant levels. The white court wall has shallow
limestone joints and pores; its normals, height and AO are generated at build
time. Baked diffuse does not disable the view-dependent runtime rim response.

## Browser renderer debug controls

The sample opts in with `[project.debug] renderer = true` in `karty.toml`.
The browser footer contains **Lightmap** and **AA: On/Off** next to the existing
game controls. Rebuild the CLI and use a compatible host when adding this
feature; restart an already-running `karty dev` process to use the new CLI,
then reload the page.

**Lightmap** shows the actual completed baked atlas and its chart/light counts.
The three directional RNM coefficient tiles are decoded from RGBM, tone mapped
and shown from left to right. This supports the imported bounced bake, rather
than exporting or rebaking it. Use **1:1 pixels** to inspect texels or **Fit atlas**
to show the complete image; **Refresh** requests a new snapshot. The corridor's
moving light is evaluated live and is absent from the six-light static bake.
Capturing reads GPU pixels only when requested, so normal game frames do not
perform debug readbacks. Escape or **Close** dismisses the screen.

**AA: On/Off** switches SMAA for Full output. It survives camera switches and
level remounts within the page. Normal, Depth and Albedo remain unchanged.

Wall detail at distance uses bounded 8× anisotropic filtering. The short
screen-pixel UV axis selects mip LOD, while eight taps filter along the long
axis; very elongated footprints widen to respect the cap. This preserves more
grazing-wall detail than the earlier isotropic filter while still reducing
distant shimmer. Filtered normal vectors approach the flat normal when detail
averages out, and bounded relief fades at grazing angles. The AA toggle operates
after material sampling and does not change the material filter.

## Camera configuration

Both camera viewports use `[project.resolution]` from `karty.toml`, including
viewpoint jumps and level reloads. The UI remains at display density.

```toml
[project.resolution]
width = 1280
height = 720

[project.camera]
fov_y_degrees = 60
near = 0.05
far = 80
ortho_height = 30
```

The CLI generates `.karty/config/project.go`; the sample reads its camera
constants rather than duplicating resolution or projection settings in Go.
Editing these settings under `karty dev` rebuilds the cartridge. Perspective
uses vertical FOV; isometric uses `ortho_height` in world units. Resolution
changes render detail; they do not add anti-aliasing. The current SDK camera
contract supports at most 1280×720 and rejects larger configured camera sizes
at build time. Scene poses and the authored diagnostic route remain sample
content.

Offline bakes now use `bake_denoise = "medium"` in showcase's `[lightmap]`
settings to reduce indirect-light grain without filtering material textures
or direct shadows. Run `karty bake --level showcase`, then rebuild/restart the
normal sample with the current host. Set `bake_denoise = "off"` to compare.

## Hook and component usage

The main [client](src/main.go) runs `engine.Hooks` directly. Its frame handler
only clears the one-frame input guard and advances camera movement and spinner
animation. Lifecycle, input and actor-query events have typed handlers; there
is no event switch in `Update`.

- [Camera controller](src/cameras.go) owns held/touch input, walking, the authored
  tour and isometric orbit. `WatchTransform` calls its component handlers after
  the SDK applies accepted host poses, including portal corrections.
- [Marker follower](src/actors.go) reacts to accepted FPS movement, with an
  initial update when its authored actor resolves. Identical feedback emits no
  command. Orbiting the isometric camera leaves the player marker in place.
- [Camera HUD](src/camera_hud.go) refreshes on accepted camera changes and explicit
  view/channel/viewpoint changes, rather than formatting text every frame.
- Spinner animation advances once per frame. Release/reload clears actor
  references immediately and rejects late results from the old mount.

The authored [welcome sequence](levels/showcase/actions.json) demonstrates a
condition, wait and action using the stable `hall/player-marker` identity. The
old [candidate directory](client-hooks/README.md) points to this single source.

Routine validation uses a small CPU harness with real public SDK components and
a captured host transport. It reads only client/component source and UI source;
it loads no sample geometry, textures, bake, graphical host or browser:

```sh
mise run check-world-camera-components
KARTY_HOME=/path/to/candidate-sdk-cache mise run check-world-camera-hooks
```

The first task also runs in the ordinary unit suite using the released SDK's
compatible camera/actor types. The second requires SDK 0.0.9 and exercises the
actual typed hook registrations, authored adapters, mount/remount, accepted
feedback and immediate UI refresh. Both cap test execution at nine seconds;
build/install time is separate. Full-scene graphics and manual/device review
remain explicit diagnostics, not prerequisites for these behavior tests.
