package main

import (
	"fmt"
	"math"

	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/.karty/processed"
	"example.com/willowbrook/internal/village"
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
			func(key string) (uint32, bool) {
				if id, ok := textureIDs[key]; ok {
					return id, true
				}
				return processed.TextureID("tiled.compile", key)
			},
			// Leave room for the hero, shadow, quest sprites and journal entities.
			karty2d.Options{Wind: true, MaxLiveSprites: engine.MaxEntityRefs - 64},
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
func (v *tiledView) mapLayer(layer int, footY float64) int32 {
	n, err := v.renderer.LayerFor(layer, footY)
	if err != nil {
		panic(err)
	}
	return n
}

func (v *tiledView) footSprite(asset uint32, p village.Point, w, h float64) *engine.Sprite2D {
	s := v.game.NewLevelSprite2D(engine.LevelTexture(v.level, asset), engine.NewVec2(float32(p.X), float32(p.Y-h/2)))
	dims := textureSizes[asset]
	s.SetScale(float32(w)/float32(dims[0]), float32(h)/float32(dims[1]))
	s.SetLayer(v.mapLayer(village.ActorLayer, p.Y))
	return s
}

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
