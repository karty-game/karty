## Make a first-person game

Choose this view for walking through rooms and interacting with objects at eye
height. Start with [one room](levels.html), then add these controls to its viewer.
Keep the loading and failure hooks from that page.

![The Hangar sample uses a first-person camera to explore its industrial rooms.](assets/hangar.jpg)

### Set the camera height

Use these settings in `game.NewCamera3D(event.Handle, settings)`. X/Y are floor
coordinates; Z is height. A yaw of zero faces positive Y. Angles use radians.

```go
settings := engine.CameraSettings{
    Projection: engine.CameraProjectionPerspective,
    Output: engine.CameraOutputFull,
    Position: engine.NewVec3D(2, 2, 1.7),
    Yaw: 0, Pitch: 0, FOVY: 1.05,
    Near: 0.05, Far: 100, OrthoHeight: 10,
    ViewportWidth: 960, ViewportHeight: 540,
}
```

Keep `game.camera.SetLayer(-1)` so the loading label and later HUD appear above
the world. Match the viewport to the resolution in `karty.toml`.

### Add walking controls

W/S move forward and back. A/D strafe. Q/E turn. Add `math` to the imports, and
add these fields inside `Game`:

```go
forward, backward, left, right bool
turnLeft, turnRight bool
fpsReady bool
```

After creating the camera and setting its layer, register accepted transforms.
Put this before `game.label.SetText("")` in `OnLevelReady`:

```go
if _, err := game.WatchTransform(game.camera, nil); err != nil {
    game.label.SetText(err.Error())
    return
}
game.fpsReady = true
```

`WatchTransform` updates `camera.Settings` with the pose the host accepted. The
next frame starts there, including any collision stop or portal change. The
`nil` callback is enough for movement. Add an interaction callback to this same
registration when you need one.

Add these entries to the existing `engine.Hooks`:

```go
OnUpdate: func(engine.Frame) { game.updateFPS() },
OnKeyDown: func(key engine.Key) { game.keyFPS(key, true) },
OnKeyUp: func(key engine.Key) { game.keyFPS(key, false) },
OnInputReset: game.resetFPS,
```

Add these methods after `main`. Karty updates at 60 Hz, so a step of `0.04`
moves 2.4 metres per second. Normalizing the input keeps diagonal movement at
the same speed.

```go
func (game *Game) keyFPS(key engine.Key, down bool) {
    switch key {
    case engine.KeyW: game.forward = down
    case engine.KeyS: game.backward = down
    case engine.KeyA: game.left = down
    case engine.KeyD: game.right = down
    case engine.KeyQ: game.turnLeft = down
    case engine.KeyE: game.turnRight = down
    }
}

func (game *Game) resetFPS() {
    game.forward, game.backward, game.left, game.right = false, false, false, false
    game.turnLeft, game.turnRight = false, false
}

func fpsAxis(positive, negative bool) float32 {
    var value float32
    if positive { value++ }
    if negative { value-- }
    return value
}

func (game *Game) updateFPS() {
    if game.camera == nil || !game.fpsReady { return }
    forward := fpsAxis(game.forward, game.backward)
    strafe := fpsAxis(game.right, game.left)
    turn := fpsAxis(game.turnRight, game.turnLeft)
    if forward == 0 && strafe == 0 && turn == 0 { return }

    if length := math.Hypot(float64(forward), float64(strafe)); length > 1 {
        forward /= float32(length)
        strafe /= float32(length)
    }
    settings := game.camera.Settings
    settings.Yaw += turn * 0.025
    sine := float32(math.Sin(float64(settings.Yaw)))
    cosine := float32(math.Cos(float64(settings.Yaw)))
    settings.Position.X += (forward*sine + strafe*cosine) * 0.04
    settings.Position.Y += (forward*cosine - strafe*sine) * 0.04
    game.camera.SetSettings(settings)
}
```

Run `karty dev`, click the game, and walk toward a wall. Movement stops while
you can still turn. Switch away from the preview and back; `OnInputReset` clears
held keys so walking does not continue after focus changes.

The camera is a point. This controller does not change Z directly or add
gravity, jumping, or automatic stair climbing. Start with a flat room before
adapting the Hangar controller for stairs or touch input.

### Give the room a purpose

Add [textures](textures.html) and [prefabs](prefabs.html) before expanding the
map. Use [lighting](lighting.html) to make the route visible. Then add a nearby
object and an [interaction](interactions.html).

The interaction example uses E. Move turning to another key or use F for the
interaction, and call both handlers from your existing input hook. Use
[KartUI](ui.html) for prompts, [media](media.html) for feedback, and
[share a build](share.html) once the first room is playable.
