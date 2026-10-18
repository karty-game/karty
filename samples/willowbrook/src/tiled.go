package main

import (
	"fmt"
	"math"

	"example.com/willowbrook/.karty/engine"
	"github.com/karty-game/tiled"
	"github.com/karty-game/tiled/karty2d"
)

// Only this bridge knows the game's generated bindings. Parsing, image identity,
// transforms, residency and animation live in the shared package.
type tiledView struct {
	levelMap *tiled.Map
	game     *engine.Game
	level    engine.LevelHandle
	renderer *karty2d.View
	tick     int64
}

func (v *tiledView) build(camera engine.Camera2DSettings) bool {
	if v.renderer == nil {
		backend := &spriteBackend{game: v.game, level: v.level, sprites: make(map[karty2d.Handle]boundSprite)}
		view, err := karty2d.New(
			v.levelMap,
			backend,
			func(key string) (uint32, bool) { id, ok := textureIDs[key]; return id, ok },
			karty2d.Options{Wind: true},
		)
		if err != nil {
			panic(err)
		}
		v.renderer = view
	}
	// Quantized conservative bounds retain the sample's 2.5:1 view plus overscan.
	x := math.Floor(float64(camera.Position.X)/128) * 128
	y := math.Floor(float64(camera.Position.Y)/128) * 128
	z := float64(camera.Zoom)
	v.tick++
	ready, err := v.renderer.Update(
		karty2d.Bounds{MinX: x - 900/z - 256, MaxX: x + 900/z + 256, MinY: y - 360/z - 256, MaxY: y + 360/z + 256},
		v.tick*1000/60,
	)
	if err != nil {
		panic(err)
	}
	return ready
}
func (g *Game) mapLayer(layer int, footY float64) int32 {
	n, err := g.view.renderer.LayerFor(layer, footY)
	if err != nil {
		panic(err)
	}
	return n
}

type boundSprite struct {
	entity *engine.EntitySprite2D
	state  karty2d.SpriteState
}
type spriteBackend struct {
	game    *engine.Game
	level   engine.LevelHandle
	sprites map[karty2d.Handle]boundSprite
}

func (b *spriteBackend) Create(s karty2d.SpriteState) (karty2d.Handle, error) {
	e := b.game.NewLevelSprite2D(engine.LevelTexture(b.level, s.AssetID), engine.NewVec2(float32(s.X), float32(s.Y)))
	e.Set(e.Position, engine.NewVec2(float32(s.ScaleX), float32(s.ScaleY)), float32(s.Rotation))
	e.SetLayer(s.Layer)
	if s.Opacity != 1 {
		e.SetAlpha(float32(s.Opacity))
	}
	h := karty2d.Handle(e.ID)
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
		e.UpdateLevelAsset(engine.LevelTexture(b.level, s.AssetID))
	}
	if s.X != p.X || s.Y != p.Y || s.ScaleX != p.ScaleX || s.ScaleY != p.ScaleY || s.Rotation != p.Rotation {
		e.Set(engine.NewVec2(float32(s.X), float32(s.Y)), engine.NewVec2(float32(s.ScaleX), float32(s.ScaleY)), float32(s.Rotation))
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
