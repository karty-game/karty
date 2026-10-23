package main

import (
	"math"

	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
	"github.com/karty-game/controllers-isometric"
)

type cameraRig struct {
	game    *engine.Game
	view    *engine.Camera3D
	focus   dungeon.Point
	height  float64
	framing *controllers.CameraComposer
	lab     cameraLab
	hud     *vigilHUD
}

func (c *cameraRig) configure() {
	options := controllers.DefaultCameraFramingOptions()
	options.Damping = controllers.Vec2{X: .1, Y: .16}
	options.DeadZone = controllers.Vec2{X: .025, Y: .04}
	options.SoftZone = controllers.Vec2{X: .2, Y: .55}
	options.MaxLookahead = .45
	c.lab.options, c.lab.defaults = options, options
	var err error
	c.framing, err = controllers.NewCameraComposer(options)
	if err != nil {
		panic(err)
	}
}

// follow uses the accepted position; proposed movement keeps the camera and hero preview in sync.
func (c *cameraRig) follow(position dungeon.Point, proposedMove bool, controller *controllers.Planar) {
	focus, err := controller.ComposeCamera(c.framing,
		controllers.Vec2{X: c.focus.X, Y: c.focus.Y},
		controllers.Vec2{X: position.X, Y: position.Y}, 1280, 720, false)
	if err != nil {
		panic(err)
	}
	changed := math.Abs(focus.X-c.focus.X)+math.Abs(focus.Y-c.focus.Y) > .00001
	c.focus = dungeon.Point{X: focus.X, Y: focus.Y}
	if changed || proposedMove {
		camera := controller.CameraPosition(focus)
		pose := engine.NewTransform3D(engine.NewVec3(float32(camera.X), float32(camera.Y), float32(camera.Z)))
		pose.Yaw, pose.Pitch = math.Pi/4, -math.Pi/6
		if err := c.view.Ref().SetPose(pose); err != nil {
			panic(err)
		}
	}
}

func (c *cameraRig) zoom(factor float64, controller *controllers.Planar) {
	c.height = controller.ZoomBy(factor)
	if c.view != nil {
		s := c.view.Settings()
		s.OrthoHeight = float32(c.height)
		c.view.SetSettings(s)
	}
}

func (c *cameraRig) create(level engine.LevelHandle) {
	c.focus = dungeon.Point{dungeon.Spawn.X + 1.6, dungeon.Spawn.Y + 1.6}
	c.view = c.game.NewCamera3D(level, engine.Camera3DSettings{
		Projection: engine.CameraProjectionIsometric, Output: engine.CameraOutputFull,
		Position: engine.NewVec3(float32(c.focus.X-18), float32(c.focus.Y-18), 14.696938),
		Yaw:      math.Pi / 4, Pitch: -math.Pi / 6, VerticalFOV: 1.05, OrthoHeight: float32(c.height),
		NearClip: .05, FarClip: 100, ViewportWidth: 1280, ViewportHeight: 720,
	})
	c.view.SetLayer(-10)
}

// Teleporting discards damping history and restores the authored spawn framing.
func (c *cameraRig) reset() {
	c.framing.Reset()
	c.focus = dungeon.Point{dungeon.Spawn.X + 1.6, dungeon.Spawn.Y + 1.6}
	pose := engine.NewTransform3D(engine.NewVec3(float32(c.focus.X-18), float32(c.focus.Y-18), 14.696938))
	pose.Yaw, pose.Pitch = math.Pi/4, -math.Pi/6
	if err := c.view.Ref().SetPose(pose); err != nil {
		panic(err)
	}
}
