package main

import (
	"math"

	"example.com/vigil/.karty/assets"
	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
)

type spellCaster struct {
	game     *engine.Game
	level    engine.LevelHandle
	hud      *vigilHUD
	held     bool
	cooldown uint8
	bolts    [4]fireball
	next     int
}

const fireballImpactFrames = 24

// The fixed four-slot pool bounds each bolt to one light, three flame cards
// and six impact sparks. Reset/stop and slot replacement release them all.
type fireball struct {
	light     *engine.Light3D
	flame     [3]*engine.Sprite3D
	sparks    [6]*engine.Sprite3D
	position  dungeon.Point
	direction dungeon.Point
	facing    uint8
	age       uint8
	fading    uint8
	impact    uint8
}

// Reuse a pool slot before spawning, keeping repeated casts within the same resource budget.
func (s *spellCaster) throw(player *pilgrim, audio *vigilAudio) {
	if !s.ready(player) {
		return
	}
	bolt := &s.bolts[s.next]
	bolt.close()
	s.next = (s.next + 1) % len(s.bolts)
	s.cooldown = 18
	s.hud.refresh()
	bolt.position, bolt.direction = player.position(), player.aim
	bolt.facing = uint8(player.facing % dungeon.WalkDirections)
	// Begin just ahead of the body, retaining segment clearance at close walls.
	blocked := false
	for range 4 {
		p, moving := dungeon.FireballStep(bolt.position, bolt.direction)
		if !moving {
			blocked = true
			break
		}
		bolt.position = p
	}
	position := engine.NewVec3(float32(bolt.position.X), float32(bolt.position.Y), .95)
	bolt.light = s.game.NewPointLight(s.level, position, engine.NewVec3(3.1, .86, .12), 3.8)
	audio.play(assets.SoundFireballCast, bolt.position, .95, .34, 1, 12, 37)
	for i := range bolt.flame {
		size := float32(.44)
		texture := uint32(fireCoreTexture)
		if i > 0 {
			size = 1.2 - float32(i-1)*.3
			texture = uint32(fireTrailTexture) + uint32(bolt.facing)
		}
		bolt.flame[i] = s.game.NewSprite3D(engine.LevelTexture(s.level, texture), engine.NewTransform3D(position), engine.SpriteRenderer3D{
			Facing: engine.WorldSpriteFacingUpright, AlphaMode: engine.WorldSpriteAlphaBlend,
			Size: engine.NewVec2(size, size/float32(math.Cos(math.Pi/6))), Anchor: engine.NewVec2(.5, .5),
		})
	}
	if blocked {
		s.impact(bolt, audio)
	}
	bolt.draw()
}

func (s *spellCaster) impact(bolt *fireball, audio *vigilAudio) {
	bolt.impact = 1
	// A world anchor lets the short reverberant tail outlive the visual bolt.
	audio.play(assets.SoundFireballImpact, bolt.position, .95, .55, 1, 14, 55)
	for i := range bolt.sparks {
		bolt.sparks[i] = s.game.NewSprite3D(
			engine.LevelTexture(s.level, fireEmberTexture),
			engine.NewTransform3D(engine.NewVec3(float32(bolt.position.X), float32(bolt.position.Y), .95)),
			engine.SpriteRenderer3D{
				Facing: engine.WorldSpriteFacingUpright, AlphaMode: engine.WorldSpriteAlphaBlend,
				Size: engine.NewVec2(.2, float32(.2/math.Cos(math.Pi/6))), Anchor: engine.NewVec2(.5, .5),
			},
		)
	}
}

// Cooldown, travel and impact lifetimes advance in fixed engine updates.
func (s *spellCaster) update(audio *vigilAudio) {
	if s.cooldown > 0 {
		s.cooldown--
	}
	for i := range s.bolts {
		bolt := &s.bolts[i]
		if bolt.light == nil {
			continue
		}
		bolt.age++
		if bolt.impact > 0 {
			bolt.impact++
			if bolt.impact > fireballImpactFrames {
				bolt.close()
				continue
			}
		} else if bolt.fading == 0 {
			p, moving := dungeon.FireballStep(bolt.position, bolt.direction)
			bolt.position = p
			if !moving {
				s.impact(bolt, audio)
			} else if bolt.age >= 90 {
				bolt.fading = 12
			}
		} else {
			bolt.fading--
			if bolt.fading == 0 {
				bolt.close()
				continue
			}
		}
		bolt.draw()
	}
}

func (bolt *fireball) close() {
	if bolt.light != nil {
		bolt.light.Destroy()
	}
	for _, flame := range bolt.flame {
		if flame != nil {
			flame.Destroy()
		}
	}
	for _, spark := range bolt.sparks {
		if spark != nil {
			spark.Destroy()
		}
	}
	*bolt = fireball{}
}

func (s *spellCaster) clear() {
	for i := range s.bolts {
		s.bolts[i].close()
	}
	s.cooldown, s.next = 0, 0
}

func (s *spellCaster) ready(player *pilgrim) bool {
	return player.hero != nil && !player.resetPending && s.cooldown == 0
}
