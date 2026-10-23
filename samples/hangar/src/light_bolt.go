package main

import (
	"math"

	"example.com/hangar/.karty/engine"
)

// lightBolt demonstrates a moving point light independently of visible actors.
// Only one bolt exists at a time; this lighting study has no projectile collision.
type lightBolt struct {
	light     *engine.Light3D
	position  engine.Vec3
	step      engine.Vec3
	remaining uint8
}

// Replace the previous bolt and launch from the camera's accepted eye position.
func (bolt *lightBolt) launch(host *engine.Game, level engine.LevelHandle, camera *engine.Camera3D) {
	if camera == nil {
		return
	}
	bolt.close()
	// Camera Settings holds the latest pose accepted by the host, including
	// collision corrections. The bolt leaves that eye in its viewing direction.
	settings := camera.Settings()
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
	*bolt = lightBolt{
		light:     host.NewPointLight(level, position, engine.NewVec3(4, 1.2, .25), 5),
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
