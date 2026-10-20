package main

import (
	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/internal/village"
)

func (g *Game) footSprite(asset uint32, p village.Point, w, h float64) *engine.Sprite2D {
	s := g.NewLevelSprite2D(engine.LevelTexture(g.level, asset), engine.NewVec2(float32(p.X), float32(p.Y-h/2)))
	dims := textureSizes[asset]
	s.SetScale(float32(w)/float32(dims[0]), float32(h)/float32(dims[1]))
	s.SetLayer(g.mapLayer(village.ActorLayer, p.Y))
	return s
}
