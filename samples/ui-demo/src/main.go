package main

import (
	"example.com/ui-demo/.karty/engine"
	"example.com/ui-demo/.karty/ui"
)

// Game connects the flight, level lifecycle and cockpit controls.
type Game struct {
	engine  engine.Game
	level   levelSession
	flight  flightDemo
	cockpit cockpit
}

func (game *Game) Hooks() engine.Hooks {
	return engine.Hooks{
		OnStart:         game.Initialize,
		OnUpdate:        game.flight.step,
		OnStop:          game.Shutdown,
		OnLevelFailed:   game.levelFailed,
		OnLevelReleased: game.levelReleased,
		OnKeyDown: func(key engine.Key) {
			if key == engine.KeyEscape {
				game.pause()
			}
		},
	}
}

func (game *Game) Initialize() {
	game.cockpit.quantity = 3
	game.flight.initialize()
	game.showMenu()
}

func (game *Game) Shutdown() {
	game.cockpit.close()
	game.flight.clear()
}

func (game *Game) mount(name engine.LevelName) {
	if game.level.mount(&game.engine, name, game.levelReady) {
		game.cockpit.view = ui.Loading(game.uiProps())
	}
}

func (game *Game) levelReady(event engine.LevelReady) {
	game.level.ready(event)
	game.flight.spawn(&game.engine)
	game.resume()
}

func (game *Game) levelFailed(event engine.LevelFailed) {
	if event.RequestID == game.level.request && game.level.request != 0 {
		game.level.request = 0
		game.cockpit.view = ui.LoadError(game.uiProps(), event.Diagnostic)
	} else if event.RequestID == game.level.release && game.level.release != 0 {
		game.level.release = 0
		game.cockpit.view = ui.Pause(game.uiProps())
	}
}

// Pause now; keep the ship until the host confirms the level release.
func (game *Game) leave() {
	if game.level.handle == 0 || game.level.release != 0 {
		return
	}
	game.flight.playing = false
	game.level.release = game.engine.ReleaseLevel(game.level.handle)
	game.cockpit.view = ui.Releasing(game.uiProps())
}

func (game *Game) levelReleased(event engine.LevelReleased) {
	if !game.level.released(event) {
		return
	}
	game.flight.clear()
	game.showMenu()
}

func (game *Game) deliver() {
	if game.flight.deliver() {
		game.cockpit.invalidate()
	}
}

//go:wasmexport karty_register
func main() {
	game := &Game{}
	engine.Run(game.Hooks())
}
