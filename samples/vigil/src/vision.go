package main

import (
	"example.com/vigil/.karty/assets"
	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
)

type sightMask struct {
	game              *engine.Game
	model             dungeon.Vision
	fog               [dungeon.MaxFog]*engine.Sprite2D
	fogBounds         [dungeon.MaxFog]dungeon.Box
	fogUsed, fogReady int
	curtain           *engine.Shape2D
}

func (v *sightMask) update(p dungeon.Point, height float64, focus dungeon.Point) {
	// Populate the bounded pool over several updates so initial entity commands
	// stay below the SDK's complete-batch 64 KiB limit. The curtain hides loading.
	for end := min(v.fogReady+64, len(v.fog)); v.fogReady < end; v.fogReady++ {
		s := v.game.NewSprite2D(assets.TextureSightMask, engine.NewVec2(0, 0))
		s.SetLayer(-9)
		s.SetAlpha(0)
		v.fog[v.fogReady] = s
	}
	if v.fogReady < len(v.fog) {
		return
	}
	if !v.model.Update(p, height, focus) {
		return
	}
	for i, b := range v.model.Fog[:v.model.Count] {
		if i >= v.fogUsed || b != v.fogBounds[i] {
			v.fog[i].SetTransform(
				engine.NewVec2(float32(b.X+(b.W+.5)/2), float32(b.Y+b.D/2)),
				engine.NewVec2(float32((b.W+.5)/2), float32(b.D/2)),
				0,
			)
			v.fogBounds[i] = b
		}
		if i >= v.fogUsed {
			v.fog[i].SetAlpha(1)
		}
	}
	for i := v.model.Count; i < v.fogUsed; i++ {
		v.fog[i].SetAlpha(0)
	}
	v.fogUsed = v.model.Count
	if v.curtain.Alpha() != 0 {
		v.curtain.SetAlpha(0)
	}
}

func (v *sightMask) createCurtain() {
	v.curtain = v.game.NewShape2D(engine.Rectangle(dungeon.FogRight-dungeon.FogLeft, 720), engine.NewVec2(dungeon.FogLeft, 0))
	v.curtain.SetFill(engine.ColorBlack)
	v.curtain.SetLayer(-8)
}
