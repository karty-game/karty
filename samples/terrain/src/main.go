package main

import (
	"example.com/terrain/.karty/assets"
	"example.com/terrain/.karty/engine"
)

type Game struct {
	engine.Game
	explorer coveExplorer
	hud      terrainHUD
}

//go:wasmexport karty_register
func main() {
	game := &Game{}
	engine.Run(engine.Hooks{
		OnStart:       game.start,
		OnKeyDown:     func(key engine.Key) { game.key(key, true) },
		OnKeyUp:       func(key engine.Key) { game.key(key, false) },
		OnInputReset:  func() { game.explorer.viewHeld = false },
		OnLevelFailed: func(event engine.LevelFailed) { game.hud.setStatus("Level failed: " + event.Diagnostic) },
		OnStop:        game.stop,
	})
}

func (game *Game) start() {
	game.hud.show()
	game.LoadLevel(assets.LevelGround, game.ready)
}

func (game *Game) ready(event engine.LevelReady) {
	game.explorer.ready(&game.Game, event)
	game.visit(0)
}

func (game *Game) visit(index int) {
	if game.explorer.level == 0 || index < 0 || index >= len(viewpoints) {
		return
	}
	game.hud.setStatus(game.explorer.visit(&game.Game, index))
}

func (game *Game) key(key engine.Key, down bool) {
	if index, visit := game.explorer.shortcut(key, down); visit {
		game.visit(index)
	}
}

func (game *Game) stop() {
	game.hud.close()
	game.explorer.release(&game.Game)
}
