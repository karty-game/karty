package main

import (
	"strconv"

	"example.com/environment/.karty/config"
	"example.com/environment/.karty/engine"
	"example.com/environment/.karty/ui"
)

type Game struct {
	engine.Game
	camera       *engine.EntityCamera3D
	chairs       [12]*engine.EntityWorldSolid
	level        engine.LevelHandle
	request      engine.RequestID
	panel        *engine.UI
	fps          *FPSController
	keys         [256]bool
	view         int
	status       string
	controlsOpen bool
	timeOfDay    float32
	cloudAmount  float32
}

//go:wasmexport karty_register
func main() { game := &Game{}; engine.Run(game.Hooks()) }

func (game *Game) Hooks() engine.Hooks {
	return engine.Hooks{
		OnStart: game.start, OnUpdate: func(frame engine.Frame) { game.fps.Update(frame) }, OnStop: game.stop,
		OnKeyDown: func(key engine.Key) { game.key(key, true) }, OnKeyUp: func(key engine.Key) { game.key(key, false) },
		OnPointer:    func(pointer engine.PointerInput) { game.fps.Pointer(pointer) },
		OnInputReset: func() { game.keys = [256]bool{}; game.fps.Reset() },
		OnLevelReady: func(event engine.LevelReady) {
			if event.RequestID != game.request {
				game.ReleaseLevel(event.Handle)
				return
			}
			game.request = 0
			game.level = event.Handle
			if err := game.World(game.level).
				Configure(engine.WorldSettings{Mode: engine.WorldSpatial, Gravity: engine.Vec3{Z: -9.81}}); err != nil {
				panic(err)
			}
			game.createChairs()
			game.applyEnvironment()
			game.visit(game.view)
		},
		OnLevelFailed: func(event engine.LevelFailed) {
			if event.RequestID != game.request {
				return
			}
			game.request = 0
			game.status = "Level failed: " + event.Diagnostic
			game.invalidate()
		},
	}
}

func (game *Game) start() {
	game.fps = NewFPSController(&game.Game)
	game.fps.SceneContains = func(x, y float32) bool {
		return !game.controlsOpen && x >= 0 && x <= float32(config.ResolutionWidth) &&
			y >= 160 && y <= float32(config.ResolutionHeight)
	}
	game.fps.OnTouchMode = game.invalidate
	game.showPanel()
	game.loadLevel()
}

func (game *Game) showPanel() {
	game.controlsOpen = false
	game.panel = ui.Hud(ui.HudProps{
		Status: func() string { return game.status }, Environment: game.environmentLabel,
		Open: game.showControls,
	})
}

func (game *Game) showControls() {
	game.fps.Release()
	if game.panel != nil {
		game.panel.Close()
	}
	game.controlsOpen = true
	game.panel = ui.Explorer(ui.ExplorerProps{
		Visit: func(index int) { game.visit(index); game.hideControls() }, Close: game.hideControls,
		Environment: game.environmentLabel, SetTime: game.setTime,
		CloudAmount: func() int32 { return int32(game.cloudAmount*100 + .5) }, SetCloudAmount: game.setCloudAmount,
		ResetEnvironment: game.resetEnvironment,
	})
}

func (game *Game) hideControls() {
	game.fps.Release()
	if game.panel != nil {
		game.panel.Close()
	}
	game.showPanel()
}

func (game *Game) loadLevel() {
	if game.request != 0 {
		game.CancelLevel(game.request)
	}
	game.clearChairs()
	game.fps.SetCamera(nil, nil)
	if game.camera != nil {
		game.camera.Destroy()
		game.camera = nil
	}
	if game.level != 0 {
		game.ReleaseLevel(game.level)
		game.level = 0
	}
	game.timeOfDay, game.cloudAmount = 12, .35
	game.status = "Preparing daylight..."
	game.invalidate()
	game.request = game.RequestLevel("levels.daylight")
}

func (game *Game) environmentLabel() string {
	return strconv.Itoa(int(game.timeOfDay)) + ":00 / Clouds " + strconv.Itoa(int(game.cloudAmount*100+.5)) + "%"
}

