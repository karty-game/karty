package main

import (
	"math"

	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
)

// Watch the physical root so a fresh spawn can acknowledge a pending reset.
func (p *pilgrim) createRoot() error {
	pose := engine.NewTransform3D(engine.NewVec3(float32(dungeon.Spawn.X), float32(dungeon.Spawn.Y), .025))
	pose.Pitch = math.Pi / 2
	p.shadow = p.game.NewSprite3D(engine.LevelTexture(p.level, shadowTexture), pose, engine.SpriteRenderer3D{
		Facing: engine.WorldSpriteFacingFixed, AlphaMode: engine.WorldSpriteAlphaBlend,
		Size: engine.NewVec2(1.1, 1.1), Anchor: engine.NewVec2(.5, .5),
	})
	var err error
	p.watch, err = p.game.WatchPose(p.shadow.Ref(), func(transform engine.Transform3D) {
		if p.resetPending && math.Abs(float64(transform.Position.X)-dungeon.Spawn.X) < .0001 &&
			math.Abs(float64(transform.Position.Y)-dungeon.Spawn.Y) < .0001 {
			p.resetPending = false
		}
	})
	return err
}

// The fixed shadow remains the authoritative collision root. The upright
// painting uses its own sole anchor without moving that physical root.
func (p *pilgrim) pose(pose int, root dungeon.Point) {
	data := pilgrimSpriteRig[pose]
	transform := engine.NewTransform3D(engine.NewVec3(float32(root.X)+data.OffsetX, float32(root.Y)+data.OffsetY, .03+data.OffsetZ))
	texture := firstWalkPilgrimTexture + pose
	if pose >= dungeon.WalkTextureCount {
		texture = firstIdlePilgrimTexture + pose - dungeon.WalkTextureCount
	}
	asset := engine.LevelTexture(p.level, uint32(texture))
	origin := engine.NewVec2(data.OriginX, data.OriginY)
	if p.hero == nil {
		p.hero = p.game.NewSprite3D(asset, transform, engine.SpriteRenderer3D{
			Facing: engine.WorldSpriteFacingUpright, AlphaMode: engine.WorldSpriteAlphaCutout,
			Size: engine.NewVec2(pilgrimProjectedSize, float32(pilgrimProjectedSize/math.Cos(math.Pi/6))), Anchor: origin,
		})
		p.hero.SetTags("pilgrim", "camera-occlusion-focus")
		p.heroTransform = transform
		p.lastTexture = texture
		return
	}
	if transform != p.heroTransform {
		if err := p.hero.Ref().SetPose(transform); err != nil {
			panic(err)
		}
		p.heroTransform = transform
	}
	if texture != p.lastTexture {
		sprite := p.hero.Settings()
		sprite.Asset, sprite.Anchor = asset, origin
		p.hero.SetSprite(sprite)
		p.lastTexture = texture
	}
}

func (p *pilgrim) reset(vision *dungeon.Vision) error {
	// EntityRef.SetPose is a swept movement proposal. A reset is a fresh spawn,
	// which must not sweep through furniture on the route back to the entrance.
	p.watch.Stop()
	p.hero.Destroy()
	p.shadow.Destroy()
	p.hero, p.shadow = nil, nil
	p.halo.SetPose(engine.NewVec3(float32(dungeon.Spawn.X), float32(dungeon.Spawn.Y), 1.63), 0, 0)
	p.walk, p.facing = dungeon.WalkCycle{}, 4
	p.aim = dungeon.FacingDirection(p.facing)
	vision.HeroFrame = dungeon.WalkTextureCount + 4
	vision.HeroOffset = dungeon.Point{}
	p.pose(dungeon.WalkTextureCount+4, dungeon.Spawn)
	return p.createRoot()
}
