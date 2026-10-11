package main

import (
	"strconv"

	"example.com/environment/.karty/config"
	"example.com/environment/.karty/engine"
	"example.com/environment/.karty/ui"
)

type Game struct {
	engine.Game
	camera      *engine.EntityCamera3D
	level       engine.LevelHandle
	request     engine.LevelRequestID
	panel       *engine.UI
	fps         *FPSController
	keys        [256]bool
	style, view int
	status      string
	timeOfDay   float32
	cloudAmount float32
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

var styles = [...]string{"daylight", "cartoon", "fantasy", "space"}

func (game *Game) start() {
	game.fps = NewFPSController(&game.Game)
	game.fps.SceneContains = func(x, y float32) bool {
		return game.panel == nil || (y > 270 && y < 500 && (x < float32(config.ResolutionWidth)-380 || y < 440))
	}
	game.fps.OnTouchMode = game.invalidate
	game.showPanel()
	game.selectStyle(0)
}

func (game *Game) showPanel() {
	game.panel = ui.Explorer(ui.ExplorerProps{
		Status: func() string { return game.status }, Controls: game.fps.Help,
		Style: func() string { return styles[game.style] }, SelectStyle: game.selectStyle,
		Visit: game.visit, Hide: func() { game.panel.Close(); game.panel = nil },
		Environment: game.environmentLabel, SetTime: game.setTime,
		CloudAmount: func() int32 { return int32(game.cloudAmount*100 + .5) }, SetCloudAmount: game.setCloudAmount,
		ResetEnvironment: game.resetEnvironment,
	})
}

func (game *Game) selectStyle(index int) {
	if index < 0 || index >= len(styles) {
		return
	}
	if game.request != 0 {
		game.CancelLevel(game.request)
	}
	game.fps.SetCamera(nil)
	if game.camera != nil {
		game.camera.Destroy()
		game.camera = nil
	}
	if game.level != 0 {
		game.ReleaseLevel(game.level)
		game.level = 0
	}
	game.style = index
	game.timeOfDay, game.cloudAmount = 15, .35
	game.status = "Preparing " + styles[index] + "..."
	game.invalidate()
	game.request = game.RequestLevel("levels." + styles[index])
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
	game.timeOfDay, game.cloudAmount = 15, .35
	game.applyEnvironment()
}

func (game *Game) applyEnvironment() {
	if game.level != 0 {
		game.SetEnvironment(game.level, engine.EnvironmentSettings{TimeOfDay: game.timeOfDay, CloudAmount: game.cloudAmount})
	}
	game.invalidate()
}

type viewpoint struct {
	name                string
	x, y, z, yaw, pitch float32
}

var viewpoints = [...]viewpoint{
	{"WINDOWS / passage on the right leads outside", 0, -8, 1.7, 0, .08},
	{"COURTYARD / the view through the windows", 1, 0, 1.7, -.18, .04},
	{"LOOK BACK / connected indoor observation room", 0, 12, 1.7, 3.14159, .06},
	{"TEST GROUND / future terrain and vegetation", 0, 32, 1.7, -.35, .18},
}

func (game *Game) visit(index int) {
	if game.level == 0 || index < 0 || index >= len(viewpoints) {
		return
	}
	game.fps.SetCamera(nil)
	if game.camera != nil {
		game.camera.Destroy()
	}
	p := viewpoints[index]
	game.camera = game.NewCamera3D(game.level, engine.CameraSettings{
		Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
		Position: engine.NewVec3D(p.x, p.y, p.z), Yaw: p.yaw, Pitch: p.pitch,
		FOVY: config.CameraFOVY, Near: config.CameraNear, Far: config.CameraFar,
		OrthoHeight: config.CameraOrthoHeight, ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	})
	game.camera.SetLayer(-10)
	game.fps.SetCamera(game.camera)
	if _, err := game.WatchTransform(game.camera, nil); err != nil {
		game.status = err.Error()
	} else {
		game.status = p.name
	}
	game.view = index
	game.invalidate()
}

func (game *Game) key(key engine.Key, down bool) {
	game.fps.Key(key, down)
	if key != engine.KeyV && key != engine.KeyP && key != engine.KeyH &&
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
	case engine.KeyP:
		game.selectStyle((game.style + 1) % len(styles))
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
		if game.panel != nil {
			game.panel.Close()
			game.panel = nil
		} else {
			game.showPanel()
		}
	}
}

func (game *Game) invalidate() {
	if game.panel != nil {
		game.panel.Invalidate()
	}
}

func (game *Game) stop() {
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
