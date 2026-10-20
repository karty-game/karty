package main

import (
	"fmt"

	"example.com/tiled-room/.karty/engine"
	"github.com/karty-game/tiled/karty2d"
)

type boundSprite struct {
	entity *engine.Sprite2D
	state  karty2d.SpriteState
}
type spriteBackend struct {
	game    *engine.Game
	level   engine.LevelHandle
	sprites map[karty2d.Handle]boundSprite
}

func (b *spriteBackend) Create(s karty2d.SpriteState) (karty2d.Handle, error) {
	e := b.game.NewLevelSprite2D(engine.LevelTexture(b.level, s.AssetID), engine.NewVec2(float32(s.X), float32(s.Y)))
	e.SetTransform(e.Position(), engine.NewVec2(float32(s.ScaleX), float32(s.ScaleY)), float32(s.Rotation))
	e.SetLayer(s.Layer)
	if s.Opacity != 1 {
		e.SetAlpha(float32(s.Opacity))
	}
	h := karty2d.Handle(e.ID())
	b.sprites[h] = boundSprite{entity: e, state: s}
	return h, nil
}
func (b *spriteBackend) Update(h karty2d.Handle, s karty2d.SpriteState) error {
	old, ok := b.sprites[h]
	if !ok {
		return fmt.Errorf("unknown Tiled sprite handle %d", h)
	}
	e, p := old.entity, old.state
	if s.AssetID != p.AssetID {
		e.SetLevelTexture(engine.LevelTexture(b.level, s.AssetID))
	}
	if s.X != p.X || s.Y != p.Y || s.ScaleX != p.ScaleX || s.ScaleY != p.ScaleY || s.Rotation != p.Rotation {
		e.SetTransform(
			engine.NewVec2(float32(s.X), float32(s.Y)),
			engine.NewVec2(float32(s.ScaleX), float32(s.ScaleY)),
			float32(s.Rotation),
		)
	}
	if s.Layer != p.Layer {
		e.SetLayer(s.Layer)
	}
	if s.Opacity != p.Opacity {
		e.SetAlpha(float32(s.Opacity))
	}
	b.sprites[h] = boundSprite{entity: e, state: s}
	return nil
}
func (b *spriteBackend) Destroy(h karty2d.Handle) error {
	old, ok := b.sprites[h]
	if !ok {
		return fmt.Errorf("unknown Tiled sprite handle %d", h)
	}
	old.entity.Destroy()
	delete(b.sprites, h)
	return nil
}
