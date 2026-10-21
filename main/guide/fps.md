# Make a first-person game

The preset combines walking, running, mouse look and two independent touch sticks. The game maps SDK events and applies motion to the last host-accepted camera pose. This flat-room starter keeps eye height fixed; gravity, stairs and jumping belong to your game.

Create the SDK 0.0.17 starter:

```sh
karty new --yes --sdk 0.0.17 --template fps my-game
cd my-game
karty dev
```

The starter includes controllers 0.1.0 in `packages/controllers/` and declares
that folder in `karty.toml`. Keep it in Git. If extending an existing project,
import the package as described in [reusable packages](../design/packages.html).
Use the room and material files from [one room](levels.html); the isometric
starter also supplies its `player` solid prefab in `world.yaml`.

![The Hangar sample uses the first-person preset to explore industrial rooms.](assets/hangar.jpg)

## Connect the first-person preset

The complete `src/main.go` below is the starter's SDK adapter. Change its module
imports if your project has a different name. Keep the viewport and project
resolution at 960 by 540. Constructors validate their options; `Reset` clears
held controls when focus changes.

```go
package main

import (
	"github.com/karty-game/controllers"

	"example.com/my-game/.karty/engine"
)

type Game struct {
	engine.Game
	request                        engine.RequestID
	camera                         *engine.Camera3D
	label                          *engine.Text2D
	controller *controllers.FirstPerson
	fpsReady bool
}

//go:wasmexport karty_register
func main() {
	options := controllers.DefaultFirstPersonOptions()
	options.Width, options.Height = 960, 540
	options.Speed, options.KeyYawSpeed = 2.4, 1.5
	controller, err := controllers.NewFirstPerson(options)
	if err != nil { panic(err) }
	game := &Game{controller: controller}
	engine.Run(engine.Hooks{
		OnUpdate:     func(engine.Frame) { game.updateFPS() },
		OnKeyDown:    func(key engine.Key) { game.keyFPS(key, true) },
		OnKeyUp:      func(key engine.Key) { game.keyFPS(key, false) },
		OnInputReset: game.resetFPS,
		OnPointer: game.pointerFPS,

		OnStart: func() {
			game.label = game.NewText2D("Loading room...", engine.NewVec2(24, 24))
			game.request = game.RequestLevel("levels.room")
		},
		OnLevelReady: func(event engine.LevelReady) {
			if event.RequestID != game.request {
				return
			}
			game.camera = game.NewCamera3D(event.Handle, engine.Camera3DSettings{
				Projection: engine.CameraProjectionPerspective,
				Output:     engine.CameraOutputFull,
				Position:   engine.NewVec3(2, 2, 1.7),
				Yaw:        0, Pitch: 0, VerticalFOV: 1.05,
				NearClip: 0.05, FarClip: 100, OrthoHeight: 10,
				ViewportWidth: 960, ViewportHeight: 540,
			})
			game.camera.SetLayer(-1)

			game.fpsReady = true

			game.label.SetText("WASD: walk | Q/E: turn | click: mouse look | touch: dual sticks")
		},
		OnLevelFailed: func(event engine.LevelFailed) {
			if event.RequestID == game.request {
				game.label.SetText(event.Diagnostic)
			}
		},
	})
}


func (game *Game) keyFPS(key engine.Key, down bool) {
	if key == engine.KeyEscape && down { game.resetFPS(); return }
	if control, ok := movementKey(key); ok { game.controller.Key(control, down); return }
	switch key {
	case engine.KeyQ: game.controller.Key(controllers.LookLeft, down)
	case engine.KeyE: game.controller.Key(controllers.LookRight, down)
	}
}

func (game *Game) resetFPS() {
	game.controller.Reset()
	game.SetCursorCaptured(false)
}

func (game *Game) pointerFPS(pointer engine.PointerInput) {
	if game.camera == nil || !game.fpsReady { return }
	if pointer.Kind != engine.PointerMouse && pointer.Kind != engine.PointerTouch { return }
	var phase controllers.PointerPhase
	switch pointer.Phase {
	case engine.PointerDown: phase = controllers.PointerDown
	case engine.PointerMove: phase = controllers.PointerMove
	case engine.PointerUp: phase = controllers.PointerUp
	case engine.PointerCancel: phase = controllers.PointerCancel
	default: return
	}
	result := game.controller.Pointer(controllers.Pointer{
		Phase: phase, Touch: pointer.Kind == engine.PointerTouch, ID: pointer.ID,
		X: pointer.X, Y: pointer.Y, DeltaX: pointer.DeltaX, DeltaY: pointer.DeltaY,
		Primary: pointer.Buttons&1 != 0, Captured: pointer.Captured,
	}, pointer.X >= 0 && pointer.X <= 960 && pointer.Y >= 0 && pointer.Y <= 540)
	if result.Capture { game.SetCursorCaptured(true) }
	if result.Release { game.SetCursorCaptured(false) }
}

func (game *Game) updateFPS() {
	if game.camera == nil || !game.fpsReady { return }
	// Settings contains the last host-accepted pose automatically.
	settings := game.camera.Settings()
	motion := game.controller.Step(settings.Yaw, settings.Pitch)
	if motion.X == 0 && motion.Y == 0 && motion.Yaw == settings.Yaw && motion.Pitch == settings.Pitch { return }
	settings.Yaw, settings.Pitch = motion.Yaw, motion.Pitch
	settings.Position.X += motion.X
	settings.Position.Y += motion.Y
	game.camera.SetSettings(settings)
}

func movementKey(key engine.Key) (controllers.Control, bool) {
	switch key {
	case engine.KeyW: return controllers.Forward, true
	case engine.KeyS: return controllers.Backward, true
	case engine.KeyA: return controllers.Left, true
	case engine.KeyD: return controllers.Right, true
	case engine.KeyArrowUp: return controllers.ForwardAlt, true
	case engine.KeyArrowDown: return controllers.BackwardAlt, true
	case engine.KeyLeft: return controllers.LeftAlt, true
	case engine.KeyRight: return controllers.RightAlt, true
	case engine.KeyShiftLeft: return controllers.RunLeft, true
	case engine.KeyShiftRight: return controllers.RunRight, true
	}
	return 0, false
}
```

WASD/arrows walk, Shift runs, and Q/E turn. Click to capture mouse look;
Escape releases it. Touch the lower-left side to walk and lower-right to look.
The minimal starter supplies gestures without drawing stick overlays. Route UI
hit tests before accepting new scene gestures when adding menus.

Read `packages/controllers/docs/presets-v1.md` in the generated project for
the complete contract, including picking and pointer targets. Extend the game
with [textures](textures.html), [prefabs](prefabs.html),
[lighting](lighting.html) and [interactions](interactions.html), then
[share a build](share.html).

The interaction guide uses E by default. Use F for interactions here, or
remap this starter's Q/E controls before wiring the interaction hook.
