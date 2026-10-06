package main

import (
	"example.com/ui-demo/.karty/engine"
	"example.com/ui-demo/.karty/ui"
)

// Game owns flight data and level lifecycle. Components own screen navigation.
type Game struct {
	engine.Game
	ui               *engine.UI
	level            string
	handle           engine.LevelHandle
	request, release engine.LevelRequestID
	quantity         int
	playing          bool
	shape            *engine.EntityVector2D
	flight           ui.FlightSettings
	deliveries       uint32
}

func (game *Game) uiProps() ui.AppProps {
	return ui.AppProps{
		Show:     func(view *engine.UI) { game.ui = view },
		Quantity: func() int { return game.quantity }, UsePotion: game.consumePotion,
		Mount: game.mount, Level: func() string { return game.level },
		Destination: func() string {
			if game.level == "levels.second" {
				return "Outer relay"
			}
			return "Orbital station"
		},
		Resume: game.resume, Pause: game.pause, Leave: game.leave,
		Flight: &game.flight, Configure: game.configureShip,
		Deliveries: func() uint32 { return game.deliveries }, Deliver: game.deliver,
	}
}

func (game *Game) Hooks() engine.Hooks {
	return engine.Hooks{
		OnStart: game.Initialize, OnUpdate: game.advanceFlight, OnStop: game.Shutdown,
		OnLevelReady: game.levelReady, OnLevelFailed: game.levelFailed,
		OnLevelReleased: game.levelReleased,
		OnKeyDown: func(key engine.Key) {
			if key == engine.KeyEscape {
				game.pause()
			}
		},
	}
}

func (game *Game) Initialize() {
	game.quantity = 3
	game.flight = ui.FlightSettings{CallSign: "Nova", Ship: 1, Power: 50, Autopilot: true}
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
	if game.request != 0 || game.handle != 0 {
		return
	}
	game.level = level
	game.request = game.RequestLevel(level)
	game.ui = ui.Loading(game.uiProps())
}

func (game *Game) levelReady(event engine.LevelReady) {
	if event.RequestID != game.request || game.request == 0 {
		return
	}
	game.request = 0
	game.handle = event.Handle
	game.shape = game.NewVector2D(engine.Rectangle(64, 32), engine.NewVec2D(96, 168))
	game.configureShip()
	game.resume()
}

func (game *Game) levelFailed(event engine.LevelFailed) {
	if event.RequestID == game.request && game.request != 0 {
		game.request = 0
		game.ui = ui.LoadError(game.uiProps(), event.Diagnostic)
	} else if event.RequestID == game.release && game.release != 0 {
		game.release = 0
		game.ui = ui.Pause(game.uiProps())
	}
}

func (game *Game) resume() {
	if game.handle == 0 || game.release != 0 {
		return
	}
	game.playing = true
	game.ui = ui.GameScreen(game.uiProps())
}

func (game *Game) pause() {
	if !game.playing {
		return
	}
	game.playing = false
	game.ui = ui.Pause(game.uiProps())
}

func (game *Game) leave() {
	if game.handle == 0 || game.release != 0 {
		return
	}
	game.playing = false
	game.release = game.ReleaseLevel(game.handle)
	game.ui = ui.Releasing(game.uiProps())
}

func (game *Game) levelReleased(event engine.LevelReleased) {
	if event.RequestID != game.release || game.release == 0 || event.Handle != game.handle {
		return
	}
	game.handle, game.release = 0, 0
	if game.shape != nil {
		game.shape.Destroy()
		game.shape = nil
	}
	game.ui = ui.Menu(game.uiProps())
}

func (game *Game) configureShip() {
	if game.shape == nil {
		return
	}
	color := engine.RGBA(112, 214, 255, 255)
	if game.flight.Ship == 2 {
		color = engine.RGBA(240, 169, 90, 255)
	}
	game.shape.SetFill(color)
	game.shape.SetStroke(engine.ColorWhite, 2)
}

func (game *Game) deliver() {
	if !game.playing {
		return
	}
	game.deliveries++
	if game.ui != nil {
		game.ui.Invalidate()
	}
}

func (game *Game) advanceFlight(engine.Frame) {
	if !game.playing || !game.flight.Autopilot || game.shape == nil {
		return
	}
	step := float32(game.flight.Power) / 25
	if game.flight.Ship == 2 {
		step *= 1.5
	}
	x := game.shape.Position.X + step
	if x > 864 {
		x = 32
	}
	game.shape.Translate(x-game.shape.Position.X, 0)
}

func (game *Game) Shutdown() {
	if game.ui != nil {
		game.ui.Close()
	}
	if game.shape != nil {
		game.shape.Destroy()
		game.shape = nil
	}
}

//go:wasmexport karty_register
func main() { game := &Game{}; engine.Run(game.Hooks()) }
