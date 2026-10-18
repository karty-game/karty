package main

import (
	"example.com/tiled-room/.karty/engine"
	"example.com/tiled-room/.karty/processed"
	"github.com/karty-game/tiled"
	"github.com/karty-game/tiled/karty2d"
)

type Game struct {
	engine.Game
	request engine.RequestID
	view    *karty2d.View
	camera  engine.Camera2DSettings
	label   *engine.EntityText2D
	keys    [4]bool
	ready   bool
	tick    int64
}

//go:wasmexport karty_register
func main() {
	g := &Game{}
	engine.Run(engine.Hooks{OnStart: g.start, OnLevelReady: g.loaded, OnUpdate: g.update,
		OnKeyDown: func(k engine.Key) { g.key(k, true) }, OnKeyUp: func(k engine.Key) { g.key(k, false) },
		OnInputReset: func() { g.keys = [4]bool{} }, OnPointer: g.pointer})
}
func (g *Game) start() {
	g.label = g.NewText2D("LOADING", engine.NewVec2(560, 24))
	g.label.SetSize(18)
	g.label.SetLayer(1000000000)
	g.request = g.RequestLevel("levels.room")
}
func (g *Game) loaded(e engine.LevelReady) {
	if e.RequestID != g.request || g.view != nil {
		return
	}
	compiledMap, ok := processed.Read("tiled.compile", "maps/room.bin")
	if !ok {
		panic("compiled Tiled map is missing")
	}
	m, err := tiled.Decode(compiledMap)
	if err != nil {
		panic(err)
	}
	b := &spriteBackend{game: &g.Game, level: e.Handle, sprites: make(map[karty2d.Handle]boundSprite)}
	g.view, err = karty2d.New(m, b, func(key string) (uint32, bool) { id, ok := textureIDs[key]; return id, ok }, karty2d.Options{})
	if err != nil {
		panic(err)
	}
	g.camera = engine.Camera2DSettings{Position: engine.NewVec2(192, 160), Zoom: 1.8, WorldMinLayer: 0, WorldMaxLayer: 100000000}
	g.SetCamera2D(g.camera)
}
func (g *Game) update(engine.Frame) {
	if g.view == nil {
		return
	}
	dx, dy := float32(0), float32(0)
	if g.keys[0] {
		dy -= 3
	}
	if g.keys[1] {
		dy += 3
	}
	if g.keys[2] {
		dx -= 3
	}
	if g.keys[3] {
		dx += 3
	}
	if dx != 0 || dy != 0 {
		g.camera.Position.X += dx
		g.camera.Position.Y += dy
		g.SetCamera2D(g.camera)
	}
	z := float64(g.camera.Zoom)
	x, y := float64(g.camera.Position.X), float64(g.camera.Position.Y)
	g.tick++
	ready, err := g.view.Update(
		karty2d.Bounds{MinX: x - 640/z - 32, MaxX: x + 640/z + 32, MinY: y - 360/z - 32, MaxY: y + 360/z + 32},
		g.tick*1000/60,
	)
	if err != nil {
		panic(err)
	}
	if ready && !g.ready {
		g.ready = true
		g.label.SetText("TILED ROOM")
	}
}
func (g *Game) key(k engine.Key, down bool) {
	switch k {
	case engine.KeyW, engine.KeyArrowUp:
		g.keys[0] = down
	case engine.KeyS, engine.KeyArrowDown:
		g.keys[1] = down
	case engine.KeyA, engine.KeyArrowLeft:
		g.keys[2] = down
	case engine.KeyD, engine.KeyArrowRight:
		g.keys[3] = down
	}
	if down && k == engine.KeyR && g.view != nil {
		g.camera.Position = engine.NewVec2(192, 160)
		g.SetCamera2D(g.camera)
	}
}
func (g *Game) pointer(p engine.PointerInput) {
	if p.Phase != engine.PointerDown || !g.ready {
		return
	}
	g.camera.Position = g.camera.ScreenToWorld(engine.NewVec2(float32(p.X), float32(p.Y)), 1280, 720)
	g.SetCamera2D(g.camera)
	g.label.SetText("TILED POINTER")
}
