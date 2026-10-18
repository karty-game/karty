package main

import (
	"example.com/controllers-consumer/.karty/engine"
	"github.com/karty-game/controllers"
)

// A fresh archive consumer deliberately exercises every preset with the
// generated SDK. It has no original source-folder dependency or level assets.
//
//go:wasmexport karty_register
func main() {
	fps, err := controllers.NewFirstPerson(controllers.DefaultFirstPersonOptions())
	if err != nil {
		panic(err)
	}
	top, err := controllers.NewTopDown2D(controllers.DefaultTopDown2DOptions())
	if err != nil {
		panic(err)
	}
	iso, err := controllers.NewIsometric(controllers.DefaultIsometricOptions())
	if err != nil {
		panic(err)
	}
	game := &engine.Game{}
	var marker *engine.EntityVector2D
	position, focus := controllers.Vec2{}, controllers.Vec2{}
	engine.Run(engine.Hooks{
		OnStart: func() {
			marker = game.NewVector2D(engine.Circle(12), engine.NewVec2(640, 360))
			marker.SetFill(engine.RGBA(220, 180, 100, 255))
		},
		OnKeyDown: func(k engine.Key) {
			if k == engine.KeyW {
				fps.Key(controllers.Forward, true)
				top.Key(controllers.Forward, true)
				iso.Key(controllers.Forward, true)
			}
		},
		OnKeyUp: func(k engine.Key) {
			if k == engine.KeyW {
				fps.Key(controllers.Forward, false)
				top.Key(controllers.Forward, false)
				iso.Key(controllers.Forward, false)
			}
		},
		OnInputReset: func() { fps.Reset(); top.Reset(); iso.Reset() },
		OnUpdate: func(engine.Frame) {
			motion := fps.Step(0, 0)
			a, b := top.Step(position), iso.Step(position)
			position.X += a.X + b.X + float64(motion.X)
			position.Y += a.Y + b.Y + float64(motion.Y)
			focus = top.Follow(focus, position, false)
			camera := iso.CameraPosition(focus)
			ground, ok := iso.ScreenToGround(controllers.Vec2{X: 640, Y: 360}, focus, 1280, 720)
			if ok {
				marker.Set(engine.NewVec2(float32(640+ground.X+camera.X), float32(360+ground.Y)), engine.NewVec2(1, 1), 0)
			}
		},
	})
}
