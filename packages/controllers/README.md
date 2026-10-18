# Controllers for Karty

One package with first-person, top-down 2D and isometric presets. Each includes
input state and its camera behavior. There is no orbit preset, engine import,
generated binding, registry requirement or separate input/camera installation.

From your game directory:

```sh
karty package import /path/to/karty-cli/packages/controllers
```

Import `github.com/karty-game/controllers` in Go. Use a folder while developing,
or export `controllers-0.1.0.kpkg` and import that archive to share a fixed version.
The portable module requires Go 1.27 and also builds with the SDK's TinyGo.

## Choose a preset

| Constructor                                   | Includes                                                                                                | Example consumers                        |
| --------------------------------------------- | ------------------------------------------------------------------------------------------------------- | ---------------------------------------- |
| `NewFirstPerson(DefaultFirstPersonOptions())` | Keyboard/mouse look, walking/running, pitch clamp, mouse capture requests, two independent touch sticks | Hangar, Environment, Terrain, Crate Room |
| `NewTopDown2D(DefaultTopDown2DOptions())`     | Screen-axis movement, pointer targets, following camera and zoom                                        | Willowbrook                              |
| `NewIsometric(DefaultIsometricOptions())`     | Camera-relative movement, following 3D camera, ground picking and orthographic zoom                     | Vigil                                    |

Constructors return a controller and an error. Start with the default options,
change the fields you need, then construct. `Configure` validates changes before
applying them and releases input. Zero speed or sensitivity remains zero.
Keep the returned controller for the lifetime of the player; do not copy it
while input is active.

## Connect your game's SDK once

Karty generates SDK types under each game's own module. The package therefore
uses ordinary numbers and vectors. The game routes its hooks to the preset and
applies the result to its SDK entities. No generated game bindings are shipped.

For first person:

1. Map SDK keys to `Forward`, `Backward`, `Left`, `Right`, `LookLeft`,
   `LookRight`, `LookUp`, `LookDown`, `RunLeft` and `RunRight` with `Key`.
2. Map SDK pointer phases into `Pointer`. Pass whether a new gesture hits the
   scene rather than your UI. Apply its `Capture`/`Release` request to the host;
   `TouchStarted` can create a game-styled overlay from `Sticks`.
3. Call `Step(camera.Settings.Yaw, camera.Settings.Pitch)` once per update.
   Apply changed yaw/pitch to the camera. Send `motion.X*60`, `motion.Y*60` to
   `body.SetHorizontalVelocity`, including zeros when input stops.
4. Call `Reset` on focus loss, pause or view replacement, and release the host
   cursor when appropriate. Stop the old body before replacing or detaching it.

The preset updates at Karty's fixed 60 Hz, independently of rendering. Options
use metres/second, radians/second and radians/mouse pixel. `Motion.X/Y` are
per-update displacements for portable collision and step assistance;
the host receives horizontal velocity and retains gravity. `Motion.Z` is an
optional vertical action axis for game-owned swimming. Read each update from
the host's accepted pose so a blocked move never accumulates.

`Pointer` supplies floating movement/look sticks in the lower 55% of the logical
viewport. Existing contacts continue receiving move/release outside the scene.
`BeginTouch`, `MoveTouch`, `EndTouch` and `Mouse` are available on the same preset
for another gesture design, as used by Crate Room. Draw overlays with your SDK
entities or KartUI; the preset does not impose assets, colors or UI layers.

## Top-down and isometric players

Both return `Planar`. Map movement keys to the same controls. Use the `Alt`
controls for arrow-key aliases, and distinct `RunLeft`/`RunRight` controls for
Shift: releasing one key never releases another key still held.

`Step(acceptedPosition)` returns the next movement displacement. Apply your
collision rules, then feed the accepted position to `Follow(currentFocus,
acceptedPosition, snap)`. `FollowOffset` adds look-ahead; `FollowRate` controls
smoothing per update. Top-down Y increases down the screen. Isometric movement
uses the configured camera yaw.

Use `SetTarget(worldPosition)` for direct pointer movement; keyboard input takes
over and cancels that target. `CancelTarget` also stops a blocked target. A
navigation system can instead use `Speed()/60` as its travel budget, as Vigil
does. The package does not search paths or infer collision geometry.

`ZoomBy(factor)` clamps to `MinZoom`/`MaxZoom`. Zoom means a 2D scale for top-down
or orthographic height for isometric. Apply it to the SDK camera. Isometric
`CameraPosition(focus)` adds the configured fixed camera offset.
`ScreenToGround` takes the current focus and logical viewport dimensions; use
the game's actual current camera, including responsive viewport handling.
It returns false for invalid dimensions. Ground picking assumes Z=0.

See the small SDK adapters in the sample sources. Game hooks, entities, collider
setup, overlay rendering and lifetime remain with the game. Stair assistance,
swimming, pathfinding, animation, quests and weapons can extend a preset without
forking its input or camera calculations.

## Validate

From the CLI repository root:

```sh
mise run test-controllers
mise run build-controllers
mise run lint-controllers
mise run check-controllers-archive
```

The tests cover input ownership, aliases, cancellation, configurable motion,
target arrival, following, projection, zoom and allocation-free steady updates.
Archive checks compile a clean consumer with Go and TinyGo; actual WASM/browser
execution is qualified separately by the migrated sample checks.

The [versioned preset contract](docs/presets-v1.md) records API and lifecycle
decisions. Version 0.1.0 is local source; no public module release is implied.
