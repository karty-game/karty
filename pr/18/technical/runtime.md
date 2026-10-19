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
`EntityVector2D` has transform, fill, alpha and drawing-layer methods.

```go
func marker(game *engine.Game) *engine.EntityVector2D {
    dot := game.NewVector2D(engine.Circle(16), engine.NewVec2(240, 180))
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

| Call or hook                     | Contract                                                             |
| -------------------------------- | -------------------------------------------------------------------- |
| `RequestLevel(name)`             | Returns the request ID for a declared level such as `levels.room`    |
| `OnLevelReady(LevelReady)`       | Supplies the request ID and mounted handle after content preparation |
| `ReleaseLevel(handle)`           | Returns the release operation's request ID                           |
| `OnLevelReleased(LevelReleased)` | Confirms release                                                     |
| `OnLevelFailed(LevelFailed)`     | Supplies the failed request ID and diagnostic                        |
| `OnLevelCancelled`               | Confirms cancellation                                                |

Match an event's `RequestID` to the pending operation before changing state.
Create cameras and resolve level textures after readiness.

Release removes the level's cameras, actors, effects and resources. Discard
their references when release completes. Project-level menu UI can outlive a level.

Admission bounds and cancellation semantics are in
`.karty/docs/level-lifecycle.md`.

[Room setup](../guide/levels.html) · [UI demo loading states](https://github.com/karty-game/karty/blob/main/samples/ui-demo/src/main.go)

## World bodies and queries

Use one `World` and `Body` API for ordinary 2D sprites and flat 3D actors.
Rendering does not choose the physics mode. `WorldPlanar` keeps bodies on XY;
`WorldSpatial` provides gravity and vertical motion for box bodies. Spatial
character following is not supported yet. All coordinates here use metres.

```go
func character(game *engine.Game, level engine.LevelHandle, entity engine.EntityRef) (*engine.Body, error) {
    world := game.World(level)
    if err := world.Configure(engine.WorldSettings{
        Mode: engine.WorldPlanar,
        Min: engine.Vec2{}, Max: engine.Vec2{X: 20, Y: 10}, CellSize: .5,
    }); err != nil { return nil, err }
    return world.AttachBody(entity, engine.CharacterBody())
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
See the project's pinned `.karty/docs/api.md` for limits and exact signatures.
