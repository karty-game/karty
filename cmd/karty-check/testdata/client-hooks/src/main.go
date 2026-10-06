package main

import "example.com/client-hooks-check/.karty/engine"

var game engine.Game
var camera *engine.EntityCamera3D
var mounted int
var level engine.LevelHandle
var completed bool
var actor engine.WorldActorRef
var inputMoved bool

//go:wasmexport karty_register
func main() {
	engine.Run(engine.Hooks{
		OnStart: func() { game.RequestLevel("levels.hooks") },
		OnLevelReady: func(event engine.LevelReady) {
			mounted++
			level = event.Handle
			camera = game.NewCamera3D(level, engine.CameraSettings{
				Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
				Position: engine.NewVec3D(2, 1, 1), FOVY: 1, OrthoHeight: 4, Near: .05, Far: 10,
				ViewportWidth: 160, ViewportHeight: 120,
			})
			// Transform acceptance belongs to the hook contract; material rendering
			// and baking have their own small GPU fixtures.
			camera.SetAlpha(0)
			if _, err := game.WatchTransform(camera, func(pose engine.Transform3D) {
				if camera.Settings.Position != pose.Position {
					panic("unsynchronized transform hook")
				}
				if inputMoved && pose.Position.X == 2.25 {
					actor.SetTags("transform-hook")
					inputMoved = false
				}
			}); err != nil {
				panic(err)
			}
			if _, err := StartAuthoredSequence(level, "levels.hooks", "verify"); err != nil {
				panic(err)
			}
		},
		OnUpdate: func(engine.Frame) {
			if completed && mounted == 1 {
				completed = false
				camera.Destroy()
				camera = nil
				game.ReleaseLevel(level)
			}
		},
		OnLevelReleased: func(engine.LevelReleased) { game.RequestLevel("levels.hooks") },
		OnKeyDown: func(key engine.Key) {
			if key == engine.KeyA && mounted == 2 && completed {
				actor.SetTags("key-hook")
				position := camera.Settings.Position
				position.X += .25
				inputMoved = true
				camera.SetPose(position, 0, 0)
			}
		},
		OnKeyUp: func(key engine.Key) {
			if key == engine.KeyA && completed {
				actor.SetTags("key-up-hook")
			}
		},
		OnPointerReleased: func(engine.PointerEvent) {
			if completed {
				actor.SetTags("pointer-hook")
			}
		},
		OnLevelFailed:    func(engine.LevelFailed) { panic("fixture level failed") },
		OnSequenceFailed: func(engine.SequenceFailure) { panic("fixture sequence failed") },
	})
}

//karty:condition fixture.ready
func ready(context engine.ActionContext, actor engine.WorldActorRef) (bool, error) {
	return actor.ID != 0, nil
}

//karty:action fixture.complete
func complete(context engine.ActionContext, target engine.WorldActorRef) error {
	actor = target
	if mounted == 1 {
		actor.SetTags("first-mount")
	} else {
		actor.SetTags("second-mount")
	}
	completed = true
	return nil
}
