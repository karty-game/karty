package main

import (
	"example.com/environment/.karty/config"
	"example.com/environment/.karty/engine"
	"example.com/environment/internal/fps"
	"github.com/karty-game/controllers"
)

// FPSController owns desktop input, two independent touch sticks and their overlay.
// Horizontal velocity retains the host's gravity and collision response.
type FPSController struct {
	game          *engine.Game
	camera        *engine.EntityCamera3D
	body          *engine.Body
	input         fps.Input
	sticks        [2]stickOverlay
	SceneContains func(x, y float32) bool
	OnTouchMode   func()
}

type stickOverlay struct{ base, thumb *engine.EntityVector2D }

func NewFPSController(game *engine.Game) *FPSController {
	return &FPSController{game: game, input: fps.New(float32(config.ResolutionWidth), float32(config.ResolutionHeight))}
}

func (controller *FPSController) SetCamera(camera *engine.EntityCamera3D, body *engine.Body) {
	controller.Release()
	controller.camera, controller.body = camera, body
}

func (controller *FPSController) Key(key engine.Key, down bool) {
	if key == engine.KeyEscape && down {
		controller.Release()
		return
	}
	var control fps.Control
	switch key {
	case engine.KeyW:
		control = fps.Forward
	case engine.KeyS:
		control = fps.Backward
	case engine.KeyA:
		control = fps.Left
	case engine.KeyD:
		control = fps.Right
	case engine.KeyLeft:
		control = fps.LookLeft
	case engine.KeyRight:
		control = fps.LookRight
	case engine.KeyArrowUp:
		control = fps.LookUp
	case engine.KeyArrowDown:
		control = fps.LookDown
	case engine.KeyShiftLeft:
		control = fps.RunLeft
	case engine.KeyShiftRight:
		control = fps.RunRight
	default:
		return
	}
	controller.input.Key(control, down)
}

func (controller *FPSController) Pointer(pointer engine.PointerInput) {
	if controller.camera == nil {
		return
	}
	if pointer.Kind != engine.PointerMouse && pointer.Kind != engine.PointerTouch {
		return
	}
	var phase controllers.PointerPhase
	switch pointer.Phase {
	case engine.PointerDown:
		phase = controllers.PointerDown
	case engine.PointerMove:
		phase = controllers.PointerMove
	case engine.PointerUp:
		phase = controllers.PointerUp
	case engine.PointerCancel:
		phase = controllers.PointerCancel
	default:
		return
	}
	scene := controller.SceneContains == nil || controller.SceneContains(pointer.X, pointer.Y)
	result := controller.input.Pointer(controllers.Pointer{
		Phase: phase, Touch: pointer.Kind == engine.PointerTouch, ID: pointer.ID,
		X: pointer.X, Y: pointer.Y, DeltaX: pointer.DeltaX, DeltaY: pointer.DeltaY,
		Primary: pointer.Buttons&1 != 0, Captured: pointer.Captured,
	}, scene)
	if result.Capture {
		controller.game.SetCursorCaptured(true)
	}
	if result.Release {
		controller.Reset()
		controller.game.SetCursorCaptured(false)
	}
	if result.TouchStarted {

		controller.showSticks()
		if controller.OnTouchMode != nil {
			controller.OnTouchMode()
		}
	}
	controller.drawSticks()
}

func (controller *FPSController) Update(engine.Frame) {
	if controller.camera == nil {
		return
	}
	settings := controller.camera.Settings
	motion := controller.input.Step(settings.Yaw, settings.Pitch)
	if motion.Yaw != settings.Yaw || motion.Pitch != settings.Pitch {
		settings.Yaw, settings.Pitch = motion.Yaw, motion.Pitch
		controller.camera.SetSettings(settings)
	}
	// Input displacements are fixed 60 Hz proposals. Send zero on release too;
	// positional camera setters teleport an attached body and clear momentum.
	if err := controller.body.SetHorizontalVelocity(engine.NewVec2(motion.X*60, motion.Y*60)); err != nil {
		panic(err)
	}
}

func (controller *FPSController) Reset() {
	controller.input.Reset()
	controller.drawSticks()
}

func (controller *FPSController) Release() {
	controller.game.SetCursorCaptured(false)
	controller.Reset()
}

func (controller *FPSController) Close() {
	controller.Release()
	controller.camera = nil
	for index := range controller.sticks {
		if controller.sticks[index].base != nil {
			controller.sticks[index].base.Destroy()
			controller.sticks[index].thumb.Destroy()
		}
		controller.sticks[index] = stickOverlay{}
	}
}

func (controller *FPSController) Help() string {
	if controller.input.TouchMode() {
		return "Left stick walk · Right stick look"
	}
	return "WASD / mouse · Shift run · V view · H controls"
}

func (controller *FPSController) showSticks() {
	for index := range controller.sticks {
		stick := controller.input.Sticks[index]
		base := controller.game.NewVector2D(engine.Circle(fps.StickRadius), engine.NewVec2(stick.X, stick.Y))
		base.SetFill(engine.RGBA(20, 30, 34, 160))
		base.SetStroke(engine.RGBA(217, 183, 113, 200), 2)
		base.SetLayer(20)
		thumb := controller.game.NewVector2D(engine.Circle(32), engine.NewVec2(stick.X, stick.Y))
		thumb.SetFill(engine.RGBA(217, 183, 113, 220))
		thumb.SetLayer(21)
		controller.sticks[index] = stickOverlay{base, thumb}
	}
}

func (controller *FPSController) drawSticks() {
	for index, overlay := range controller.sticks {
		if overlay.base == nil {
			continue
		}
		stick := controller.input.Sticks[index]
		base, thumb := engine.NewVec2(stick.X, stick.Y), engine.NewVec2(stick.X+stick.DX, stick.Y+stick.DY)
		if overlay.base.Position != base {
			overlay.base.Set(base, engine.NewVec2(1, 1), 0)
		}
		if overlay.thumb.Position != thumb {
			overlay.thumb.Set(thumb, engine.NewVec2(1, 1), 0)
		}
		alpha := float32(.35)
		if stick.ID != 0 {
			alpha = .8
		}
		if overlay.base.Alpha != alpha {
			overlay.base.SetAlpha(alpha)
			overlay.thumb.SetAlpha(alpha)
		}
	}
}
