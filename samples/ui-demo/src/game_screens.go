package main

import (
	"example.com/ui-demo/.karty/engine"
	"example.com/ui-demo/.karty/ui"
)

// Every screen shares the same live settings, inventory and flight progress.
func (game *Game) uiProps() ui.AppProps {
	return ui.AppProps{
		Show:        func(view *engine.UI) { game.cockpit.view = view },
		Quantity:    func() int { return game.cockpit.quantity },
		UsePotion:   game.cockpit.consumePotion,
		Mount:       game.mount,
		Level:       func() engine.LevelName { return game.level.name },
		Destination: game.level.destination,
		Resume:      game.resume,
		Pause:       game.pause,
		Leave:       game.leave,
		Flight:      &game.flight.settings,
		Configure:   game.flight.configure,
		Deliveries:  func() uint32 { return game.flight.deliveries },
		Deliver:     game.deliver,
	}
}

func (game *Game) showMenu() {
	game.cockpit.view = ui.Menu(game.uiProps())
}

func (game *Game) resume() {
	if game.level.handle == 0 || game.level.release != 0 {
		return
	}
	game.flight.playing = true
	game.cockpit.view = ui.GameScreen(game.uiProps())
}

func (game *Game) pause() {
	if !game.flight.playing {
		return
	}
	game.flight.playing = false
	game.cockpit.view = ui.Pause(game.uiProps())
}
