package main

import (
	"math"

	"example.com/particle-lab/.karty/config"
	"example.com/particle-lab/.karty/engine"
)

// Game mounts the authored particles; it never sends a per-particle update.
type Game struct {
	engine.Game
	camera           *engine.EntityCamera3D
	panel            *engine.UI
	status           string
	level            engine.LevelHandle
	request, release engine.LevelRequestID
	keys             [256]bool
	view             int
	captured, hidden bool
	mode             int
	mouseX, mouseY   float32
	pointers         [2]cameraPointer
	mouseDragging    bool
}

//go:wasmexport karty_register
func main() { game := &Game{}; engine.Run(game.Hooks()) }

func (game *Game) Hooks() engine.Hooks {
	return engine.Hooks{
		OnStart: game.start, OnUpdate: game.update, OnStop: game.stop,
		OnKeyDown: func(key engine.Key) { game.key(key, true) },
		OnKeyUp:   func(key engine.Key) { game.key(key, false) },
		OnPointer: game.pointer,
		OnInputReset: func() {
			game.keys = [256]bool{}
			game.releasePointer()
		},
		OnLevelReady: game.ready, OnLevelFailed: game.failed,
		OnLevelReleased: game.released,
	}
}

func (game *Game) start() {
	game.mode, game.view = fireMode, 0
	game.status = "Loading effects"
	game.showControls()
	game.request = game.RequestLevel(game.levelName())
}

const (
	collectionMode = iota
	fireMode
	smokeMode
)

func (game *Game) levelName() string {
	if game.mode == fireMode {
		return "levels.brackeys-fire"
	}
	if game.mode == smokeMode {
		return "levels.smoke"
	}
	return "levels.lab"
}

type viewpoint struct {
	name                string
	x, y, z, yaw, pitch float32
}

// Diagonal close views separate each effect from the following row's stations.
// Welding and acid use shorter viewing distances to reveal their small details.
var viewpoints = [...]viewpoint{
	{"Brackeys combined fire", -1.25, -1.6, .85, 0, -.10},
	{"Electric sparkles", 1, -5.5, 2.1, -0.380506377, -0.36},
	{"Acid bubbles", 7, -5.5, 2.1, -0.380506377, -0.34},
	{"Reactor", -4.6, 0.8, 3.3, -0.412410442, -0.03},
	{"Electric discharge", 1.4, 0.8, 2.4, -0.412410442, -0.31},
	{"Explosion · Restart to replay", 7.4, 0.8, 2.4, -0.412410442, -0.31},
	{"Collection", 0, -10, 6.5, 0, -0.42},
	{"Brackeys separate fire", 1.25, -1.6, .85, 0, -.10},
	{"Brackeys fire comparison", 0, -5, 1.5, 0, -.10},
	{"Smoke comparison", 0, -10, 2.9, 0, 0},
	{"Thin smoke", -2.3, -3.2, 2.5, -0.2153577, 0},
	{"Rolling smoke", 4.4, -4.5, 2.85, -0.30161905, 0},
}

func (game *Game) ready(event engine.LevelReady) {
	if event.RequestID != game.request || game.request == 0 {
		game.ReleaseLevel(event.Handle)
		return
	}
	game.request, game.level = 0, event.Handle
	game.visit(game.view)
}

func (game *Game) failed(event engine.LevelFailed) {
	if event.RequestID == game.request && game.request != 0 {
		game.request = 0
	} else if event.RequestID == game.release && game.release != 0 {
		game.release = 0
		game.visit(game.view)
	} else {
		return
	}
	game.setStatus("Load failed · try Restart")
}

func (game *Game) released(event engine.LevelReleased) {
	if event.RequestID != game.release || game.release == 0 || event.Handle != game.level {
		return
	}
	game.level, game.release = 0, 0
	game.request = game.RequestLevel(game.levelName())
}

func (game *Game) visit(index int) {
	if index < 0 || index >= len(viewpoints) {
		return
	}
	game.view = index
	game.hidden = index == 8 || index == 9 || index == 6
	wanted := fireMode
	switch {
	case index >= 9:
		wanted = smokeMode
	case index >= 1 && index <= 6:
		wanted = collectionMode
	}
	if wanted != game.mode {
		game.mode = wanted
		game.restart()
		return
	}
	if game.level == 0 || game.release != 0 {
		return
	}
	game.releasePointer()
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
	if _, err := game.WatchTransform(game.camera, nil); err != nil {
		game.setStatus("Camera unavailable")
		return
	}
	game.setStatus(p.name)
}

