package main

import (
	"example.com/vigil/.karty/assets"
	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
)

func (g *Game) updateVision(p dungeon.Point) {
	// Populate the bounded pool over several updates so initial entity commands
	// stay below the SDK's complete-batch 64 KiB limit. The curtain hides loading.
	for end := min(g.fogReady+64, len(g.fog)); g.fogReady < end; g.fogReady++ {
		s := g.NewSprite2D(assets.TextureSightMask, engine.NewVec2(0, 0))
		s.SetLayer(-9)
		s.SetAlpha(0)
		g.fog[g.fogReady] = s
	}
	if g.fogReady < len(g.fog) {
		return
	}
	if !g.vision.Update(p, g.height, g.focus) {
		return
	}
	for i, b := range g.vision.Fog[:g.vision.Count] {
		if i >= g.fogUsed || b != g.fogBounds[i] {
			g.fog[i].SetTransform(
				engine.NewVec2(float32(b.X+(b.W+.5)/2), float32(b.Y+b.D/2)),
				engine.NewVec2(float32((b.W+.5)/2), float32(b.D/2)),
				0,
			)
			g.fogBounds[i] = b
		}
		if i >= g.fogUsed {
			g.fog[i].SetAlpha(1)
		}
	}
	for i := g.vision.Count; i < g.fogUsed; i++ {
		g.fog[i].SetAlpha(0)
	}
	g.fogUsed = g.vision.Count
	if g.curtain.Alpha() != 0 {
		g.curtain.SetAlpha(0)
	}
}
