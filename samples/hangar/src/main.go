package main

import (
	"example.com/hangar/.karty/config"
	"example.com/hangar/.karty/engine"
	"example.com/hangar/.karty/ui"
)

type Game struct {
	engine.Game
	camera  *engine.EntityCamera3D
	level   engine.LevelHandle
	request engine.LevelRequestID
	panel   *engine.UI
	fps     *FPSController
	keys    [256]bool
	view    int
	status  string
}

//go:wasmexport karty_register
func main() { game := &Game{}; engine.Run(game.Hooks()) }

func (game *Game) Hooks() engine.Hooks {
	return engine.Hooks{
		OnStart: game.start, OnUpdate: game.update, OnStop: game.stop,
		OnKeyDown:    func(key engine.Key) { game.key(key, true) },
		OnKeyUp:      func(key engine.Key) { game.key(key, false) },
		OnPointer:    func(pointer engine.PointerInput) { game.fps.Pointer(pointer) },
		OnInputReset: func() { game.keys = [256]bool{}; game.fps.Reset() },
		OnLevelReady: func(event engine.LevelReady) {
			if event.RequestID != game.request {
				return
			}
			game.level = event.Handle
			game.visit(0)
		},
		OnLevelFailed: func(event engine.LevelFailed) {
			game.status = "Level failed: " + event.Diagnostic
			if game.panel != nil {
				game.panel.Invalidate()
			}
		},
	}
}

func (game *Game) start() {
	game.status = "Preparing the hangar..."
	game.fps = NewFPSController(&game.Game)
	game.fps.SceneContains = func(x, y float32) bool {
		return game.panel == nil || (y > 145 && y < float32(config.ResolutionHeight)-125)
	}
	game.fps.OnTouchMode = func() {
		if game.panel != nil {
			game.panel.Invalidate()
		}
	}
	game.showPanel()
	game.request = game.RequestLevel("levels.hangar")
}

func (game *Game) showPanel() {
	game.panel = ui.Explorer(ui.ExplorerProps{
		Status: func() string { return game.status }, Controls: game.fps.Help,
		Visit: game.visit, Hide: func() { game.panel.Close(); game.panel = nil },
	})
}

type viewpoint struct {
	name                string
	x, y, z, yaw, pitch float32
}

var viewpoints = [...]viewpoint{
	{"01 / WINDOWED ARRIVAL", 9, -4, 1.7, -.314, 0},
	{"02 / REACTOR HALL", 4.1, 16.5, 1.7, .42, .06},
	{"03 / CONTROL DECK", -8, 17.8, 2.9, -.65, -.05},
	{"04 / ZIGZAG TRENCH", 27.5, 21.8, 1.9, .419, -.157},
	{"05 / LOADING HALL", 21.2, 6, 2.1, 1.658, .06},
	{"06 / EXTRACTION", 25, 40, 1.7, .873, 0},
	{"07 / SERVICE WINDOWS", 8, 6, 1.7, -1.9897, 0},
}

func (game *Game) visit(index int) {
	if game.level == 0 || index < 0 || index >= len(viewpoints) {
		return
	}
	nextHeld := game.keys[engine.KeyV]
	game.keys = [256]bool{}
	game.keys[engine.KeyV] = nextHeld
	if game.camera != nil {
		game.camera.Destroy()
	}
	p := viewpoints[index]
	game.camera = game.NewCamera3D(game.level, engine.CameraSettings{
		Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
		Position: engine.NewVec3D(p.x, p.y, p.z), Yaw: p.yaw, Pitch: p.pitch,
		FOVY: config.CameraFOVY, Near: config.CameraNear, Far: config.CameraFar,
		OrthoHeight:   config.CameraOrthoHeight,
		ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	})
	game.camera.SetLayer(-10)
	game.fps.SetCamera(game.camera)
	// The generated hook applies the host's accepted pose to camera.Settings.
	// Every walking proposal starts from that pose, including on the stairs.
	if _, err := game.WatchTransform(game.camera, func(engine.Transform3D) {}); err != nil {
		game.status = err.Error()
	} else {
		game.status = p.name
	}
	game.view = index
	if game.panel != nil {
		game.panel.Invalidate()
	}
}

func (game *Game) key(key engine.Key, down bool) {
	game.fps.Key(key, down)
	if key != engine.KeyV && key != engine.KeyH {
		return
	}
	held := game.keys[key]
	game.keys[key] = down
	if !down || held {
		return
	}
	if key == engine.KeyV {
		game.visit((game.view + 1) % len(viewpoints))
	}
	if key == engine.KeyH {
		if game.panel != nil {
			game.panel.Close()
			game.panel = nil
		} else {
			game.showPanel()
		}
	}
}

func (game *Game) update(frame engine.Frame) { game.fps.Update(frame) }

func (game *Game) stop() {
	game.fps.Close()
	if game.camera != nil {
		game.camera.Destroy()
	}
	if game.panel != nil {
		game.panel.Close()
	}
}
