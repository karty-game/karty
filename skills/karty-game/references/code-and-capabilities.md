# Code and capabilities

## Code and hooks

Keep the starter's registration and import using the actual `go.mod` module path:

```go
import "example.com/my-game/.karty/engine"

//go:wasmexport karty_register
func main() { engine.Run(engine.Hooks{ /* game handlers */ }) }
```

`src/` files can share `package main`; separate player, scene, HUD and audio state
when useful. Put portable rules/codecs in `internal/` packages without generated
imports; connect them through game-owned adapters. Generated assets and UI types
come from `.karty/assets` and `.karty/ui`.

Hooks: `OnStart` initializes/requests scenes; `OnUpdate` runs fixed 60 Hz logic
(`engine.TickSeconds` where available); `OnStop` cleans up. Typed events precede
`OnUpdate`. TinyGo has no scheduler: use update-driven state, not goroutines,
sleeps or blocking I/O. Create persistent objects once; reuse hot-path storage.

## Capability lookup

References below are under `.karty/docs/`; exact signatures come from the selected
`.karty/engine/`. Prefer these host systems over game-owned replacements.
For worked integrations, use the feature-to-sample map in
[Public sources](public-sources.md#working-samples).

| Need               | APIs / authored inputs                                                                             | Reference                                                                 |
| ------------------ | -------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| 2D                 | `NewSprite2D`, `NewLevelSprite2D`, shapes/text, `Transform2D`, `SpriteRenderer2D`, `SetCamera2D`   | `api.md`, `cameras.md`                                                    |
| 3D                 | Ready level → `NewCamera3D(handle, settings)`; `NewSprite3D`, `NewSolid3D`; rooms/portals/prefabs  | `cameras.md`, `world-actors.md`, `static-solids.md`, `solid-renderers.md` |
| Lights/effects     | Authored lighting, animation/particle presets; `NewPointLight`, `NewSpotLight`                     | `lighting.md`, `dynamic-lights.md`, `animations.md`                       |
| Input              | `OnKeyDown`, `OnKeyUp`, `OnPointer`, `OnPointerReleased`, `OnInputReset`, `SetCursorCaptured`      | `client-hooks.md`, `pointer-input.md`                                     |
| Player             | Installed controller package and its generated SDK adapter                                         | Package README and component documentation                                |
| UI                 | `.kui`, widgets/themes, typed props/callbacks, generated view factories                            | `kartui.md`, `widgets.md`, `styles.md`, `theme.md`                        |
| Physics            | `game.World(handle)`, `Configure`, `AttachBody`, body movement/forces/teleport                     | `api.md`, `simulation.md`, `physics.md`, `customization.md`               |
| Queries            | `World.Raycast`, `World.OverlapBox`; asynchronous result hooks                                     | `api.md`, `customization.md`                                              |
| Navigation         | `World.FindPath`, `World.Navigate`, journey lifecycle                                              | `navigation.md`, `simulation.md`                                          |
| Actors/actions     | `FindEntity`, `FindEntitiesByTag`, query results; `//karty:action`, `//karty:condition`, sequences | `world-actors.md`, `client-hooks.md`                                      |
| Sound              | `PlaySound`, `PlaySoundFrom`, listener, `PlayMusic`, `PlayAmbience`, emitters                      | `assets.md`, `audio-emitters.md`                                          |
| Video              | Declared video ID, `PlayVideo`, `StopVideo`                                                        | `assets.md`                                                               |
| Saves              | `ReadSave`, `WriteSave`, `DeleteSave`; `[project].storage_id`                                      | `saves.md`                                                                |
| Pause/presentation | World time scale, focus policy, camera settings                                                    | `customization.md`, `cameras.md`                                          |

Use the installed `controllers-3d` package's first-person component after
camera/level readiness. The package owns body/gravity policy,
input, movement, touch controls and cleanup. Do not also route its controls or
move its body each update; use supported movement hooks for extensions. Choose
a package version compatible with the pinned SDK. The engine supplies generic
camera, physics and component lifecycle APIs; player policy belongs to packages.
Read the installed package's contract before using its component API.

## Scene, entity and simulation contracts

- `RequestLevel` returns a request ID. Match ready/failed/cancelled events before
  using its mount; cancel obsolete requests and release unused levels.
- Metadata/data/query slices are borrowed for their update; copy retained values.
  Queries need matching result hooks; never block or assume next-frame completion.
- Typed `Ref()` and discovery return `EntityRef`; `Alive()` checks validity.
  Release/replacement/destruction retires references. Save authored names and
  gameplay variables, never runtime handles or numeric asset IDs.
- Movement setters propose poses. Use accepted `ref.Pose()`/body state as the
  next baseline, including portal crossings; `WatchPose` is optional. Local
  proposals do not immediately become accepted state. Keep body origin, sprite
  anchor and camera eye height distinct.
- Rendering does not select physics. Configure one `WorldPlanar` or
  `WorldSpatial` mode per mount. Physics uses metres/seconds; canvas uses pixels
  with explicit mapping. Reconfiguring one world cannot combine spatial boxes
  and planar character following. Full spatial character traversal, stairs and
  jumping need documented support or narrow game extensions.
- `CharacterBodySettings` is planar; `DynamicBoxSettings` supports spatial boxes.
  Use `Body.Move`/`Stop`, velocity, forces/impulses and `Teleport`; do not integrate
  the same body's position or duplicate mounted obstacles in another solver.
- `FindPath` computes a route without moving/replanning. It supports planar and
  constrained static flat spatial queries. Spatial routes need one floor Z and
  ordinary reciprocal portals; terrain, slopes/stairs, stacked floors,
  transformed portals and moving-obstacle planning are unsupported.
  `Navigate` follows planar character bodies. Spatial following is a game adapter
  using host collision, not another pathfinder.
- Tags/actions do not create proximity, picking or collision triggers; game rules
  choose when to invoke them. Handle checked errors: host aggregate validation
  can reject a complete command batch atomically.

## Lighting and physics recipes

Fragments assume an `engine.Game` named `game`, ready `handle` and live `camera`.
Lighting authoring/baking is in [Assets and levels](assets-and-levels.md).
A torch uses a light component, separate from its emissive artwork:

```go
torch := game.NewSpotLight(handle, engine.Vec3{},
    engine.NewVec3(2, 1.8, 1.4), 12, 0.2, 0.45)
torch.AttachTo(camera.Ref(), engine.NewVec3(0.12, 0.1, -0.1))
// torch.SetShadows(true) // only with supported topology and free shadow slot
```

Create once, retain, toggle `SetEnabled`. Cone half-angles use radians; attachment
axes are X right/Y forward/Z up and follow accepted parent poses. The reviewed
contract permits 50 unbaked authored + dynamic lights per mount and one shadowed
spotlight, counting disabled lights. Lights default unshadowed; point lights
cannot shadow. Shadowed spots require ordinary connected reciprocal portals,
without transformed portals. Verify pinned budgets before reserving lights.

A movable crate needs a solid-only `renderable: true` prefab named `crate`, with
collision-disabled renderer solids, plus a host body. Do not also place a
collidable static instance. In an error-returning setup helper, configure the
world once unless another owner already did:

```go
world := game.World(handle)
if err := world.Configure(engine.WorldSettings{
    Mode: engine.WorldSpatial, Gravity: engine.Vec3{Z: -9.81},
}); err != nil { return err }
solid := game.NewSolid3D(handle,
    engine.NewTransform3D(engine.Vec3{X: 2, Y: 2, Z: 1}),
    engine.SolidRenderer{Prefab: "crate"})
settings := engine.DynamicBoxSettings()
settings.Collider = engine.BoxCollider{HalfExtents: engine.Vec3{X: .5, Y: .5, Z: .5}}
body, err := world.AttachBody(solid.Ref(), settings)
if err != nil { return err }
_ = body // retain for impulses/teleports; the host advances it automatically
```

Collider size is independent of visual scale. This zero-offset collider needs a
matching centred mesh: footprint X/Y ±0.5, bottom/top Z -0.5/0.5. The default box
preset instead offsets upward from a feet origin. A body owns its entity pose;
ordinary transform setters cannot also drive it.

## KartUI

Use supplied widgets/layout/themes for menus, HUDs, buttons and dialogue.
In SDK/host 0.0.17+, KartUI renders separately at presentation resolution, keeping
interface text sharp
when the game image is rendered at a lower resolution. Ordinary `Text2D`, sprites
and shapes stay in the game-resolution image even at high numbered layers.
Layers affect ordering and 2D camera layer selection; they do not move entities
between the 3D → game 2D → UI passes.
Use `Text2D` for game-space text, not interface text. See the selected SDK's
`cameras.md` responsive presentation contract.

Example `ui/views/pause.kui`:

```kui
<template>
  <panel modal="true" onBack={props.Resume}>
    <label>Paused</label>
    <button onClick={props.Resume}>Resume</button>
  </panel>
</template>
<script setup lang="go">
type PauseProps struct { Resume func() }
func setup(props PauseProps) {}
</script>
```

- Required `<template>` has one panel root; script/style are optional. `setup`
  runs once per instance. Layouts use slots and no runtime script. Keyed children
  preserve instance state; use stable keys, not list positions.
- Build, then inspect `.karty/ui/` for the factory/props names. Retain the view;
  `Invalidate()` after external model changes, `Close()` when finished. UI-local
  changes outside automatically invalidating handlers need `invalidate()`.
- Supported widgets own painting, focus, hit-testing, scrolling and touch.
  KartUI is bounded markup/styles, not HTML/CSS; unsupported styles may warn and
  be ignored. Independent panels/components preserve painting caches. Use label
  `reveal` for dialogue typing while retaining final text/wrapping.
- Modal roots capture game input; supported nonmodal HUD controls also deliver
  ordinary game pointer events, so guard world actions. Clear game-owned
  keys/touches/capture on `OnInputReset`. Map pointers through the camera/viewport inverse.
- Modal capture does not pause simulation. World time scale does not pause
  audio/video; apply the supported game/media policy separately.

## Media, storage and packages

Use WAV one-shots, declared streaming music/environment loops and persistent
emitters. `PlaySoundFrom` uses XY attenuation; emitters support XYZ attenuation
and optional collision obstruction. Set/refresh the camera listener and its yaw
basis. Browser audio needs user interaction. Emitter `SetOptions` changes volume;
`StreamOptions` may expose only loop/crossfade, with no master/music/ambience gain
setter. Bind sliders only to real controls; do not invent buses or a guest mixer.
Video has one bounded player; do not assume seek/loop/pause/completion hooks.

Saves are bounded game-scoped bytes, not files/cloud sync. Use stable storage
identity and bounded binary codecs; handle unavailable/quota/corrupt data.
The cartridge dependency closure rejects JSON/CSV/XML/gob/YAML/TOML decoders,
including indirect imports. Compile editable data during builds. Do not assume
guest filesystem, HTTP, processes or environment access. Use supported bounded
stdout/stderr diagnostics rather than filesystem logs or `log/slog` dependencies.

## Acquire reusable packages

Check `karty.toml` first: a starter may already pin the package it needs.
The [Plugins catalog](https://karty-game.github.io/karty-packages/) lists published
controllers, FPS materials/prefabs/decals, dialogue, saves and Tiled packages.
CLI/SDK installation does not install them all. Import by name for the latest
stable release, or specify an exact published version:

```sh
karty package import savegame
karty package import fps-kit@0.0.1
```

Inspect the selected package's README/manifest for SDK compatibility. Its optional
`sdk` field is a minimum; the game's exact SDK pin must meet or exceed it. Imports
record immutable archive URLs and SHA-256 checksums in `karty.toml`; keep those
pins and any Go module changes in Git. Builds restore the pinned packages under
ignored `.karty/packages/` without consulting the catalog, and can reuse verified
archives offline. Use `karty package upgrade NAME` to select the latest stable
release of an installed archive dependency, or
`karty package import --replace NAME@VERSION` for an exact release. Upgrade always
uses the catalog and rejects linked development folders; replace a link explicitly
when switching to published bytes. A package's manifest version need not be a
public Go module tag.

For package development, link a source folder from a selected public revision of
[`karty-packages`](https://github.com/karty-game/karty-packages), or another
publisher's repository. Keep folder imports explicit; another machine needs
that source location too. Local `.kpkg` archives also remain supported:

```sh
karty package import --replace ../karty-packages/packages/savegame
karty package export --output /path/to/savegame.kpkg ../karty-packages/packages/savegame
karty package import --replace /path/to/savegame.kpkg
```

Yarn/Tiled processors produce runtime binaries; package import alone does not
select processors or register artwork. Edit maintained sources, never restored
`.karty/packages/` copies.