func (game *Game) setTime(time float32) {
	if time < 0 || time >= 24 {
		return
	}
	game.timeOfDay = time
	game.applyEnvironment()
}

func (game *Game) setCloudAmount(percent int32) {
	if percent < 0 || percent > 100 {
		return
	}
	game.cloudAmount = float32(percent) / 100
	game.applyEnvironment()
}

func (game *Game) resetEnvironment() {
	game.timeOfDay, game.cloudAmount = 12, .35
	game.applyEnvironment()
}

func (game *Game) applyEnvironment() {
	if game.level != 0 {
		game.World(game.level).ConfigureEnvironment(engine.EnvironmentSettings{TimeOfDay: game.timeOfDay, CloudAmount: game.cloudAmount})
	}
	game.invalidate()
}

type viewpoint struct {
	name                string
	x, y, z, yaw, pitch float32
}

var viewpoints = [...]viewpoint{
	// Frame the west recess obliquely, with its dining table in the foreground
	// and the courtyard and mountains visible beyond the glass.
	{"Windows", -8.3, -10.5, 1.7, .55, .06},
	{"Courtyard", 1, 0, 1.7, -.08, .04},
	{"Look back", 1, 13, 1.7, 3.20, .08},
	{"Test ground", 0, 32, 3.14, -.35, .18},
}

func (game *Game) visit(index int) {
	if game.level == 0 || index < 0 || index >= len(viewpoints) {
		return
	}
	game.fps.SetCamera(nil, nil)
	if game.camera != nil {
		game.camera.Destroy()
	}
	p := viewpoints[index]
	game.camera = game.NewCamera3D(game.level, engine.CameraSettings{
		Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
		Position: engine.NewVec3(p.x, p.y, p.z), Yaw: p.yaw, Pitch: p.pitch,
		FOVY: config.CameraFOVY, Near: config.CameraNear, Far: config.CameraFar,
		OrthoHeight: config.CameraOrthoHeight, ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	})
	game.camera.SetLayer(-10)
	body, err := game.World(game.level).
		AttachBody(game.camera.Ref(), engine.BodySettings{Kind: engine.BodyDynamic, Collider: engine.BoxCollider{HalfExtents: engine.NewVec3(.3, .3, .85), Offset: engine.NewVec3(0, 0, -.85)}, Rotate: false, Forces: engine.BodyForces{GravityScale: 1, Buoyancy: false}, Mass: 80, Friction: .5})
	if err != nil {
		panic(err)
	}
	game.fps.SetCamera(game.camera, body)

	game.status = p.name

	game.view = index
	game.invalidate()
}

func (game *Game) key(key engine.Key, down bool) {
	game.fps.Key(key, down)
	if key != engine.KeyV && key != engine.KeyH &&
		key != engine.KeyT && key != engine.KeyC && key != engine.KeyR {
		return
	}
	held := game.keys[key]
	game.keys[key] = down
	if !down || held {
		return
	}
	switch key {
	case engine.KeyV:
		game.visit((game.view + 1) % len(viewpoints))
	case engine.KeyT:
		time := float32(9)
		switch game.timeOfDay {
		case 9:
			time = 12
		case 12:
			time = 17
		case 17:
			time = 0
		}
		game.setTime(time)
	case engine.KeyC:
		percent := int32(0)
		switch int32(game.cloudAmount*100 + .5) {
		case 0:
			percent = 35
		case 35:
			percent = 70
		case 70:
			percent = 100
		}
		game.setCloudAmount(percent)
	case engine.KeyR:
		game.resetEnvironment()
	case engine.KeyH:
		game.showControls()
	}
}

func (game *Game) invalidate() {
	if game.panel != nil {
		game.panel.Invalidate()
	}
}

func (game *Game) stop() {
	game.clearChairs()
	game.fps.Close()
	if game.camera != nil {
		game.camera.Destroy()
	}
	if game.request != 0 {
		game.CancelLevel(game.request)
	}
	if game.level != 0 {
		game.ReleaseLevel(game.level)
	}
	if game.panel != nil {
		game.panel.Close()
	}
}
