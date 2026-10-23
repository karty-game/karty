package main

import (
	"example.com/ui-demo/.karty/engine"
	"example.com/ui-demo/.karty/ui"
)

type flightDemo struct {
	settings   ui.FlightSettings
	shape      *engine.Shape2D
	playing    bool
	deliveries uint32
}

func (flight *flightDemo) initialize() {
	flight.settings = ui.FlightSettings{CallSign: "Nova", Ship: 1, Power: 50, Autopilot: true}
}

// Recreate the ship for each level while keeping the pilot's settings.
func (flight *flightDemo) spawn(game *engine.Game) {
	flight.shape = game.NewShape2D(engine.Rectangle(64, 32), engine.NewVec2(96, 168))
	flight.configure()
}

func (flight *flightDemo) configure() {
	if flight.shape == nil {
		return
	}
	color := engine.RGBA(112, 214, 255, 255)
	if flight.settings.Ship == 2 {
		color = engine.RGBA(240, 169, 90, 255)
	}
	flight.shape.SetFill(color)
	flight.shape.SetStroke(engine.ColorWhite, 2)
}

func (flight *flightDemo) deliver() bool {
	if !flight.playing {
		return false
	}
	flight.deliveries++
	return true
}

// The console settings drive this simple flight demonstration at 60 Hz.
func (flight *flightDemo) step(engine.Frame) {
	if !flight.playing || !flight.settings.Autopilot || flight.shape == nil {
		return
	}
	step := float32(flight.settings.Power) / 25
	if flight.settings.Ship == 2 {
		step *= 1.5
	}
	x := flight.shape.Position().X + step
	if x > 864 {
		x = 32
	}
	flight.shape.Translate(x-flight.shape.Position().X, 0)
}

func (flight *flightDemo) clear() {
	if flight.shape != nil {
		flight.shape.Destroy()
		flight.shape = nil
	}
}
