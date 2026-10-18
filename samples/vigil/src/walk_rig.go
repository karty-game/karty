package main

import (
	"math"

	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
)

func (g *Game) createMovementRoot() bool {
	pose := engine.NewTransform3D(engine.NewVec3(float32(dungeon.Spawn.X), float32(dungeon.Spawn.Y), .025))
	pose.Pitch = math.Pi / 2
	g.shadow = g.NewWorldSprite(engine.LevelTexture(g.level, shadowTexture), pose, engine.WorldSprite{
		Facing: engine.WorldSpriteFacingFixed, Alpha: engine.WorldSpriteAlphaBlend,
		Size: engine.NewVec2(1.1, 1.1), Origin: engine.NewVec2(.5, .5),
	})
	var err error
	g.watch, err = g.WatchTransform(g.shadow.Ref(), func(transform engine.Transform3D) {
		if g.resetPending && math.Abs(float64(transform.Position.X)-dungeon.Spawn.X) < .0001 &&
			math.Abs(float64(transform.Position.Y)-dungeon.Spawn.Y) < .0001 {
			g.resetPending = false
		}
	})
	if err != nil {
		g.status = err.Error()
		g.refresh()
		return false
	}
	return true
}

// The fixed shadow remains the authoritative collision root. The upright
// painting uses its own sole anchor without moving that physical root.
func (g *Game) updateHeroPose(pose int, root dungeon.Point) {
	data := pilgrimSpriteRig[pose]
	transform := engine.NewTransform3D(engine.NewVec3(float32(root.X)+data.OffsetX, float32(root.Y)+data.OffsetY, .03+data.OffsetZ))
	texture := firstWalkPilgrimTexture + pose
	if pose >= dungeon.WalkTextureCount {
		texture = firstIdlePilgrimTexture + pose - dungeon.WalkTextureCount
	}
	asset := engine.LevelTexture(g.level, uint32(texture))
	origin := engine.NewVec2(data.OriginX, data.OriginY)
	if g.hero == nil {
		g.hero = g.NewWorldSprite(asset, transform, engine.WorldSprite{
			Facing: engine.WorldSpriteFacingUpright, Alpha: engine.WorldSpriteAlphaCutout,
			Size: engine.NewVec2(pilgrimProjectedSize, float32(pilgrimProjectedSize/math.Cos(math.Pi/6))), Origin: origin,
		})
		g.hero.SetTags("pilgrim", "camera-occlusion-focus")
		g.heroTransform = transform
		g.lastTexture = texture
		return
	}
	if transform != g.heroTransform {
		if err := g.hero.Ref().SetPose(transform); err != nil {
			panic(err)
		}
		g.heroTransform = transform
	}
	if texture != g.lastTexture {
		sprite := g.hero.WorldSprite
		sprite.Asset, sprite.Origin = asset, origin
		g.hero.SetSprite(sprite)
		g.lastTexture = texture
	}
}
