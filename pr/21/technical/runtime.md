# Runtime

The host drives cartridge callbacks and manages mounted levels.

## Game lifecycle

`engine.Run(engine.Hooks{...})` registers the cartridge's callbacks. The host
owns the loop and advances the simulation at 60 Hz.

| Hook                                                                   | Called for                                         |
| ---------------------------------------------------------------------- | -------------------------------------------------- |
| `OnStart`                                                              | Game initialization                                |
| `OnUpdate(Frame)`                                                      | Each simulation tick                               |
| `OnKeyDown`, `OnKeyUp`, `OnPointer`                                    | Input delivery                                     |
| `OnInputReset`                                                         | Clearing held input after focus or routing changes |
| `OnLevelReady`, `OnLevelFailed`, `OnLevelCancelled`, `OnLevelReleased` | Level operation results                            |
| `OnStop`                                                               | Cartridge shutdown                                 |

`Frame.Number` identifies the tick. Counting ticks measures simulation time;
it excludes time while the game is suspended. Hooks run on the game loop, so
blocking a callback also blocks updates.

Dispose screen-owned entities, subscriptions and playback when leaving that
screen. `OnStop` only runs at cartridge shutdown.

[Complete game](../guide/2d.html) · [UI demo lifecycle](https://github.com/karty-game/karty/blob/main/samples/ui-demo/src/main.go)

## Entities and components

Typed entities expose the components they support. For example,
`Shape2D` has transform, fill, alpha and drawing-layer methods.

```go
func marker(game *engine.Game) *engine.Shape2D {
    dot := game.NewShape2D(engine.Circle(16), engine.NewVec2(240, 180))
    dot.SetFill(engine.RGBA(110, 231, 204, 255))
    dot.SetLayer(10)
    return dot
}
```

| Operation                                 | Effect                               |
| ----------------------------------------- | ------------------------------------ |
| A setter such as `SetFill` or `Translate` | Sends a change to the host           |
| Direct assignment to a component field    | Changes only the local Go value      |
| `SetAlpha(0)`                             | Hides drawing and retains the entity |
| `Destroy()`                               | Removes the entity                   |

Keep references to entities you control. Destroy them when their owning screen
closes. Level release removes its cameras, actors and resources; discard those
references too. Entity IDs are runtime identities and do not belong in saves.

[Transform 2D](transforms.html#transform-2d) · [Transform 3D](transforms.html#transform-3d) · [Media lab source](https://github.com/karty-game/karty/blob/main/samples/media-lab/src/main.go)

## Levels and resources

Level operations are asynchronous. A `RequestID` identifies an operation;
a `LevelHandle` identifies mounted content.

| Call or hook                     | Contract                                                      |
| -------------------------------- | ------------------------------------------------------------- |
| `LoadLevel(name, onReady)`       | Loads a typed level reference and calls its ready callback    |
| `RequestLevel(name)`             | Low-level string request; ready results use the global hook   |
| `OnLevelReady(LevelReady)`       | Receives ready results from requests without a local callback |
| `ReleaseLevel(handle)`           | Returns the release operation's request ID                    |
| `OnLevelReleased(LevelReleased)` | Confirms release                                              |
| `OnLevelFailed(LevelFailed)`     | Supplies the failed request ID and diagnostic                 |
| `OnLevelCancelled`               | Confirms cancellation                                         |

Use a generated level reference and a request-local success callback:

```go
func loadRoom(game *engine.Game, onReady func(engine.LevelReady)) engine.RequestID {
    return game.LoadLevel(assets.LevelRoom, onReady)
}
```

The CLI generates `.karty/assets/levels.go` from `[level].name` in each
`levels/*/level.toml`; `levels.room` becomes `assets.LevelRoom`, while
`art.opening` becomes `assets.LevelArtOpening`. Identifier collisions fail the
build with both names. Source folder names do not change the logical reference.

Karty delivers success once to the registered callback; `OnLevelFailed` and
`OnLevelCancelled` remain global hooks. Keep a returned request ID when you
need cancellation or failure routing. Dynamic names can use an explicit
`engine.LevelName(name)` conversion, or the existing `RequestLevel(name)` API.

For global failure and release hooks, match the event's `RequestID` to the
pending operation before changing state.
Create cameras and resolve level textures after readiness.

Release removes the level's cameras, actors, effects and resources. Discard
their references when release completes. Project-level menu UI can outlive a level.

Admission bounds and cancellation semantics are in
`.karty/docs/level-lifecycle.md`.

When the installed runtime supports prepared chunk streams, the CLI packages
final render batches, material surface slots and liquid rest geometry at build
time. The builder checks geometry and lightmap coverage before publishing the
chunk checksum; the shipped file contains prepared streams without a duplicate
semantic mesh. The host checks file identity and bounded stream structure, then
uploads it without proving geometry again, rebuilding faces or tessellating
liquid after download. Waves still animate in the vertex shader.
The builder keeps output compatible with the installed runtime. Shader compilation
and GPU uploads still need a live graphics device; the complete encoded chunk
budget includes the prepared streams.

A render chunk downloads as one immutable file. With baked lighting enabled, its
geometry, lighting layout and completed default lightmap travel together. Its
checksum covers all of these resources; the host publishes the chunk after the
complete unit is validated and uploaded.
Shared material atlases belong to the root level, which is ready before its chunks.
For time-of-day or weather lightmaps, the initial chunk embeds its default image.
Other baked conditions are optional image modules loaded when selected. A
replacement becomes active after its complete image is validated and uploaded.

[Room setup](../guide/levels.html) · [UI demo loading states](https://github.com/karty-game/karty/blob/main/samples/ui-demo/src/main.go)

## World bodies and queries

Use one `World` and `Body` API for ordinary 2D sprites and flat 3D actors.
Rendering does not choose the physics mode. `WorldPlanar` keeps bodies on XY;
`WorldSpatial` provides gravity and vertical motion for rigid boxes and upright
`CapsuleCollider` characters. The shared character motor handles slopes and
ordinary steps; the `controllers-3d` package supplies first-person input and mobile controls.
Automatic navigation remains planar. All coordinates here use metres.

```go
func character(game *engine.Game, level engine.LevelHandle, entity engine.EntityRef) (*engine.Body, error) {
    world := game.World(level)
    if err := world.Configure(engine.WorldSettings{
        Mode: engine.WorldPlanar,
        Min: engine.Vec2{}, Max: engine.Vec2{X: 20, Y: 10}, CellSize: .5,
    }); err != nil { return nil, err }
    return world.AttachBody(entity, engine.CharacterBodySettings())
}

func inspect(world engine.World, player *engine.Body) (engine.RequestID, error) {
    return world.Raycast(engine.RaycastQuery{
        Origin: player.Position(), Direction: engine.Vec3{X: 1},
        Distance: 10, Exclude: player.Ref(),
    })
}

func route(world engine.World, player *engine.Body) (engine.RequestID, error) {
    return world.FindPath(player, engine.Vec3{X: 8, Y: 5})
}
```

Use `BoxCollider` or `EllipseCollider` in `BodySettings`; presets provide useful
defaults. `body.Move(direction, speed)` controls a planar character.
`body.SetHorizontalVelocity` controls a spatial box while preserving falling
velocity. `body.Teleport` changes a physical origin deliberately; `body.Detach`
returns pose control to the entity. `World.Navigate` starts planar following;
`World.FindPath` returns route data and also accepts an independent
`NavigationAgent` without requiring a body.

Raycasts are read-only. `OnRaycastResult` distinguishes `HitWorld`, `HitEntity`
and `HitMiss`; entity hits carry a lifetime-checked `EntityRef`. `OverlapBox`
returns `HitsWorld` and a borrowed entity-reference slice through
`OnOverlapResult`. Copy borrowed results to retain them after the update.
`ApplyRayImpulse` and `Explode` are separate force-producing operations.
See your project's `.karty/docs/api.md` for limits and exact signatures.
