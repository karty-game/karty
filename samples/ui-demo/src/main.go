package main

import (
	"example.com/ui-demo/.karty/engine"
	"example.com/ui-demo/.karty/ui"
)

// Game owns gameplay and domain data. Screen bindings and handlers live in .ui.
type Game struct {
	engine.Game
	ui       *engine.UI
	level    string
	handle   engine.LevelHandle
	request  engine.LevelRequestID
	quantity int
	playing  bool
	shape    *engine.EntityVector2D
}

// uiProps connects UI-owned props to domain data; UI handlers stay in .ui files.
func (game *Game) uiProps() ui.AppProps {
	return ui.AppProps{
		Show:      func(view *engine.UI) { game.ui = view },
		Quantity:  func() int { return game.quantity },
		UsePotion: game.consumePotion,
		Mount:     game.mount,
		Level:     func() string { return game.level },
		Resume:    game.resume,
		Leave:     game.leave,
	}
}

func (game *Game) Initialize() {
	game.quantity = 3
	game.ui = ui.Menu(game.uiProps())
}

func (game *Game) consumePotion() bool {
	if game.quantity <= 0 {
		return false
	}
	game.quantity--
	return true
}

func (game *Game) mount(level string) {
	game.level = level
	game.request = game.RequestLevel(level)
	game.ui = ui.Loading(game.uiProps())
}

func (game *Game) resume() {
	game.playing = true
	game.ui = game.ShowLevelUI(game.handle, "ui.hud", nil, nil)
}

func (game *Game) leave() {
	game.playing = false
	game.request = game.ReleaseLevel(game.handle)
	game.ui = ui.Releasing(game.uiProps())
}

func (game *Game) Update(frame engine.Frame) {
	for _, event := range frame.Events {
		switch event.Type {
		case engine.EventLevelReady:
			if event.RequestID != uint32(game.request) {
				continue
			}
			game.handle = engine.LevelHandle(event.Handle)
			game.shape = game.NewVector2D(engine.Rectangle(70, 70), engine.NewVec2D(100, 180))
			game.shape.SetFill(engine.RGBA(70, 160, 220, 255))
			game.resume()
		case engine.EventLevelFailed:
			if event.RequestID != uint32(game.request) {
				continue
			}
			game.ui = ui.LoadError(game.uiProps(), event.Diagnostic)
		case engine.EventLevelReleased:
			if event.Handle != uint32(game.handle) {
				continue
			}
			game.handle = 0
			if game.shape != nil {
				game.shape.Destroy()
				game.shape = nil
			}
			game.ui = ui.Menu(game.uiProps())
		case engine.EventPointerUp:
			if game.playing {
				game.playing = false
				game.ui = ui.Pause(game.uiProps())
			}
		}
	}
	if game.playing && game.shape != nil {
		game.shape.Translate(100+float32(frame.Number%360)-game.shape.Position.X, 0)
	}
}

func (game *Game) Shutdown() {
	if game.ui != nil {
		game.ui.Close()
	}
}

//go:wasmexport karty_register
func main() { engine.Run(&Game{}) }
