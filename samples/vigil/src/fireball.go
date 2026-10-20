package main

import (
	"math"

	"example.com/vigil/.karty/assets"
	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
)

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

func (g *Game) throwFireball() {
	if g.hero == nil || g.resetPending || g.fireCooldown != 0 {
		return
	}
	bolt := &g.fireballs[g.nextFireball]
	bolt.close()
	g.nextFireball = (g.nextFireball + 1) % len(g.fireballs)
	g.fireCooldown = 18
	g.refresh()
	bolt.position, bolt.direction = g.position(), g.aim
	bolt.facing = uint8(g.facing % dungeon.WalkDirections)
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
	bolt.light = g.NewPointLight(g.level, position, engine.NewVec3(3.1, .86, .12), 3.8)
	g.playWorldSound(assets.SoundFireballCast, bolt.position, .95, .34, 1, 12, 37)
	for i := range bolt.flame {
		size := float32(.44)
		texture := uint32(fireCoreTexture)
		if i > 0 {
			size = 1.2 - float32(i-1)*.3
			texture = uint32(fireTrailTexture) + uint32(bolt.facing)
		}
		bolt.flame[i] = g.NewSprite3D(engine.LevelTexture(g.level, texture), engine.NewTransform3D(position), engine.SpriteRenderer3D{
			Facing: engine.WorldSpriteFacingUpright, AlphaMode: engine.WorldSpriteAlphaBlend,
			Size: engine.NewVec2(size, size/float32(math.Cos(math.Pi/6))), Anchor: engine.NewVec2(.5, .5),
		})
	}
	if blocked {
		g.impactFireball(bolt)
	}
	bolt.draw()
}

func (g *Game) impactFireball(bolt *fireball) {
	bolt.impact = 1
	// A world anchor lets the short reverberant tail outlive the visual bolt.
	g.playWorldSound(assets.SoundFireballImpact, bolt.position, .95, .55, 1, 14, 55)
	for i := range bolt.sparks {
		bolt.sparks[i] = g.NewSprite3D(
			engine.LevelTexture(g.level, fireEmberTexture),
			engine.NewTransform3D(engine.NewVec3(float32(bolt.position.X), float32(bolt.position.Y), .95)),
			engine.SpriteRenderer3D{
				Facing: engine.WorldSpriteFacingUpright, AlphaMode: engine.WorldSpriteAlphaBlend,
				Size: engine.NewVec2(.2, float32(.2/math.Cos(math.Pi/6))), Anchor: engine.NewVec2(.5, .5),
			},
		)
	}
}

func (g *Game) updateFireballs() {
	if g.fireCooldown > 0 {
		g.fireCooldown--
	}
	for i := range g.fireballs {
		bolt := &g.fireballs[i]
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
				g.impactFireball(bolt)
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

func (bolt *fireball) draw() {
	if bolt.impact > 0 {
		bolt.drawImpact()
		return
	}
	fade := float32(1)
	if bolt.fading > 0 {
		fade = float32(bolt.fading) / 12
	}
	settings := bolt.light.Settings()
	settings.Position = engine.NewVec3(float32(bolt.position.X), float32(bolt.position.Y), .95)
	flicker := float32(.96 + .04*math.Sin(float64(bolt.age)*1.7))
	settings.Color = engine.NewVec3(3.1*fade*flicker, .86*fade*flicker, .12*fade)
	bolt.light.SetSettings(settings)
	for i, flame := range bolt.flame {
		// Directional silhouettes overlap into a single taper rather than a row
		// of independent candle flames. The actual planes remain upright.
		distance := float64(i) * .10
		p := engine.NewVec3(float32(bolt.position.X-bolt.direction.X*distance), float32(bolt.position.Y-bolt.direction.Y*distance), .95)
		if err := flame.Ref().SetPose(engine.NewTransform3D(p)); err != nil {
			panic(err)
		}
		s := flame.Settings()
		size := .44 * fade * flicker
		if i > 0 {
			size = (1.2 - float32(i-1)*.3) * fade * (2 - flicker)
		}
		s.Size = engine.NewVec2(size, size/float32(math.Cos(math.Pi/6)))
		flame.SetSprite(s)
	}
}

func (bolt *fireball) drawImpact() {
	t := float64(bolt.impact-1) / fireballImpactFrames
	fade := 1 - t
	settings := bolt.light.Settings()
	settings.Position = engine.NewVec3(float32(bolt.position.X), float32(bolt.position.Y), .95)
	// A brief hotter, wider flash distinguishes impact from ordinary expiry.
	pulse := float32(2.1 * fade * fade * fade)
	settings.Color = engine.NewVec3(3.1*pulse, 1.05*pulse, .19*pulse)
	settings.Radius = float32(3.8 + 1.3*fade*fade)
	bolt.light.SetSettings(settings)
	for i, flame := range bolt.flame {
		size := float32(.001)
		texture := uint32(fireFlashTexture)
		x, y, z := bolt.position.X, bolt.position.Y, .95
		if i == 0 && bolt.impact <= 10 {
			// Expand in three frames, then collapse; smoke supplies the tail.
			flash := float64(bolt.impact-1) / 9
			size = float32((.45 + 2.2*(1-math.Exp(-12*flash))) * math.Pow(1-flash, 1.5))
		} else if i > 0 && bolt.impact >= 6 {
			smoke := float64(bolt.impact-6) / (fireballImpactFrames - 6)
			size = float32(.48 + .88*smoke)
			texture = uint32(fireSmokeTexture)
			if bolt.impact >= 17 {
				texture++
			}
			side := float64(i*2 - 3)
			x += side * (.10 + .16*smoke)
			y += .10 * smoke
			z += .52*smoke + float64(i-1)*.11
		}
		size = max(.001, size)
		height := size / float32(math.Cos(math.Pi/6))
		p := engine.NewVec3(float32(x), float32(y), max(float32(z), height*.5+.04))
		if err := flame.Ref().SetPose(engine.NewTransform3D(p)); err != nil {
			panic(err)
		}
		s := flame.Settings()
		s.Asset = engine.LevelTexture(bolt.light.Level(), texture)
		s.Size = engine.NewVec2(size, height)
		flame.SetSprite(s)
	}
	for i, spark := range bolt.sparks {
		angle := float64(i)*2*math.Pi/float64(len(bolt.sparks)) + math.Atan2(bolt.direction.Y, bolt.direction.X) + float64(i%2)*.21
		distance := .08 + (1.05+float64(i%3)*.23)*(1-math.Exp(-3*t))
		life := max(0, 1-float64(bolt.impact-1)/float64(14+i%3*3))
		size := max(float32(.001), float32((.11+float64(i%2)*.025)*math.Sqrt(life)))
		height := size / float32(math.Cos(math.Pi/6))
		z := max(float32(.95+.40*math.Sin(math.Pi*t)-.7*t), height*.5+.04)
		p := engine.NewVec3(float32(bolt.position.X+math.Cos(angle)*distance), float32(bolt.position.Y+math.Sin(angle)*distance), z)
		if err := spark.Ref().SetPose(engine.NewTransform3D(p)); err != nil {
			panic(err)
		}
		s := spark.Settings()
		s.Size = engine.NewVec2(size, height)
		spark.SetSprite(s)
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

func (g *Game) clearFireballs() {
	for i := range g.fireballs {
		g.fireballs[i].close()
	}
	g.fireCooldown, g.nextFireball = 0, 0
}
