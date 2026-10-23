package main

import (
	"math"

	"example.com/crate-room/.karty/assets"
	"example.com/crate-room/.karty/engine"
)

type rayGun struct {
	cooldown      uint8
	shots, blasts uint64
	pending       [4]engine.RequestID
}

// fire aims from the camera and excludes the player from its own impulse.
func (gun *rayGun) fire(host *engine.Game, level engine.LevelHandle, camera *engine.Camera3D, blast bool) {
	settings := camera.Settings()
	yawSin, yawCos := math.Sincos(float64(settings.Yaw))
	pitchSin, pitchCos := math.Sincos(float64(settings.Pitch))
	ray := engine.RayImpulse{
		Origin:    settings.Position,
		Direction: engine.NewVec3(float32(yawSin*pitchCos), float32(yawCos*pitchCos), float32(pitchSin)),
		Distance:  60,
		Impulse:   95,
		Exclude:   camera.Ref(),
	}
	if blast {
		ray.Impulse = 260
		ray.ExplosionRadius = 5
		gun.blasts++
		for i, request := range gun.pending {
			if request != 0 {
				continue
			}
			var err error
			gun.pending[i], err = host.World(level).Raycast(engine.RaycastQuery{
				Origin: ray.Origin, Direction: ray.Direction, Distance: ray.Distance, Exclude: ray.Exclude,
			})
			if err != nil {
				panic(err)
			}
			break
		}
	} else {
		gun.shots++
		host.PlaySound(assets.SoundShoot)
	}
	if err := host.World(level).ApplyRayImpulse(ray); err != nil {
		panic(err)
	}
}

func (gun *rayGun) acceptHit(result engine.RaycastResult) bool {
	for i, request := range gun.pending {
		if request == 0 || request != result.RequestID {
			continue
		}
		gun.pending[i] = 0
		return result.Hit
	}
	return false
}

// update keeps held fire on the fixed cooldown while blasts remain immediate.
func (gun *rayGun) update(input *roomInput, fire func(bool)) {
	if gun.cooldown > 0 {
		gun.cooldown--
	}
	if (input.pendingShot || input.mouseHeld || input.shootID != 0 || input.spaceHeld) && gun.cooldown == 0 {
		fire(false)
		gun.cooldown = 8
		input.pendingShot = false
	}
	if input.pendingBlast {
		input.pendingBlast = false
		fire(true)
	}
}
