package main

import (
	"example.com/effect-lab/.karty/config"
	"example.com/effect-lab/.karty/engine"
	fpscontroller "example.com/effect-lab/.karty/processed/code/controllers-3d.sdk/controller"
)

// cameraRig owns the camera and the controls that move it.
type cameraRig struct {
	entity     *engine.Camera3D
	controller *fpscontroller.FirstPersonController
	keys       [256]bool
	view       int
}

// mount sets the shared projection before the first authored viewpoint is selected.
func (rig *cameraRig) mount(host *engine.Game, level engine.LevelHandle) {
	rig.entity = host.NewCamera3D(level, engine.Camera3DSettings{
		Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
		Position:    engine.NewVec3(0, -9, 1.65),
		VerticalFOV: config.CameraFOVY, NearClip: config.CameraNear, FarClip: config.CameraFar,
		OrthoHeight:   config.CameraOrthoHeight,
		ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	})
	rig.entity.SetLayer(-10)
}

// visit teleports by replacing the camera, avoiding a collision sweep between views.
func (rig *cameraRig) visit(host *engine.Game, level engine.LevelHandle, index int) string {
	if rig.entity == nil {
		return ""
	}
	rig.resetKeys()
	poses := [...]struct {
		position   engine.Vec3
		yaw, pitch float32
		name       string
	}{
		{engine.NewVec3(0, -9, 1.65), 0, -.025, "EFFECT LAB / overview"},
		{engine.NewVec3(0, 4.5, 1.65), 0, 0, "EFFECT LAB / grayscale and color ramps"},
		{engine.NewVec3(-6.8, -6.2, 1.65), .4, -.05, "EFFECT LAB / materials and colored light"},
	}
	rig.view = index
	pose := poses[index]
	settings := rig.entity.Settings()
	settings.Position, settings.Yaw, settings.Pitch = pose.position, pose.yaw, pose.pitch
	rig.entity.Destroy()
	rig.entity = host.NewCamera3D(level, settings)
	rig.entity.SetLayer(-10)
	options := fpscontroller.DefaultFirstPersonOptions()
	options.Mode = fpscontroller.FirstPersonFree
	options.Width, options.Height, options.Speed = float32(config.ResolutionWidth), float32(config.ResolutionHeight), 3.5
	options.ArrowKeysLook, options.TurnKeys = true, false
	options.Touch.StickRadius, options.Touch.BaseRadius, options.Touch.ThumbRadius = 96, 96, 32
	options.SceneContains = func(x, y float32) bool { return y > 125 }
	options.OnReset = rig.resetKeys
	var err error
	rig.controller, err = fpscontroller.AttachFirstPerson(*host, rig.entity, options)
	if err != nil {
		panic(err)
	}
	return pose.name
}

// Only scene shortcuts need sample-owned key edges; the package owns navigation.
func (rig *cameraRig) pressed(key engine.Key, down bool) bool {
	if int(key) >= len(rig.keys) {
		return false
	}
	held := rig.keys[key]
	rig.keys[key] = down
	return down && !held
}

func (rig *cameraRig) resetKeys() { rig.keys = [256]bool{} }
