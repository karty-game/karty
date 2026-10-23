package main

import (
	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/internal/village"
	"github.com/karty-game/controllers-topdown"
)

func (g *Game) key(k engine.Key, down bool) {
	if k == engine.KeyC {
		if down {
			g.rig.toggleSettings()
		}
		return
	}
	if control, ok := movementControl(k); ok {
		g.player.controller.Key(control, down)
		if down {
			g.player.controller.CancelTarget()
			g.player.clearRoute()
		}
		return
	}
	switch k {
	case engine.KeyShiftLeft:
		g.player.controller.Key(controllers.RunLeft, down)
		return
	case engine.KeyShiftRight:
		g.player.controller.Key(controllers.RunRight, down)
		return
	case engine.KeyEscape:
		if down {
			if g.loaded {
				g.quest.close(&g.player, &g.hud)
			}
			g.player.resetInput()
		}
		return
	case engine.KeyE, engine.KeySpace:
		if down && !g.player.interactHeld && g.loaded {
			g.quest.inspectNearby(&g.player, &g.hud)
		}
		g.player.interactHeld = down
	case engine.KeyR:
		if down && g.loaded {
			g.restart()
		}
	}
}

func (g *Game) pointer(p engine.PointerInput) {
	if p.Phase == engine.PointerCancel {
		g.player.resetInput()
		return
	}
	if p.Phase == engine.PointerDown && g.loaded {
		g.player.controller.CancelTarget()
		g.player.clearRoute()
		if g.quest.selectAt(float64(p.X), float64(p.Y), g.rig.settings, &g.player, &g.hud) {
			return
		}
		q := g.rig.settings.ScreenToWorld(engine.NewVec2(float32(p.X), float32(p.Y)), 1280, 720)
		g.player.requestPath(&g.engine, g.level, village.Point{X: float64(q.X), Y: float64(q.Y)})
	}
}
