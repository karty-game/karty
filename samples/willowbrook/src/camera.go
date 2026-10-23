package main

import (
	"math"

	"example.com/willowbrook/.karty/engine"
	"example.com/willowbrook/internal/village"
	"github.com/karty-game/controllers-topdown"
)

type cameraRig struct {
	game     *engine.Game
	settings engine.Camera2DSettings
	focus    village.Point
	framing  *controllers.CameraComposer
	lab      cameraLab
	hud      *villageHUD
}

func (c *cameraRig) configure() {
	options := controllers.DefaultCameraFramingOptions()
	options.Damping = controllers.Vec2{X: .12, Y: .18}
	options.DeadZone = controllers.Vec2{X: .035, Y: .05}
	options.SoftZone = controllers.Vec2{X: .2, Y: .55}
	options.LookaheadTime, options.MaxLookahead = .15, 24
	c.lab.options, c.lab.defaults = options, options
	var err error
	c.framing, err = controllers.NewCameraComposer(options)
	if err != nil {
		panic(err)
	}
}

// follow snaps on spawn/reset and otherwise lets the composer smooth movement.
func (c *cameraRig) follow(position village.Point, controller *controllers.Planar, snap bool) {
	focus, err := controller.ComposeCamera(c.framing,
		controllers.Vec2{X: c.focus.X, Y: c.focus.Y},
		controllers.Vec2{X: position.X, Y: position.Y}, 1280, 720, snap)
	if err != nil {
		panic(err)
	}
	next := village.Point{X: focus.X, Y: focus.Y}
	if !snap && math.Hypot(next.X-c.focus.X, next.Y-c.focus.Y) < .001 {
		return
	}
	c.focus = next
	c.settings.Position = engine.NewVec2(float32(next.X), float32(next.Y))
	c.game.SetCamera2D(c.settings)
}
