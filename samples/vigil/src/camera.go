package main

import (
	"example.com/vigil/internal/dungeon"
	"github.com/karty-game/controllers"
)

func (g *Game) configureCameraFraming() {
	options := controllers.DefaultCameraFramingOptions()
	options.Damping = controllers.Vec2{X: .1, Y: .16}
	options.DeadZone = controllers.Vec2{X: .025, Y: .04}
	options.SoftZone = controllers.Vec2{X: .2, Y: .55}
	options.MaxLookahead = .45
	g.cameraLab.options, g.cameraLab.defaults = options, options
	var err error
	g.framing, err = controllers.NewCameraComposer(options)
	if err != nil {
		panic(err)
	}
}

func (g *Game) composeCamera(position dungeon.Point, snap bool) controllers.Vec2 {
	focus, err := g.controller.ComposeCamera(g.framing,
		controllers.Vec2{X: g.focus.X, Y: g.focus.Y},
		controllers.Vec2{X: position.X, Y: position.Y}, 1280, 720, snap)
	if err != nil {
		panic(err)
	}
	return focus
}
