package main

import (
	"example.com/stdlib-precision/.karty/engine"
	"github.com/karty-game/stdlib-spike/controls"
	"github.com/karty-game/stdlib-spike/fps"
)

const profileName = "precision"

func controllerOptions() fps.Options {
	options := fps.DefaultOptions()
	options.Speed = 1.65
	options.LookSensitivity = .005
	return options
}
func bindings() map[engine.Key]controls.Action {
	return map[engine.Key]controls.Action{
		engine.KeyArrowUp: controls.Forward, engine.KeyArrowDown: controls.Backward,
		engine.KeyA: controls.Left, engine.KeyD: controls.Right,
		engine.KeyLeft: controls.TurnLeft, engine.KeyRight: controls.TurnRight,
		engine.KeyShiftLeft: controls.Run, engine.KeyShiftRight: controls.Run,
	}
}

// This game's precision mode disables strafing without changing library source.
func customize(intent *fps.Intent) { intent.Strafe = 0 }
