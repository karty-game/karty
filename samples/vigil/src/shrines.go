package main

import (
	"math"

	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
)

type vigilShrines struct {
	game   *engine.Game
	level  engine.LevelHandle
	flames [3]*engine.Sprite3D
	lights [3]*engine.Light3D
	lit    [3]bool
}

func (s *vigilShrines) near(player *pilgrim) int {
	if player.hero == nil {
		return -1
	}
	for i, p := range dungeon.Shrines {
		if !s.lit[i] && dungeon.Distance(player.position(), p) < 1.65 && dungeon.Visible(player.position(), p) {
			return i
		}
	}
	return -1
}

func (s *vigilShrines) progress() int {
	n := 0
	for _, lit := range s.lit {
		if lit {
			n++
		}
	}
	return n
}

func (s *vigilShrines) animate(frame uint64) {
	// Only the three lit flames change texture, at authored flipbook cadence.
	if frame%7 == 0 {
		for i, lit := range s.lit {
			if lit {
				sprite := s.flames[i].Settings()
				sprite.Asset = engine.LevelTexture(s.level, uint32(flameTexture+int(frame/7)%3))
				s.flames[i].SetSprite(sprite)
			}
		}
	}
}

func (s *vigilShrines) reset() {
	for i := range s.lit {
		s.lit[i] = false
		if s.flames[i] != nil {
			s.flames[i].Destroy()
			s.flames[i] = nil
		}
		s.lights[i].SetEnabled(false)
	}
}

func (s *vigilShrines) light(i int, audio *vigilAudio) string {
	s.lit[i] = true
	p := dungeon.Shrines[i]
	t := engine.NewTransform3D(engine.NewVec3(float32(p.X), float32(p.Y), 1.205))
	s.flames[i] = s.game.NewSprite3D(engine.LevelTexture(s.level, flameTexture), t, engine.SpriteRenderer3D{
		Facing: engine.WorldSpriteFacingUpright, AlphaMode: engine.WorldSpriteAlphaBlend,
		Size: engine.NewVec2(.4, float32(.7/math.Cos(math.Pi/6))), Anchor: engine.NewVec2(.5, 1),
	})
	s.lights[i].SetEnabled(true)
	audio.lightShrine(i, p)
	status := []string{"A light for those we remember.", "A light for those who keep watch.", "A light against the coming night."}[i]
	if s.progress() == 3 {
		status = "The vigil is kept. Peace be with you."
	}
	return status
}

func (s *vigilShrines) createLights() {
	for i, p := range dungeon.Shrines {
		s.lights[i] = s.game.NewPointLight(s.level, engine.NewVec3(float32(p.X), float32(p.Y), 1.6), engine.NewVec3(1, .58, .22), 5)
		s.lights[i].SetEnabled(false)
	}
}
