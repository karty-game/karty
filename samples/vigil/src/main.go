package main

import (
	"strconv"

	"example.com/vigil/.karty/assets"
	"example.com/vigil/.karty/engine"
	"example.com/vigil/.karty/ui"
	"example.com/vigil/internal/dungeon"
)

type Game struct {
	engine  engine.Game
	level   engine.LevelHandle
	player  pilgrim
	rig     cameraRig
	spells  spellCaster
	audio   vigilAudio
	sight   sightMask
	shrines vigilShrines
	hud     vigilHUD
}

//go:wasmexport karty_register
func main() {
	g := &Game{player: pilgrim{lastTexture: -1}, rig: cameraRig{height: 13.2}, hud: vigilHUD{help: true}}
	engine.Run(engine.Hooks{
		OnStart:      g.start,
		OnUpdate:     g.update,
		OnStop:       g.stop,
		OnPathResult: g.pathResult,
		OnLevelFailed: func(e engine.LevelFailed) {
			g.hud.status = "Unable to enter the crypt: " + e.Diagnostic
			g.hud.refresh()
		},
		OnKeyDown:    func(k engine.Key) { g.key(k, true) },
		OnKeyUp:      func(k engine.Key) { g.key(k, false) },
		OnPointer:    g.point,
		OnInputReset: g.resetInput,
	})
}

func (g *Game) start() {
	g.player.game = &g.engine
	g.rig.game, g.rig.hud = &g.engine, &g.hud
	g.spells.game, g.spells.hud = &g.engine, &g.hud
	g.audio.game = &g.engine
	g.sight.game = &g.engine
	g.shrines.game = &g.engine
	g.player.configure(g.rig.height)
	g.rig.configure()
	g.hud.status = "Descending beneath the abbey..."
	g.createHUD()
	g.engine.LoadLevel(assets.LevelCrypt, g.ready)
}

// Effects and HUD readiness keep ticking while a reset waits for its accepted spawn.
func (g *Game) update(f engine.Frame) {
	if g.player.hero == nil {
		return
	}
	g.audio.frame = f.Number
	g.spells.update(&g.audio)
	g.hud.update(g.audio.frame, g.spells.ready(&g.player))
	p := g.player.position()
	// Ignore old accepted poses until the fresh spawn is acknowledged.
	if g.player.resetPending {
		g.player.pose(dungeon.WalkTextureCount+4, dungeon.Spawn)
		g.sight.update(dungeon.Spawn, g.rig.height, g.rig.focus)
		return
	}
	// Use the latest drag position once per update, including its final target.
	if g.player.navigation.routePending {
		g.player.navigation.routePending = false
		g.route(g.player.navigation.pointerX, g.player.navigation.pointerY)
	}
	travel := g.player.acceptedTravel(p)
	g.audio.update(travel, p)
	proposedMove := g.player.move(p, travel, &g.sight.model)
	g.rig.follow(p, proposedMove, g.player.controller)
	g.sight.update(p, g.rig.height, g.rig.focus)
	g.hud.location(p, g.shrines.near(&g.player) >= 0)
	g.shrines.animate(f.Number)
}

func (g *Game) stop() {
	g.resetInput()
	g.spells.clear()
	g.audio.stop()
	g.player.watch.Stop()
	if g.hud.view != nil {
		g.hud.view.Close()
	}
}

func (g *Game) createHUD() {
	g.hud.view = ui.Vigil(ui.VigilProps{
		Area:       func() string { return g.hud.area },
		Status:     func() string { return g.hud.status },
		Progress:   func() string { return strconv.Itoa(g.shrines.progress()) + " / 3" },
		Prompt:     func() string { return g.hud.prompt },
		Help:       func() bool { return g.hud.help },
		SpellReady: func() bool { return g.spells.ready(&g.player) },
		ToggleHelp: g.hud.toggleHelp,
		Interact:   g.interact,
		Reset:      g.restart,
		Fireball:   func() { g.spells.throw(&g.player, &g.audio) },
		ZoomIn:     func() { g.rig.zoom(.9, g.player.controller) },
		ZoomOut:    func() { g.rig.zoom(1.1, g.player.controller) },
		Camera:     g.rig.settingsProps(),
	})
}