// Restart waits for release before mounting a fresh particle timeline.
func (game *Game) restart() {
	if game.request != 0 || game.release != 0 {
		return
	}
	game.releasePointer()
	if game.camera != nil {
		game.camera.Destroy()
		game.camera = nil
	}
	game.setStatus("Restarting effects")
	if game.level != 0 {
		game.release = game.ReleaseLevel(game.level)
	} else {
		game.request = game.RequestLevel(game.levelName())
	}
}

func (game *Game) key(key engine.Key, down bool) {
	if int(key) >= len(game.keys) {
		return
	}
	held := game.keys[key]
	game.keys[key] = down
	if !down || held {
		return
	}
	switch {
	case key == engine.KeyV:
		game.visit((game.view + 1) % len(viewpoints))
	case key == engine.KeyM:
		game.visit(9)
	case key == engine.KeyDigit0 && game.mode == smokeMode:
		game.visit(9)
	case key == engine.KeyB || key == engine.KeyDigit0:
		game.visit(8)
	case game.mode == smokeMode && key >= engine.KeyDigit1 && key <= engine.KeyDigit2:
		game.visit(10 + int(key-engine.KeyDigit1))
	case game.mode == fireMode && key >= engine.KeyDigit1 && key <= engine.KeyDigit3:
		game.selectFire(int(key - engine.KeyDigit1))
	case key >= engine.KeyDigit1 && key <= engine.KeyDigit9:
		game.visit(int(key - engine.KeyDigit1))
	case key == engine.KeyF:
		if game.request == 0 && game.release == 0 {
			if game.mode == fireMode {
				game.selectView(collectionMode, 6)
			} else {
				game.selectView(fireMode, 0)
			}
		}
	case key == engine.KeyR:
		game.restart()
	case key == engine.KeyEscape:
		game.releasePointer()
	case key == engine.KeyH:
		if game.panel == nil {
			game.showControls()
		} else {
			game.panel.Close()
			game.panel = nil
		}
	}
}

func (game *Game) releasePointer() {
	game.SetCursorCaptured(false)
	game.captured = false
	game.mouseDragging = false
	game.pointers = [2]cameraPointer{}
	game.mouseX, game.mouseY = 0, 0
}

func axis(positive, negative bool) float32 {
	var result float32
	if positive {
		result++
	}
	if negative {
		result--
	}
	return result
}

func (game *Game) update(engine.Frame) {
	if game.camera == nil || game.release != 0 {
		return
	}
	forward := axis(game.keys[engine.KeyW], game.keys[engine.KeyS])
	strafe := axis(game.keys[engine.KeyD], game.keys[engine.KeyA])
	turn := axis(game.keys[engine.KeyRight], game.keys[engine.KeyLeft])
	tilt := axis(game.keys[engine.KeyArrowUp], game.keys[engine.KeyArrowDown])
	if forward == 0 && strafe == 0 && turn == 0 && tilt == 0 && game.mouseX == 0 && game.mouseY == 0 {
		return
	}
	settings := game.camera.Settings
	settings.Yaw += turn*0.025 + game.mouseX*0.0025
	settings.Pitch = min(max(settings.Pitch+tilt*0.025-game.mouseY*0.0025, -1.4), 1.4)
	game.mouseX, game.mouseY = 0, 0
	if length := math.Hypot(float64(forward), float64(strafe)); length > 1 {
		forward /= float32(length)
		strafe /= float32(length)
	}
	sine, cosine := float32(math.Sin(float64(settings.Yaw))), float32(math.Cos(float64(settings.Yaw)))
	settings.Position.X += (forward*sine + strafe*cosine) * 0.045
	settings.Position.Y += (forward*cosine - strafe*sine) * 0.045
	game.camera.SetSettings(settings)
}

func (game *Game) stop() {
	game.releasePointer()
	if game.request != 0 {
		game.CancelLevel(game.request)
	}
	if game.camera != nil {
		game.camera.Destroy()
	}
	if game.level != 0 && game.release == 0 {
		game.ReleaseLevel(game.level)
	}
	if game.panel != nil {
		game.panel.Close()
	}
}
