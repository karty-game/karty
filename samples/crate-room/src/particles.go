package main

import (
	"math"

	"example.com/crate-room/.karty/engine"
)

// Level textures are numbered by sorted name: blast-smoke, then its fades.
const blastSmokeTexture, blastSmokeFadeTexture, blastSmokeFrames = 1, 2, 16

// The released SDK has no gameplay trigger for mounted GPU particles. This
// small game-owned emitter uses camera-facing world sprites.
// Four replaceable bursts cap the effect at 24 smoke puffs.
type roomParticles struct {
	bursts [4][6]roomParticle
	next   int
}

type roomParticle struct {
	sprite             *engine.Sprite3D
	position, velocity engine.Vec3
	age, life          uint8
	delay              uint8
	size               float32
}

func (system *roomParticles) burst(host *engine.Game, level engine.LevelHandle, position engine.Vec3) {
	burst := &system.bursts[system.next]
	system.next = (system.next + 1) % len(system.bursts)
	for i := range burst {
		particle := &burst[i]
		particle.close()
		angle := float64(i)*2.399963 + float64(system.next)*.7
		sin, cos := math.Sincos(angle)
		speed, rise, size := float32(1.1), .45+float32(i%3)*.1, 1.05+float32(i%2)*.25
		life := uint8(140 + i*5)
		if i == 0 {
			// One readable curled cloud carries the pop; tiny puffs drift away.
			speed, rise, size, life = .3, .35, 3.8, 132
		}
		*particle = roomParticle{
			position: position, velocity: engine.NewVec3(float32(sin)*speed, float32(cos)*speed, rise),
			life: life, size: size, delay: uint8(i * 2),
		}
		particle.sprite = host.NewSprite3D(
			engine.LevelTexture(level, blastSmokeTexture),
			engine.NewTransform3D(position),
			engine.SpriteRenderer3D{
				Facing: engine.WorldSpriteFacingCamera, AlphaMode: engine.WorldSpriteAlphaBlend,
				Size: engine.NewVec2(.05, .05), Anchor: engine.NewVec2(.5, .5),
			},
		)
	}
}

func (system *roomParticles) update(level engine.LevelHandle) {
	for b := range system.bursts {
		for i := range system.bursts[b] {
			particle := &system.bursts[b][i]
			if particle.sprite == nil {
				continue
			}
			if particle.delay > 0 {
				particle.delay--
				continue
			}
			particle.age++
			if particle.age >= particle.life {
				particle.close()
				continue
			}
			progress := float32(particle.age) / float32(particle.life)
			particle.position.X += particle.velocity.X / 60
			particle.position.Y += particle.velocity.Y / 60
			particle.position.Z = max(.15, particle.position.Z+particle.velocity.Z/60)
			// Keep expanding and travelling as opacity falls; never shrink away.
			grow := min(1, float32(particle.age)/8)
			size := particle.size * (.75 + progress*.9) * grow
			if err := particle.sprite.Ref().SetPose(engine.NewTransform3D(particle.position)); err != nil {
				panic(err)
			}
			particle.sprite.SetSprite(engine.SpriteRenderer3D{
				Asset: engine.LevelTexture(level, smokeTexture(progress)), Facing: engine.WorldSpriteFacingCamera,
				AlphaMode: engine.WorldSpriteAlphaBlend, Size: engine.NewVec2(size, size), Anchor: engine.NewVec2(.5, .5),
			})
		}
	}
}

// The released sprite API has no opacity field. These atlas regions retain the
// same artwork with progressively lower alpha, generated offline at 256px.
func smokeTexture(progress float32) uint32 {
	frame := min(blastSmokeFrames-1, int(max(0, (progress-.12)/.88)*blastSmokeFrames))
	if frame == 0 {
		return blastSmokeTexture
	}
	return uint32(blastSmokeFadeTexture + frame - 1)
}

func (particle *roomParticle) close() {
	if particle.sprite != nil {
		particle.sprite.Destroy()
	}
	*particle = roomParticle{}
}

func (system *roomParticles) clear() {
	for b := range system.bursts {
		for i := range system.bursts[b] {
			system.bursts[b][i].close()
		}
	}
	system.next = 0
}
