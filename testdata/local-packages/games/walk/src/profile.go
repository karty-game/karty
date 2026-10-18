package main

import (
	"example.com/stdlib-walk/.karty/engine"
	"github.com/karty-game/stdlib-spike/controls"
	"github.com/karty-game/stdlib-spike/fps"
)

const profileName = "walk"

func controllerOptions() fps.Options { return fps.DefaultOptions() }
func bindings() map[engine.Key]controls.Action {
	return map[engine.Key]controls.Action{
		engine.KeyW: controls.Forward, engine.KeyS: controls.Backward,
		engine.KeyA: controls.Left, engine.KeyD: controls.Right,
		engine.KeyLeft: controls.TurnLeft, engine.KeyRight: controls.TurnRight,
		engine.KeyArrowUp: controls.LookUp, engine.KeyArrowDown: controls.LookDown,
		engine.KeyShiftLeft: controls.Run, engine.KeyShiftRight: controls.Run,
	}
}
func customize(*fps.Intent) {}
