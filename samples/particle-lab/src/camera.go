package main

import (
	"math"

	"example.com/particle-lab/.karty/config"
	"example.com/particle-lab/.karty/engine"
)

type effectCamera struct {
	entity        *engine.Camera3D
	keys          [256]bool
	pointers      [2]cameraPointer
	mouseDragging bool
}

func axis(positive, negative bool) float32 {
	var result float32
	if positive {
		result++
	}
	if negative {
		result--
	}
	return result
}

// update moves through the room on fixed ticks and leaves idle frames command-free.
func (camera *effectCamera) update(engine.Frame) {
	if camera.entity == nil {
		return
	}
	forward := axis(camera.keys[engine.KeyW], camera.keys[engine.KeyS])
	strafe := axis(camera.keys[engine.KeyD], camera.keys[engine.KeyA])
	turn := axis(camera.keys[engine.KeyRight], camera.keys[engine.KeyLeft])
	tilt := axis(camera.keys[engine.KeyArrowUp], camera.keys[engine.KeyArrowDown])
	if forward == 0 && strafe == 0 && turn == 0 && tilt == 0 {
		return
	}
	settings := camera.entity.Settings()
	settings.Yaw += turn * 0.025
	settings.Pitch = min(max(settings.Pitch+tilt*0.025, -1.4), 1.4)
	if length := math.Hypot(float64(forward), float64(strafe)); length > 1 {
		forward /= float32(length)
		strafe /= float32(length)
	}
	sine, cosine := float32(math.Sin(float64(settings.Yaw))), float32(math.Cos(float64(settings.Yaw)))
	settings.Position.X += (forward*sine + strafe*cosine) * 0.045
	settings.Position.Y += (forward*cosine - strafe*sine) * 0.045
	camera.entity.SetSettings(settings)
}

// rotate preserves the distance to the selected effect while orbiting around it.
func (camera *effectCamera) rotate(view int, amount float32) {
	if camera.entity == nil {
		return
	}
	target := engine.Vec2{}
	if view >= 0 && view < len(viewpoints) {
		target = viewpoints[view].target
	}
	x, y := target.X, target.Y
	settings := camera.entity.Settings()
	radius := math.Hypot(float64(settings.Position.X-x), float64(settings.Position.Y-y))
	settings.Yaw += amount
	settings.Position.X = x - float32(math.Sin(float64(settings.Yaw))*radius)
	settings.Position.Y = y - float32(math.Cos(float64(settings.Yaw))*radius)
	camera.entity.SetSettings(settings)
}

// zoom changes the lens, keeping the camera clear of the particle stations.
func (camera *effectCamera) zoom(factor float32) {
	if camera.entity == nil || factor <= 0 {
		return
	}
	settings := camera.entity.Settings()
	settings.VerticalFOV = min(max(settings.VerticalFOV*factor, .38), 1.45)
	camera.entity.SetSettings(settings)
}

// visit replaces the old camera with the station's authored inspection pose.
func (camera *effectCamera) visit(host *engine.Game, level engine.LevelHandle, index int) {
	if camera.entity != nil {
		camera.entity.Destroy()
	}
	pose := viewpoints[index]
	camera.entity = host.NewCamera3D(level, engine.Camera3DSettings{
		Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
		Position: pose.position, Yaw: pose.yaw, Pitch: pose.pitch,
		VerticalFOV: config.CameraFOVY, NearClip: config.CameraNear, FarClip: config.CameraFar,
		OrthoHeight:   config.CameraOrthoHeight,
		ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	})
	camera.entity.SetLayer(-10)
}
