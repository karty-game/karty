package main

import (
	"example.com/terrain/.karty/config"
	"example.com/terrain/.karty/engine"
	fpscontroller "example.com/terrain/.karty/processed/code/controllers-3d.sdk/controller"
	"example.com/terrain/internal/fps"
)

// coveExplorer owns the mounted landscape, camera and landmark selection.
type coveExplorer struct {
	level      engine.LevelHandle
	camera     *engine.Camera3D
	controller *fpscontroller.FirstPersonController
	swimming   bool
	view       int
	viewHeld   bool
}

type viewpoint struct {
	position   engine.Vec3
	yaw, pitch float32
	name       string
}

var viewpoints = [...]viewpoint{
	{engine.NewVec3(22, 18, 2.980826), -1.93, .015, "Cove beach · Terrain study"},
	{engine.NewVec3(-16, -6, 1.685851), -1.3, .18, "Cove cliff foot · Terrain study"},
	{engine.NewVec3(-18, 30, 16.234175), 2.8, -.1, "Cove overlook · Terrain study"},
	{engine.NewVec3(11.4, 13.8, 2.85), 3.1415927, -.025, "Wooden harbor · Cove study"},
	{engine.NewVec3(-30.4, -6, 15.95), 1.5707963, -.04, "Cliff bunker · Cove study"},
	{engine.NewVec3(-36, 5, 19.7), 3.1415927, .28, "Lighthouse approach · Cove study"},
	{engine.NewVec3(-16, 19, 7.2), -1.95, -.08, "Beach / bunker / lighthouse paths"},
	{engine.NewVec3(17, 10, 2.1), .82, .4, "Beach palms · Curved trunks and folded fronds"},
	{engine.NewVec3(28, 36, 16.75), -1.05, .25, "Broadleaf grove · Connected branches and leaf clusters"},
	{engine.NewVec3(25, 29, 8.34), 1.35, -.16, "Coastal understory · Grass and small shrubs"},
	{engine.NewVec3(20, -12, -1.7), .55, -.25, "Cove seabed · Seagrass and ribbon kelp"},
}

// Restore the old body's gravity before replacing it with a fresh landmark camera.
func (explorer *coveExplorer) visit(host *engine.Game, index int) string {
	if explorer.controller != nil && explorer.swimming && explorer.controller.Body().Ref().Alive() {
		if err := explorer.controller.Body().SetForceSettings(engine.BodyForceSettings{GravityScale: 1}); err != nil {
			panic(err)
		}
	}
	if explorer.camera != nil {
		explorer.camera.Destroy()
	}
	view := viewpoints[index]
	explorer.camera = host.NewCamera3D(explorer.level, engine.Camera3DSettings{
		Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
		Movement: engine.CameraMovementUnrestricted,
		Position: view.position, Yaw: view.yaw, Pitch: view.pitch,
		VerticalFOV: config.CameraFOVY, NearClip: config.CameraNear, FarClip: config.CameraFar,
		OrthoHeight: config.CameraOrthoHeight, ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	})
	explorer.camera.SetLayer(-10)
	options := fpscontroller.DefaultFirstPersonOptions()
	options.Width, options.Height = float32(config.ResolutionWidth), float32(config.ResolutionHeight)
	options.ArrowKeysLook, options.TurnKeys = true, false
	options.Touch.StickRadius, options.Touch.InsetY, options.Touch.ThumbRadius = 96, 140, 32
	options.Touch.BaseRadius = 96
	options.Body.Friction = .5
	options.SceneContains = func(x, y float32) bool { return y > 80 }
	options.Move = explorer.move
	var err error
	explorer.controller, err = fpscontroller.AttachFirstPerson(*host, explorer.camera, options)
	if err != nil {
		panic(err)
	}
	explorer.swimming = false

	explorer.view = index
	return view.name
}

// Configure level physics before attaching the landmark camera's character motor.
func (explorer *coveExplorer) ready(host *engine.Game, event engine.LevelReady) {
	explorer.level = event.Handle
	if err := host.World(explorer.level).
		Configure(engine.WorldSettings{Mode: engine.WorldSpatial, Gravity: engine.Vec3{Z: -9.81}}); err != nil {
		panic(err)
	}
}

// R returns to the beach on key-down; V advances only once per held key.
func (explorer *coveExplorer) shortcut(key engine.Key, down bool) (index int, visit bool) {
	if key == engine.KeyR && down {
		return 0, true
	}
	if key != engine.KeyV {
		return 0, false
	}
	pressed := down && !explorer.viewHeld
	explorer.viewHeld = down
	return (explorer.view + 1) % len(viewpoints), pressed
}

func (explorer *coveExplorer) release(host *engine.Game) {
	if explorer.level != 0 {
		host.ReleaseLevel(explorer.level)
	}
}

// The authored ocean and bunker decide when motion becomes neutral-buoyancy swimming.
func (explorer *coveExplorer) move(controller *fpscontroller.FirstPersonController, motion fpscontroller.FirstPersonMotion) error {
	const oceanSurfaceZ = float32(0)
	settings := controller.Camera().Settings()
	if motion.Yaw != settings.Yaw || motion.Pitch != settings.Pitch {
		settings.Yaw, settings.Pitch = motion.Yaw, motion.Pitch
		controller.Camera().SetSettings(settings)
	}
	position := motion.Position
	swimming := fps.OceanSwimming(position.X, position.Y, position.Z, oceanSurfaceZ)
	body := controller.Body()
	if swimming != explorer.swimming {
		gravity := float32(1)
		if swimming {
			gravity = 0
		}
		if err := body.SetForceSettings(engine.BodyForceSettings{GravityScale: gravity}); err != nil {
			return err
		}
		explorer.swimming = swimming
	}
	if !swimming {
		return controller.ApplyMotion(motion)
	}
	input := fps.Motion{X: motion.X, Y: motion.Y, Z: motion.Z, Yaw: motion.Yaw, Pitch: motion.Pitch}
	x, y, z := fps.SwimVelocity(input, position.Z, oceanSurfaceZ)
	return body.SetVelocity(engine.NewVec3(x, y, z))
}
