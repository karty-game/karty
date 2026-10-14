# Make an isometric game

Choose this view when players need to see several rooms or plan around objects.
Start with [one room](levels.html), then replace its fixed viewpoint with an
orbiting camera. Keep the loading and failure hooks from that page.

![The Roman court sample viewed from above with its isometric camera.](assets/isometric.jpg)

Try the camera switch in [Roman court & galleries](../world-camera/) to compare
its walking and overhead views.

## Frame the room

Use these settings in `game.NewCamera3D(event.Handle, settings)`. It looks down
toward the centre of the eight-metre room. The diagonal yaw and downward pitch
give the isometric angle. Angles use radians.

```go
settings := engine.CameraSettings{
    Projection: engine.CameraProjectionIsometric,
    Output: engine.CameraOutputFull,
    Position: engine.NewVec3D(-7.313708, -7.313708, 11.313708),
    Yaw: 0.7853982, Pitch: -0.6154797,
    FOVY: 1.05, OrthoHeight: 12,
    Near: 0.05, Far: 100,
    ViewportWidth: 960, ViewportHeight: 540,
}
```

`OrthoHeight` is the visible height in world units. Increase it to show more of
the map. Parallel projection keeps distant objects the same size. This camera
can sit outside the world. Downward-facing ceilings disappear when viewed
from above.

Keep `game.camera.SetLayer(-1)` and match the viewport to `karty.toml`.

## Orbit without moving a player

Q/E rotate the view around `(4, 4)`. The camera keeps its distance, height, and
pitch. These controls move the view only; they do not move a player or actor.
Add `math` to the imports and these fields inside `Game`:

```go
orbitLeft, orbitRight bool
isoReady bool
```

After camera creation and the layer setting, put this before clearing the
loading label:

```go
if _, err := game.WatchTransform(game.camera, nil); err != nil {
    game.label.SetText(err.Error())
    return
}
game.isoReady = true
```

`WatchTransform` keeps `camera.Settings` synchronized with the accepted pose.
Start each proposal from those settings. Add these entries to the existing
`engine.Hooks`:

```go
OnUpdate: func(engine.Frame) { game.updateOrbit() },
OnKeyDown: func(key engine.Key) { game.keyOrbit(key, true) },
OnKeyUp: func(key engine.Key) { game.keyOrbit(key, false) },
OnInputReset: game.resetOrbit,
```

Add these methods after `main`. The orbit is centred on the room's floor, with
a radius of 16 metres. Change the centre when your level grows.

```go
func (game *Game) keyOrbit(key engine.Key, down bool) {
    switch key {
    case engine.KeyQ: game.orbitLeft = down
    case engine.KeyE: game.orbitRight = down
    }
}

func (game *Game) resetOrbit() {
    game.orbitLeft, game.orbitRight = false, false
}

func (game *Game) updateOrbit() {
    if game.camera == nil || !game.isoReady { return }
    var direction float32
    if game.orbitLeft { direction-- }
    if game.orbitRight { direction++ }
    if direction == 0 { return }

    settings := game.camera.Settings
    settings.Yaw += direction * 0.015
    // Wrap the angle so a long orbit stays precise.
    settings.Yaw = float32(math.Mod(float64(settings.Yaw)+2*math.Pi, 2*math.Pi))
    settings.Position.X = 4 - float32(math.Sin(float64(settings.Yaw))) * 16
    settings.Position.Y = 4 - float32(math.Cos(float64(settings.Yaw))) * 16
    game.camera.SetSettings(settings)
}
```

Run `karty dev`, click the game, and hold Q or E. The view should turn around
the room without changing its scale. Switch away and back to check that
`OnInputReset` stops the orbit.

## Build around the view

Use [textures](textures.html) that read at this distance and
[prefabs](prefabs.html) for objects with distinct shapes. Check
[lighting](lighting.html) while orbiting so important objects stay visible.

An orbiting camera's position is outside the room. Base player proximity
checks on a player actor or separate walking controller. The
[interaction guide](interactions.html) explains accepted poses and explicit
input hooks; its camera trigger applies to a walking camera.

Add [KartUI](ui.html) for selection and status, [media](media.html) for feedback,
then [share a build](share.html). The Roman court sample shows a separate
walking controller and an isometric camera over the same world.
