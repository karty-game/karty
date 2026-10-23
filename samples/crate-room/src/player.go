package main

import (
	"example.com/crate-room/.karty/config"
	"example.com/crate-room/.karty/engine"
	fpscontroller "example.com/crate-room/.karty/processed/code/controllers-3d.sdk/controller"
)

type roomPlayer struct {
	input      roomInput
	gun        rayGun
	camera     *engine.Camera3D
	controller *fpscontroller.FirstPersonController
}

// create attaches the shared character motor for slopes, steps and crate pushing.
func (player *roomPlayer) create(host *engine.Game, level engine.LevelHandle) {
	player.camera = host.NewCamera3D(
		level,
		engine.Camera3DSettings{
			Projection:     engine.CameraProjectionPerspective,
			Output:         engine.CameraOutputFull,
			Position:       engine.NewVec3(.2, -10, 1.67),
			VerticalFOV:    config.CameraFOVY,
			OrthoHeight:    config.CameraOrthoHeight,
			NearClip:       config.CameraNear,
			FarClip:        config.CameraFar,
			ViewportWidth:  config.ResolutionWidth,
			ViewportHeight: config.ResolutionHeight,
		},
	)
	options := fpscontroller.DefaultFirstPersonOptions()
	options.Width, options.Height = width, height
	options.ArrowKeysLook, options.TurnKeys = true, false
	options.Speed, options.RunMultiplier = 4.5, 7.0/4.5
	options.KeyYawSpeed, options.KeyPitchSpeed = 2.1, 1.5
	options.MouseSensitivity, options.MinPitch, options.MaxPitch = .006, -1.2, 1.2
	options.Touch.StickRadius, options.Touch.Deadzone = 65, 0
	options.Touch.InsetX, options.Touch.InsetY = 110, 104
	options.Touch.BaseRadius, options.Touch.ThumbRadius = 86, 25
	options.Touch.StartYFraction = 0
	options.Touch.AlwaysVisible, options.Touch.FixedMove, options.Touch.LookDrag = true, true, true
	options.Touch.HideLookThumb = true
	options.Touch.FillColor = engine.RGBA(12, 24, 35, 165)
	options.Touch.StrokeColor = engine.RGBA(225, 184, 110, 190)
	options.Touch.ThumbColor = engine.RGBA(233, 191, 113, 210)
	options.Touch.StrokeWidth, options.Touch.Layer = 2, 30
	options.Touch.IdleAlpha, options.Touch.ActiveAlpha = 1, 1
	options.Body = playerBody()
	options.SceneContains = func(x, y float32) bool { return !shootContains(x, y) }
	options.OnReset = player.input.reset
	options.Move = player.move
	var err error
	player.controller, err = fpscontroller.AttachFirstPerson(*host, player.camera, options)
	if err != nil {
		panic(err)
	}
}

// A queued reset must run before movement or attacks can affect the new round.
func (player *roomPlayer) move(controller *fpscontroller.FirstPersonController, motion fpscontroller.FirstPersonMotion) error {
	if player.input.pendingReset {
		return nil
	}
	return controller.ApplyMotion(motion)
}

// Teleport clears momentum without changing the gun's cooldown.
func (player *roomPlayer) reset() {
	if err := player.controller.Teleport(engine.NewVec3(.2, -10, 1.67), 0, 0); err != nil {
		panic(err)
	}
}

func playerBody() engine.BodySettings {
	return engine.BodySettings{
		Kind:          engine.BodyCharacter,
		Collider:      engine.CapsuleCollider{Radius: .3, Height: 1.8, Offset: engine.NewVec3(0, 0, -.75)},
		AllowRotation: false,
		Forces:        engine.BodyForceSettings{GravityScale: 1, Buoyancy: true},
		Mass:          80,
	}
}
