package main

import (
	"example.com/controllers-consumer/.karty/engine"
	fpscontroller "github.com/karty-game/controllers-3d"
	isometric "github.com/karty-game/controllers-isometric"
	topdown "github.com/karty-game/controllers-topdown"
)

// A fresh archive consumer deliberately exercises every preset with the
// generated SDK. It has no original source-folder dependency or level assets.
//
//go:wasmexport karty_register
func main() {
	fps, err := fpscontroller.NewFirstPerson(fpscontroller.DefaultFirstPersonOptions())
	if err != nil {
		panic(err)
	}
	top, err := topdown.NewTopDown2D(topdown.DefaultTopDown2DOptions())
	if err != nil {
		panic(err)
	}
	iso, err := isometric.NewIsometric(isometric.DefaultIsometricOptions())
	if err != nil {
		panic(err)
	}
	composer, err := topdown.NewCameraComposer(topdown.DefaultCameraFramingOptions())
	if err != nil {
		panic(err)
	}
	game := &engine.Game{}
	var marker *engine.Shape2D
	position, focus := topdown.Vec2{}, topdown.Vec2{}
	engine.Run(engine.Hooks{
		OnStart: func() {
			marker = game.NewShape2D(engine.Circle(12), engine.NewVec2(640, 360))
			marker.SetFill(engine.RGBA(220, 180, 100, 255))
		},
		OnKeyDown: func(k engine.Key) {
			if k == engine.KeyW {
				fps.Key(fpscontroller.Forward, true)
				top.Key(topdown.Forward, true)
				iso.Key(isometric.Forward, true)
			}
		},
		OnKeyUp: func(k engine.Key) {
			if k == engine.KeyW {
				fps.Key(fpscontroller.Forward, false)
				top.Key(topdown.Forward, false)
				iso.Key(isometric.Forward, false)
			}
		},
		OnInputReset: func() { fps.Reset(); top.Reset(); iso.Reset() },
		OnUpdate: func(engine.Frame) {
			motion := fps.Step(0, 0)
			a, b := top.Step(position), iso.Step(isometric.Vec2(position))
			position.X += a.X + b.X + float64(motion.X)
			position.Y += a.Y + b.Y + float64(motion.Y)
			focus, err = top.ComposeCamera(composer, focus, position, 1280, 720, false)
			if err != nil {
				panic(err)
			}
			camera := iso.CameraPosition(isometric.Vec2(focus))
			ground, ok := iso.ScreenToGround(isometric.Vec2{X: 640, Y: 360}, isometric.Vec2(focus), 1280, 720)
			if ok {
				marker.SetTransform(engine.NewVec2(float32(640+ground.X+camera.X), float32(360+ground.Y)), engine.NewVec2(1, 1), 0)
			}
		},
	})
}
