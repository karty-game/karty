package main

import (
	"example.com/hangar/.karty/config"
	"example.com/hangar/.karty/engine"
	"example.com/hangar/internal/fps"
)

// FPSController owns desktop input, two independent touch sticks and their overlay.
// Camera collision and stair movement remain authoritative in the host.
type FPSController struct {
	game                *engine.Game
	camera              *engine.EntityCamera3D
	input               fps.Input
	captured, touchMode bool
	sticks              [2]stickOverlay
	SceneContains       func(x, y float32) bool
	OnTouchMode         func()
}

type stickOverlay struct{ base, thumb *engine.EntityVector2D }

func NewFPSController(game *engine.Game) *FPSController {
	return &FPSController{game: game, input: fps.New(float32(config.ResolutionWidth), float32(config.ResolutionHeight))}
}

func (controller *FPSController) SetCamera(camera *engine.EntityCamera3D) {
	controller.Release()
	controller.camera = camera
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
	if pointer.Kind == engine.PointerMouse {
		if pointer.Phase == engine.PointerDown && pointer.Buttons&1 != 0 && !controller.captured &&
			(controller.SceneContains == nil || controller.SceneContains(pointer.X, pointer.Y)) {
			controller.game.SetCursorCaptured(true)
			controller.captured = true
		}
		if pointer.Phase == engine.PointerMove && pointer.Captured {
			controller.input.Mouse(pointer.DeltaX, pointer.DeltaY)
		}
		if pointer.Phase == engine.PointerCancel {
			controller.Reset()
		}
		return
	}
	if pointer.Kind != engine.PointerTouch {
		return
	}
	switch pointer.Phase {
	case engine.PointerDown:
		if !controller.touchMode {
			controller.touchMode = true
			controller.showSticks()
			if controller.OnTouchMode != nil {
				controller.OnTouchMode()
			}
		}
		if pointer.Y < float32(config.ResolutionHeight)*.45 ||
			(controller.SceneContains != nil && !controller.SceneContains(pointer.X, pointer.Y)) {
			return
		}
		side := 0
		if pointer.X >= float32(config.ResolutionWidth)*.5 {
			side = 1
		}
		controller.input.BeginTouch(side, pointer.ID, pointer.X, pointer.Y)
	case engine.PointerMove:
		controller.input.MoveTouch(pointer.ID, pointer.X, pointer.Y)
	case engine.PointerUp, engine.PointerCancel:
		controller.input.EndTouch(pointer.ID)
	}
	controller.drawSticks()
}

func (controller *FPSController) Update(engine.Frame) {
	if controller.camera == nil {
		return
	}
	settings := controller.camera.Settings
	motion := controller.input.Step(settings.Yaw, settings.Pitch)
	if motion.X == 0 && motion.Y == 0 && motion.Yaw == settings.Yaw && motion.Pitch == settings.Pitch {
		return
	}
	settings.Position.X += motion.X
	settings.Position.Y += motion.Y
	settings.Yaw, settings.Pitch = motion.Yaw, motion.Pitch
	controller.camera.SetSettings(settings)
}

func (controller *FPSController) Reset() {
	controller.input.Reset()
	controller.captured = false
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
	if controller.touchMode {
		return "Left stick walk · Right stick look · Tap a view to explore"
	}
	return "WASD walk · Click scene for mouse look · Esc release · Shift run · V next view · H hide"
}

func (controller *FPSController) showSticks() {
	for index := range controller.sticks {
		stick := controller.input.Sticks[index]
		base := controller.game.NewVector2D(engine.Circle(fps.StickRadius), engine.NewVec2D(stick.X, stick.Y))
		base.SetFill(engine.RGBA(20, 30, 34, 160))
		base.SetStroke(engine.RGBA(217, 183, 113, 200), 2)
		base.SetLayer(20)
		thumb := controller.game.NewVector2D(engine.Circle(23), engine.NewVec2D(stick.X, stick.Y))
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
		base, thumb := engine.NewVec2D(stick.X, stick.Y), engine.NewVec2D(stick.X+stick.DX, stick.Y+stick.DY)
		if overlay.base.Position != base {
			overlay.base.Set(base, engine.NewVec2D(1, 1), 0)
		}
		if overlay.thumb.Position != thumb {
			overlay.thumb.Set(thumb, engine.NewVec2D(1, 1), 0)
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
