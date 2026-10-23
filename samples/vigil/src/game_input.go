package main

import (
	"example.com/vigil/.karty/engine"
	"github.com/karty-game/controllers-isometric"
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
			g.player.navigation.routePending = false
			g.player.navigation.path.Cancel()
			g.player.navigation.held = false
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
			g.resetInput()
		}
	case engine.KeyE, engine.KeySpace:
		if down {
			g.interact()
		}
	case engine.KeyH:
		if down {
			g.hud.toggleHelp()
		}
	case engine.KeyF:
		if down && !g.spells.held {
			g.spells.throw(&g.player, &g.audio)
		}
		g.spells.held = down
	case engine.KeyR:
		if down {
			g.restart()
		}
	case engine.KeyEqual:
		if down {
			g.rig.zoom(.9, g.player.controller)
		}
	case engine.KeyMinus:
		if down {
			g.rig.zoom(1.1, g.player.controller)
		}
	}
}

func (g *Game) point(p engine.PointerInput) {
	if g.player.hero == nil {
		return
	}
	if p.Phase == engine.PointerCancel {
		g.resetInput()
		return
	}
	g.player.navigation.point(p)
}

func (g *Game) resetInput() {
	g.player.navigation.routePending = false
	g.player.controller.Reset()
	g.player.navigation.held, g.spells.held = false, false
	g.player.navigation.path.Cancel()
}
