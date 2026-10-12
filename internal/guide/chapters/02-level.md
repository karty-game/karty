## Build one room

A level has a manifest, world geometry, and its own textures. Create
`levels/room/`, add a tileable `surface.png`, and save this as `level.toml`:

```toml
[level]
name = "levels.room"
kind = "world"

[world]
source = "world.yaml"

[[textures]]
name = "surface"
source = "surface.png"
profile = "environment"
```

Save this as `world.yaml`. X/Y describe the floor plan; Z is height. These
coordinates make an eight-metre square room with a three-metre ceiling.

```yaml
version: 6
rooms:
  - id: room
    floor: {c: 0}
    ceiling: {c: 3}
    floor_material: surface
    ceiling_material: surface
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 8, y: 0}, material: surface}
      - {id: east, start: {x: 8, y: 0}, end: {x: 8, y: 8}, material: surface}
      - {id: north, start: {x: 8, y: 8}, end: {x: 0, y: 8}, material: surface}
      - {id: west, start: {x: 0, y: 8}, end: {x: 0, y: 0}, material: surface}
```

Walk the boundary counterclockwise and keep each edge joined to the next.
Run `karty schema` for editor suggestions, or `karty schema --check` to check
the world before processing assets.

Replace `src/main.go` with this room viewer. Change the import if your project
has a different name. The camera starts at `(2, 2, 1.7)`; the text shows loading
errors above the scene.

```go
package main

import "example.com/my-game/.karty/engine"

type Game struct {
    engine.Game
    request engine.LevelRequestID
    camera *engine.EntityCamera3D
    label *engine.EntityText2D
}

//go:wasmexport karty_register
func main() {
    game := &Game{}
    engine.Run(engine.Hooks{
        OnStart: func() {
            game.label = game.NewText2D("Loading room...", engine.NewVec2D(24, 24))
            game.request = game.RequestLevel("levels.room")
        },
        OnLevelReady: func(event engine.LevelReady) {
            if event.RequestID != game.request { return }
            game.camera = game.NewCamera3D(event.Handle, engine.CameraSettings{
                Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
                Position: engine.NewVec3D(2, 2, 1.7), FOVY: 1.05,
                Near: 0.05, Far: 100, OrthoHeight: 10,
                ViewportWidth: 960, ViewportHeight: 540,
            })
            game.camera.SetLayer(-1)
            game.label.SetText("")
        },
        OnLevelFailed: func(event engine.LevelFailed) {
            if event.RequestID == game.request { game.label.SetText(event.Diagnostic) }
        },
    })
}
```

Run `karty dev` to view the room. Match the viewport to your `karty.toml`
resolution if you change it. This camera stays still. Add [first-person
controls](fps.html) to walk around, or an [isometric camera](isometric.html) to
view the room from above.

![Roman court and galleries connects textured rooms with open passages.](assets/world-camera.jpg)

Add a second room only after you can explore the first. A connection names an
edge in each room and opens a portal between them; see the court sample for
complete connections.
