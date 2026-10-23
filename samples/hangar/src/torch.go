package main

import (
	"math"

	"example.com/hangar/.karty/engine"
)

// lightAim eases the beam toward camera rotation with a quarter-second time
// constant. Frames advance at the engine's fixed 60 Hz, including after gaps.
type lightAim struct {
	yaw, pitch             float64
	cameraYaw, cameraPitch float64
	lastFrame              uint64
	updated                bool
}

// Keep the beam's local lag when a portal changes the accepted camera rotation.
func (tour *hangarTour) acceptTorchPose(pose engine.Transform3D) {
	if tour.torch == nil {
		return
	}
	aim := &tour.torchAim
	yaw, pitch := float64(pose.Yaw), float64(pose.Pitch)
	if yaw == aim.cameraYaw && pitch == aim.cameraPitch {
		return
	}
	// A portal can rotate the accepted camera away from its proposed heading.
	// Carry the existing local lag through that correction just as the host
	// rotates the attached beam; its position continues to follow the host.
	lsy, lcy := math.Sincos(float64(tour.torch.Settings().Yaw))
	lsp, lcp := math.Sincos(float64(tour.torch.Settings().Pitch))
	sy, cy := math.Sincos(yaw)
	sp, cp := math.Sincos(pitch)
	x, y, z := lsy*lcp, lcy*lcp, lsp
	wx, wy, wz := cy*x+sy*cp*y-sy*sp*z, -sy*x+cy*cp*y-cy*sp*z, sp*y+cp*z
	aim.yaw = math.Atan2(wx, wy)
	aim.pitch = math.Atan2(wz, math.Hypot(wx, wy))
	aim.cameraYaw, aim.cameraPitch = yaw, pitch
}

// Ease world aim toward the camera, then express it in the attached light's local axes.
func (tour *hangarTour) updateTorch(frame uint64) {
	if tour.torch == nil || tour.camera == nil || tour.torchAim.updated && frame == tour.torchAim.lastFrame {
		return
	}
	aim := &tour.torchAim
	frames := uint64(1)
	if aim.updated && frame > aim.lastFrame {
		frames = frame - aim.lastFrame
	}
	aim.lastFrame, aim.updated = frame, true
	camera := tour.camera.Settings()
	yaw, pitch := float64(camera.Yaw), float64(camera.Pitch)
	aim.cameraYaw, aim.cameraPitch = yaw, pitch
	deltaYaw := math.Remainder(yaw-aim.yaw, 2*math.Pi)
	deltaPitch := pitch - aim.pitch
	blend := 1 - math.Exp(-float64(frames)/(60*.25))
	aim.yaw = math.Remainder(aim.yaw+deltaYaw*blend, 2*math.Pi)
	aim.pitch += deltaPitch * blend
	if math.Abs(math.Remainder(yaw-aim.yaw, 2*math.Pi)) < .00001 {
		aim.yaw = math.Remainder(yaw, 2*math.Pi)
	}
	if math.Abs(pitch-aim.pitch) < .00001 {
		aim.pitch = pitch
	}

	// The camera rotates the attached light's local direction using its full
	// right/forward/up basis. Invert that basis to keep the eased world aim;
	// subtracting pitch alone is incorrect when yaw also differs.
	delta := math.Remainder(aim.yaw-yaw, 2*math.Pi)
	sx, cx := math.Sincos(delta)
	sp, cp := math.Sincos(aim.pitch)
	sc, cc := math.Sincos(pitch)
	x, y, z := sx*cp, cx*cp*cc+sp*sc, -cx*cp*sc+sp*cc
	localYaw := float32(math.Atan2(x, y))
	localPitch := float32(max(-math.Pi/2+.00001, min(math.Pi/2-.00001, math.Atan2(z, math.Hypot(x, y)))))
	if delta == 0 && aim.pitch == pitch {
		localYaw, localPitch = 0, 0
	}
	settings := tour.torch.Settings()
	if settings.Yaw == localYaw && settings.Pitch == localPitch {
		return
	}
	settings.Yaw, settings.Pitch = localYaw, localPitch
	tour.torch.SetSettings(settings)
}

func (tour *hangarTour) toggleTorch() {
	tour.torchEnabled = !tour.torchEnabled
	if tour.torch != nil {
		tour.torch.SetEnabled(tour.torchEnabled)
	}
}

func (tour *hangarTour) clearLights() {
	tour.bolt.close()
	if tour.torch != nil {
		tour.torch.Destroy()
		tour.torch = nil
	}
	tour.torchAim = lightAim{}
}

func (tour *hangarTour) createTorch(host *engine.Game, level engine.LevelHandle, view viewpoint) {
	tour.torch = host.NewSpotLight(level, engine.NewVec3(view.x, view.y, view.z), engine.NewVec3(3, 2.7, 2.16), 18, .25, .50)
	// Camera-local axes are right, forward and up: hold the torch low on the right.
	tour.torch.AttachTo(tour.camera.Ref(), engine.NewVec3(.35, .20, -.30))
	tour.torch.SetEnabled(tour.torchEnabled)
	// Spotlight shadow maps require an ordinary Euclidean world. The loop
	// keeps the live beam while leaving its shadows disabled.
	tour.torch.SetShadows(false)
	tour.torchAim = lightAim{
		yaw: float64(view.yaw), pitch: float64(view.pitch),
		cameraYaw: float64(view.yaw), cameraPitch: float64(view.pitch),
	}
}
