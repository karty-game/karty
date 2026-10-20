package main

import (
	"example.com/terrain/.karty/config"
	"example.com/terrain/.karty/engine"
)

const viewpointCount = 11

type Game struct {
	engine.Game
	camera   *engine.Camera3D
	level    engine.LevelHandle
	fps      *FPSController
	status   *engine.Text2D
	view     int
	viewHeld bool
}

//go:wasmexport karty_register
func main() {
	game := &Game{}
	engine.Run(engine.Hooks{
		OnStart:       game.start,
		OnUpdate:      func(frame engine.Frame) { game.fps.Update(frame) },
		OnKeyDown:     func(key engine.Key) { game.key(key, true) },
		OnKeyUp:       func(key engine.Key) { game.key(key, false) },
		OnPointer:     func(pointer engine.PointerInput) { game.fps.Pointer(pointer) },
		OnInputReset:  func() { game.fps.Reset(); game.viewHeld = false },
		OnLevelReady:  game.ready,
		OnLevelFailed: func(event engine.LevelFailed) { game.status.SetText("Level failed: " + event.Diagnostic) },
		OnStop: func() {
			game.fps.Close()
			if game.level != 0 {
				game.ReleaseLevel(game.level)
			}
		},
	})
}

func (game *Game) start() {
	game.fps = NewFPSController(&game.Game)
	game.fps.SceneContains = func(x, y float32) bool { return y > 80 }
	game.status = game.NewText2D("Loading cove terrain...", engine.NewVec2(22, 18))
	game.status.SetFontSize(20)
	game.status.SetColor(engine.RGBA(245, 241, 223, 255))
	game.status.SetLayer(30)
	help := game.NewText2D(
		"Coastal vegetation · WASD / mouse · Shift run · V views (11) · R reset · Touch: left move / right look",
		engine.NewVec2(22, 48),
	)
	help.SetFontSize(14)
	help.SetLayer(30)
	game.RequestLevel("levels.ground")
}

func (game *Game) ready(event engine.LevelReady) {
	game.level = event.Handle
	if err := game.World(game.level).
		Configure(engine.WorldSettings{Mode: engine.WorldSpatial, Gravity: engine.Vec3{Z: -9.81}}); err != nil {
		panic(err)
	}
	game.visit(0)
}

func (game *Game) visit(index int) {
	if game.level == 0 || index < 0 || index >= viewpointCount {
		return
	}
	game.fps.SetCamera(nil, nil)
	if game.camera != nil {
		game.camera.Destroy()
	}
	position, yaw, pitch, name := coveViewpoint(index)
	game.camera = game.NewCamera3D(game.level, engine.Camera3DSettings{
		Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
		Movement: engine.CameraMovementUnrestricted,
		Position: position, Yaw: yaw, Pitch: pitch,
		VerticalFOV: config.CameraFOVY, NearClip: config.CameraNear, FarClip: config.CameraFar,
		OrthoHeight: config.CameraOrthoHeight, ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	})
	game.camera.SetLayer(-10)
	body, err := game.World(game.level).
		AttachBody(game.camera.Ref(), engine.BodySettings{Kind: engine.BodyDynamic, Collider: engine.BoxCollider{HalfExtents: engine.NewVec3(.3, .3, .85), Offset: engine.NewVec3(0, 0, -.85)}, AllowRotation: false, Forces: engine.BodyForceSettings{GravityScale: 1, Buoyancy: false}, Mass: 80, Friction: .5})
	if err != nil {
		panic(err)
	}
	game.fps.SetCamera(game.camera, body)

	game.status.SetText(name)

	game.view = index
}

func coveViewpoint(index int) (position engine.Vec3, yaw, pitch float32, name string) {
	switch index {
	case 10:
		return engine.NewVec3(20, -12, -1.7), .55, -.25, "Cove seabed · Seagrass and ribbon kelp"
	case 9:
		return engine.NewVec3(25, 29, 8.34), 1.35, -.16, "Coastal understory · Grass and small shrubs"
	case 7:
		return engine.NewVec3(17, 10, 2.1), .82, .4, "Beach palms · Curved trunks and folded fronds"
	case 8:
		return engine.NewVec3(28, 36, 16.75), -1.05, .25, "Broadleaf grove · Connected branches and leaf clusters"
	case 1:
		return engine.NewVec3(-16, -6, 1.685851), -1.3, .18, "Cove cliff foot · Terrain study"
	case 2:
		return engine.NewVec3(-18, 30, 16.234175), 2.8, -.1, "Cove overlook · Terrain study"
	case 3:
		return engine.NewVec3(11.4, 13.8, 2.85), 3.1415927, -.025, "Wooden harbor · Cove study"
	case 4:
		return engine.NewVec3(-30.4, -6, 15.95), 1.5707963, -.04, "Cliff bunker · Cove study"
	case 5:
		return engine.NewVec3(-36, 5, 19.7), 3.1415927, .28, "Lighthouse approach · Cove study"
	case 6:
		return engine.NewVec3(-16, 19, 7.2), -1.95, -.08, "Beach / bunker / lighthouse paths"
	default:
		return engine.NewVec3(22, 18, 2.980826), -1.93, .015, "Cove beach · Terrain study"
	}
}

func (game *Game) key(key engine.Key, down bool) {
	game.fps.Key(key, down)
	if key == engine.KeyR && down {
		game.visit(0)
	}
	if key != engine.KeyV {
		return
	}
	if down && !game.viewHeld {
		game.visit((game.view + 1) % viewpointCount)
	}
	game.viewHeld = down
}
