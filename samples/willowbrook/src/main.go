package main

import (
	"example.com/willowbrook/.karty/assets"
	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/.karty/processed"
	"example.com/willowbrook/.karty/ui"
	"example.com/willowbrook/internal/village"
	"github.com/karty-game/tiled"
)

type Game struct {
	engine engine.Game
	level  engine.LevelHandle
	loaded bool
	view   tiledView
	player villagePlayer
	rig    cameraRig
	quest  treasureQuest
	hud    villageHUD
}

//go:wasmexport karty_register
func main() {
	g := &Game{player: villagePlayer{lastPose: -1}}
	engine.Run(engine.Hooks{
		OnStart:       g.start,
		OnUpdate:      g.update,
		OnKeyDown:     func(k engine.Key) { g.key(k, true) },
		OnKeyUp:       func(k engine.Key) { g.key(k, false) },
		OnInputReset:  g.player.resetInput,
		OnPointer:     g.pointer,
		OnPathResult:  func(result engine.PathResult) { g.player.pathResult(result, g.level) },
		OnLevelFailed: func(e engine.LevelFailed) { g.hud.setTitle("Village loading failed: " + e.Diagnostic) },
	})
}

func (g *Game) start() {
	compiledMap, ok := processed.Read("tiled.compile", "internal/village/village.bin")
	if !ok {
		panic("compiled Tiled map is missing")
	}
	levelMap, mapErr := tiled.Decode(compiledMap)
	if mapErr != nil {
		panic(mapErr)
	}
	village.Configure(levelMap)
	g.player.position = village.Spawn

	g.player.configure()
	g.rig.game, g.rig.hud = &g.engine, &g.hud
	g.rig.configure()
	g.hud.title = "LOADING VILLAGE"
	g.hud.view = ui.CameraHud(ui.CameraHUDProps{
		Camera:          g.rig.settingsProps(),
		Title:           func() string { return g.hud.title },
		JournalText:     func() string { return g.quest.text },
		JournalOpen:     func() bool { return g.quest.journalOpen },
		JournalReady:    func() bool { return g.loaded },
		ActivateJournal: func() { g.quest.activate(g.loaded, &g.player, &g.hud) },
	})
	g.engine.LoadLevel(assets.LevelVillage, g.ready)
}

func (g *Game) ready(e engine.LevelReady) {
	if g.level != 0 {
		return
	}
	g.level = e.Handle
	g.player.configureWorld(&g.engine, g.level)
	compiledRender, ok := processed.Read("tiled.compile", "internal/village/village.render.bin")
	if !ok {
		panic("compiled Tiled render map is missing")
	}
	renderMap, err := tiled.Decode(compiledRender)
	if err != nil {
		panic(err)
	}
	g.view = tiledView{game: &g.engine, level: e.Handle, levelMap: renderMap}
	g.rig.settings = engine.Camera2DSettings{Zoom: float32(village.Zoom()), WorldMinLayer: 0, WorldMaxLayer: 100000000}
	g.rig.focus = village.CameraFocus(g.player.position)
	g.rig.follow(g.player.position, g.player.controller, true)
}

func (g *Game) update(engine.Frame) {
	if g.level == 0 {
		return
	}
	if !g.loaded {
		if !g.view.build(g.rig.settings) {
			return
		}
		g.loaded = true
		g.player.paint(&g.view, false)
		g.quest.start(&g.view, &g.player, &g.hud)
		g.hud.setTitle("WILLOWBROOK")
		return
	}
	g.view.build(g.rig.settings)
	g.player.advanceRoute()
	walking := g.player.move()
	g.quest.update(&g.player, &g.hud)
	g.rig.follow(g.player.position, g.player.controller, false)
	g.player.paint(&g.view, walking)
}

func (g *Game) restart() {
	g.player.position = village.Spawn
	g.player.walkPhase = 0
	g.player.resetInput()
	g.quest.restart(&g.player, &g.hud)
	g.rig.follow(g.player.position, g.player.controller, true)
	g.player.paint(&g.view, false)
}
