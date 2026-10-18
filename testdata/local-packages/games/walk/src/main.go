package main

import (
	"example.com/stdlib-walk/.karty/engine"
	"github.com/karty-game/stdlib-spike/controls"
	"github.com/karty-game/stdlib-spike/fps"
)

// The game owns lifecycle, camera creation, collider, input routing and cleanup.
// The shared library never imports this game's generated engine package.
type Game struct {
	engine.Game
	controller *fps.Controller
	input      *controls.Input[engine.Key]
	camera     *engine.EntityCamera3D
	label      *engine.EntityText2D
	level      engine.LevelHandle
	request    engine.RequestID
	paused     bool
}

//go:wasmexport karty_register
func main() {
	game := &Game{}
	controller, err := fps.New(controllerOptions())
	if err != nil {
		panic(err)
	}
	game.controller = controller
	game.input = controls.New(bindings())
	engine.Run(engine.Hooks{
		OnStart: func() {
			game.label = game.NewText2D("Loading controller spike...", engine.NewVec2(20, 20))
			game.label.SetLayer(20)
			game.request = game.RequestLevel("levels.room")
		},
		OnLevelReady:  game.ready,
		OnLevelFailed: func(event engine.LevelFailed) { game.label.SetText(event.Diagnostic) },
		OnUpdate: func(engine.Frame) {
			// Game-specific behavior can edit intent without copying the controller.
			intent := game.input.Sample(72, .15)
			customize(&intent)
			game.controller.Update(intent)
		},
		OnKeyDown:    func(key engine.Key) { game.key(key, true) },
		OnKeyUp:      func(key engine.Key) { game.key(key, false) },
		OnPointer:    game.pointer,
		OnInputReset: game.reset,
		OnStop:       game.stop,
	})
}

func (game *Game) ready(event engine.LevelReady) {
	if event.RequestID != game.request || game.level != 0 {
		game.ReleaseLevel(event.Handle)
		return
	}
	game.level, game.request = event.Handle, 0
	game.replaceCamera()
}

func (game *Game) replaceCamera() {
	if game.level == 0 {
		return
	}
	game.reset()
	game.controller.Attach(nil)
	if game.camera != nil {
		game.camera.Destroy()
	}
	game.camera = game.NewCamera3D(game.level, engine.CameraSettings{
		Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
		Position: engine.NewVec3(4, 4, 1.7), FOVY: 1.05, Near: .05, Far: 60,
		OrthoHeight: 10, ViewportWidth: 960, ViewportHeight: 540,
	})
	game.camera.SetLayer(-10)
	world := game.World(game.level)
	if err := world.Configure(engine.WorldSettings{Mode: engine.WorldSpatial, Gravity: engine.Vec3{Z: -9.81}}); err != nil {
		panic(err)
	}
	settings := engine.DynamicBox()
	settings.Collider = engine.BoxCollider{HalfExtents: engine.Vec3{X: .3, Y: .3, Z: .85}, Offset: engine.Vec3{Z: -.85}}
	settings.Mass = 80
	settings.Rotate = false
	settings.Forces.Buoyancy = false
	physics, err := world.AttachBody(game.camera.Ref(), settings)
	if err != nil {
		panic(err)
	}

	game.controller.Attach(cameraBody{physics, game.camera})
	game.controller.SetEnabled(!game.paused)
	game.label.SetText(profileName + " ready | P: pause | V: replace camera | Esc: release")
}

func (game *Game) key(key engine.Key, down bool) {
	// These actions belong to the game and never reach the input adapter.
	switch key {
	case engine.KeyP:
		if down {
			game.reset()
			game.paused = !game.paused
			game.controller.SetEnabled(!game.paused)
		}
		return
	case engine.KeyV:
		if down {
			game.replaceCamera()
		}
		return
	case engine.KeyEscape:
		if down {
			game.reset()
		}
		return
	}
	if !game.paused {
		game.input.Key(key, down)
	}
}

func (game *Game) pointer(pointer engine.PointerInput) {
	if pointer.Phase == engine.PointerCancel {
		game.reset()
		return
	}
	if game.paused || game.camera == nil {
		return
	}
	if pointer.Kind == engine.PointerMouse {
		if pointer.Phase == engine.PointerDown && pointer.Buttons&1 != 0 && pointer.Y > 100 {
			game.SetCursorCaptured(true)
		}
		if pointer.Phase == engine.PointerMove && pointer.Captured {
			game.input.Mouse(pointer.DeltaX, pointer.DeltaY)
		}
		return
	}
	if pointer.Kind != engine.PointerTouch {
		return
	}
	switch pointer.Phase {
	case engine.PointerDown:
		if pointer.Y > 100 {
			slot := 0
			if pointer.X >= 480 {
				slot = 1
			}
			game.input.BeginTouch(slot, pointer.ID, pointer.X, pointer.Y)
		}
	case engine.PointerMove:
		game.input.MoveTouch(pointer.ID, pointer.X, pointer.Y)
	case engine.PointerUp:
		game.input.EndTouch(pointer.ID)
	}
}

func (game *Game) reset() {
	game.input.Reset()
	game.controller.Reset()
	game.SetCursorCaptured(false)
}

func (game *Game) stop() {
	game.reset()
	game.controller.Attach(nil)
	if game.camera != nil {
		game.camera.Destroy()
		game.camera = nil
	}
	if game.level != 0 {
		game.ReleaseLevel(game.level)
		game.level = 0
	}
	if game.label != nil {
		game.label.Destroy()
		game.label = nil
	}
}

// This is the complete SDK bridge; it lives in the consumer, not the library.
type cameraBody struct {
	physics *engine.Body
	camera  *engine.EntityCamera3D
}

func (body cameraBody) Heading() (float32, float32) {
	return body.camera.Settings.Yaw, body.camera.Settings.Pitch
}
func (body cameraBody) Look(yaw, pitch float32) {
	settings := body.camera.Settings
	settings.Yaw, settings.Pitch = yaw, pitch
	body.camera.SetSettings(settings)
}
func (body cameraBody) Move(x, y float32) {
	if err := body.physics.SetHorizontalVelocity(engine.NewVec2(x, y)); err != nil {
		panic(err)
	}
}
