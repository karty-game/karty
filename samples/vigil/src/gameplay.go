package main

import (
	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
	"github.com/karty-game/controllers-isometric"
)

// Install the route grid before spawning the player into the authored world.
func (g *Game) ready(e engine.LevelReady) {
	if g.player.hero != nil {
		return
	}
	g.level = e.Handle
	g.player.level, g.spells.level, g.audio.level, g.shrines.level = e.Handle, e.Handle, e.Handle, e.Handle
	if err := g.engine.World(g.level).Configure(engine.WorldSettings{
		Mode:     engine.WorldSpatial,
		Min:      engine.Vec2{},
		Max:      engine.Vec2{X: dungeon.Columns * dungeon.Cell, Y: dungeon.Rows * dungeon.Cell},
		CellSize: dungeon.Cell,
	}); err != nil {
		panic(err)
	}
	g.player.lastPosition = dungeon.Spawn
	g.sight.createCurtain()
	g.rig.create(g.level)
	g.player.facing = 4
	g.player.aim = dungeon.FacingDirection(g.player.facing)
	g.player.pose(dungeon.WalkTextureCount+4, dungeon.Spawn)
	if err := g.player.createRoot(); err != nil {
		g.hud.setStatus(err.Error())
		return
	}
	g.audio.create(g.player.shadow, g.rig.view)
	g.player.halo = g.engine.NewPointLight(
		g.level,
		engine.NewVec3(float32(dungeon.Spawn.X), float32(dungeon.Spawn.Y), 1.63),
		engine.NewVec3(.58, .39, .2),
		3.8,
	)
	g.shrines.createLights()
	g.hud.status = "Rekindle the three vigil lights."
	g.hud.refresh()
}

// Discard old input and effects before rebuilding the movement root at the entrance.
func (g *Game) restart() {
	if g.player.hero == nil {
		return
	}
	g.resetInput()
	g.spells.clear()
	g.audio.reset()
	g.player.resetPending = true
	if err := g.player.reset(&g.sight.model); err != nil {
		g.hud.setStatus(err.Error())
		return
	}
	g.audio.bindReceiver(g.player.shadow, g.rig.view)
	g.player.lastPosition = dungeon.Spawn
	g.rig.reset()
	g.shrines.reset()
	g.hud.status = "Rekindle the three vigil lights."
	g.hud.refresh()
}

func (g *Game) route(x, y float32) {
	ground, ok := g.player.controller.ScreenToGround(
		controllers.Vec2{X: float64(x), Y: float64(y)},
		controllers.Vec2{X: g.rig.focus.X, Y: g.rig.focus.Y},
		1280,
		720,
	)
	if !ok {
		return
	}
	target := dungeon.Point{X: ground.X, Y: ground.Y}
	// Hidden ground remains a valid destination. A nearby shrine tap also
	// acts as the touch equivalent of E; distant taps walk toward it.
	for i, p := range dungeon.Shrines {
		if !g.shrines.lit[i] && dungeon.Distance(target, p) < 1 && dungeon.Distance(g.player.position(), p) < 1.65 {
			g.interact()
			return
		}
	}
	g.requestPath(target)
}

func (g *Game) interact() {
	i := g.shrines.near(&g.player)
	if i < 0 {
		return
	}
	g.hud.setStatus(g.shrines.light(i, &g.audio))
}

// requestPath clears the old route before checking whether the player can move.
func (g *Game) requestPath(target dungeon.Point) {
	g.player.navigation.path.Cancel()
	if g.level == 0 || g.player.resetPending {
		return
	}
	if err := g.player.navigation.requestPath(&g.engine, g.level, g.player.position(), target); err != nil {
		g.hud.setStatus(err.Error())
	}
}

// Ignore late path results after a new click, reset or level change.
func (g *Game) pathResult(result engine.PathResult) {
	if g.player.hero == nil || g.player.resetPending || result.Handle != g.level ||
		uint32(result.RequestID) != g.player.navigation.path.Pending {
		return
	}
	if err := g.player.navigation.acceptPath(result, g.player.position()); err != nil {
		g.hud.setStatus(err.Error())
	}
}
