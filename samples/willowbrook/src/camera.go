package main

import "github.com/karty-game/controllers"

func (g *Game) configureCameraFraming() {
	options := controllers.DefaultCameraFramingOptions()
	options.Damping = controllers.Vec2{X: .12, Y: .18}
	options.DeadZone = controllers.Vec2{X: .035, Y: .05}
	options.SoftZone = controllers.Vec2{X: .2, Y: .55}
	options.LookaheadTime, options.MaxLookahead = .15, 24
	g.cameraLab.options, g.cameraLab.defaults = options, options
	var err error
	g.framing, err = controllers.NewCameraComposer(options)
	if err != nil {
		panic(err)
	}
}

func (g *Game) composeCamera(snap bool) controllers.Vec2 {
	focus, err := g.controller.ComposeCamera(g.framing,
		controllers.Vec2{X: g.focus.X, Y: g.focus.Y},
		controllers.Vec2{X: g.position.X, Y: g.position.Y}, 1280, 720, snap)
	if err != nil {
		panic(err)
	}
	return focus
}
