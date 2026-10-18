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

func (game *Game) acceptTorchPose(pose engine.Transform3D) {
	if game.torch == nil {
		return
	}
	aim := &game.torchAim
	yaw, pitch := float64(pose.Yaw), float64(pose.Pitch)
	if yaw == aim.cameraYaw && pitch == aim.cameraPitch {
		return
	}
	// A portal can rotate the accepted camera away from its proposed heading.
	// Carry the existing local lag through that correction just as the host
	// rotates the attached beam; its position continues to follow the host.
	lsy, lcy := math.Sincos(float64(game.torch.Settings.Yaw))
	lsp, lcp := math.Sincos(float64(game.torch.Settings.Pitch))
	sy, cy := math.Sincos(yaw)
	sp, cp := math.Sincos(pitch)
	x, y, z := lsy*lcp, lcy*lcp, lsp
	wx, wy, wz := cy*x+sy*cp*y-sy*sp*z, -sy*x+cy*cp*y-cy*sp*z, sp*y+cp*z
	aim.yaw = math.Atan2(wx, wy)
	aim.pitch = math.Atan2(wz, math.Hypot(wx, wy))
	aim.cameraYaw, aim.cameraPitch = yaw, pitch
}

func (game *Game) updateTorch(frame uint64) {
	if game.torch == nil || game.camera == nil || game.torchAim.updated && frame == game.torchAim.lastFrame {
		return
	}
	aim := &game.torchAim
	frames := uint64(1)
	if aim.updated && frame > aim.lastFrame {
		frames = frame - aim.lastFrame
	}
	aim.lastFrame, aim.updated = frame, true
	camera := game.camera.Settings
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
	settings := game.torch.Settings
	if settings.Yaw == localYaw && settings.Pitch == localPitch {
		return
	}
	settings.Yaw, settings.Pitch = localYaw, localPitch
	game.torch.SetSettings(settings)
}

// lightBolt demonstrates a moving point light independently of visible actors.
// Only one bolt exists at a time; this lighting study has no projectile collision.
type lightBolt struct {
	light     *engine.EntityLight3D
	position  engine.Vec3
	step      engine.Vec3
	remaining uint8
}

func (game *Game) toggleTorch() {
	game.torchEnabled = !game.torchEnabled
	if game.torch != nil {
		game.torch.SetEnabled(game.torchEnabled)
	}
	if game.panel != nil {
		game.panel.Invalidate()
	}
}

func (game *Game) launchLight() {
	if game.camera == nil {
		return
	}
	game.bolt.close()
	// Camera Settings holds the latest pose accepted by the host, including
	// collision corrections. The bolt leaves that eye in its viewing direction.
	settings := game.camera.Settings
	yaw, pitch := float64(settings.Yaw), float64(settings.Pitch)
	direction := engine.NewVec3(
		float32(math.Sin(yaw)*math.Cos(pitch)),
		float32(math.Cos(yaw)*math.Cos(pitch)),
		float32(math.Sin(pitch)),
	)
	position := settings.Position
	position.X += direction.X * .5
	position.Y += direction.Y * .5
	position.Z += direction.Z * .5
	game.bolt = lightBolt{
		light:     game.NewPointLight(game.level, position, engine.NewVec3(4, 1.2, .25), 5),
		position:  position,
		step:      engine.NewVec3(direction.X*.14, direction.Y*.14, direction.Z*.14),
		remaining: 90,
	}
}

func (bolt *lightBolt) update() {
	if bolt.light == nil {
		return
	}
	bolt.remaining--
	if bolt.remaining == 0 {
		bolt.close()
		return
	}
	bolt.position.X += bolt.step.X
	bolt.position.Y += bolt.step.Y
	bolt.position.Z += bolt.step.Z
	bolt.light.SetPose(bolt.position, 0, 0)
	if bolt.remaining < 20 {
		fade := float32(bolt.remaining) / 20
		bolt.light.SetColor(engine.NewVec3(4*fade, 1.2*fade, .25*fade))
	}
}

func (bolt *lightBolt) close() {
	if bolt.light != nil {
		bolt.light.Destroy()
	}
	*bolt = lightBolt{}
}

func (game *Game) clearLights() {
	game.bolt.close()
	if game.torch != nil {
		game.torch.Destroy()
		game.torch = nil
	}
	game.torchAim = lightAim{}
}
