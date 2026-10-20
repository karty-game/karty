package main

import (
	"example.com/hangar/.karty/config"
	"example.com/hangar/.karty/engine"
	"example.com/hangar/.karty/ui"
)

type Game struct {
	engine.Game
	camera         *engine.Camera3D
	torch          *engine.Light3D
	torchAim       lightAim
	torchEnabled   bool
	bolt           lightBolt
	level          engine.LevelHandle
	request        engine.RequestID
	panel          *engine.UI
	fps            *FPSController
	crates         [len(freightStacks)]*engine.Solid3D
	audio          hangarAudio
	audioPending   bool
	elevatorFrame  uint64
	keys           [256]bool
	view           int
	status         string
	cameraMenuOpen bool
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
			game.elevatorFrame = 0
			game.World(game.level).SetFloorZ("acid-control-west-lift#0", .2)
			if err := game.World(game.level).
				Configure(engine.WorldSettings{Mode: engine.WorldSpatial, Gravity: engine.Vec3{Z: -9.81}}); err != nil {
				panic(err)
			}
			game.createCrates()
			game.visit(0)
			game.audioPending = true
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
	game.torchEnabled = true
	game.fps = NewFPSController(&game.Game)
	game.fps.OnTouchMode = func() {
		if game.panel != nil {
			game.panel.Invalidate()
		}
	}
	game.showPanel()
	game.request = game.RequestLevel("levels.hangar")
}

func (game *Game) showPanel() {
	game.cameraMenuOpen = false
	game.panel = ui.Explorer(ui.ExplorerProps{
		Status: func() string { return game.status }, Controls: game.fps.Help,
		Visit: game.visit, Hide: func() { game.panel.Close(); game.panel = nil },
		CameraOpen:   func() bool { return game.cameraMenuOpen },
		TorchEnabled: func() bool { return game.torchEnabled },
		ToggleTorch:  game.toggleTorch,
		// LaunchLight: game.launchLight, // Point-light demo paused.
		ToggleCamera: func() {
			game.cameraMenuOpen = !game.cameraMenuOpen
			game.panel.Invalidate()
		},
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
	{"08 / ACID CONTROL ROOM", 31, 14.6, 5.1, 0, -.24},
	{"09 / BURNING BARREL", 12.6, -1.55, 1.9, 1.5708, -.2443},
	{"10 / UNDER THE ACID", 37, 23.2, -1.8, -1.5708, -.05},
	{"11 / FLOAT DEMO", 48.8, 22, 1.9, -1.5708, -.35},
}

func (game *Game) visit(index int) {
	if game.level == 0 || index < 0 || index >= len(viewpoints) {
		return
	}
	held := game.keys
	game.keys = [256]bool{}
	for _, key := range [...]engine.Key{engine.KeyV, engine.KeyF} {
		game.keys[key] = held[key]
	}
	game.clearLights()
	if game.camera != nil {
		game.camera.Destroy()
	}
	p := viewpoints[index]
	game.camera = game.NewCamera3D(game.level, engine.Camera3DSettings{
		Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
		Position: engine.NewVec3(p.x, p.y, p.z), Yaw: p.yaw, Pitch: p.pitch,
		VerticalFOV: config.CameraFOVY, NearClip: config.CameraNear, FarClip: config.CameraFar,
		OrthoHeight:   config.CameraOrthoHeight,
		ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	})
	game.camera.SetLayer(-10)
	// Keep the walking controller's floor and stair proposals; the body lets
	// accepted steps push the movable freight through the host's box solver.
	body, err := game.World(game.level).
		AttachBody(game.camera.Ref(), engine.BodySettings{Kind: engine.BodyDynamic, Collider: engine.BoxCollider{HalfExtents: engine.NewVec3(.3, .3, .85), Offset: engine.NewVec3(0, 0, -.85)}, AllowRotation: false, Forces: engine.BodyForceSettings{GravityScale: 1, Buoyancy: false}, Mass: 80, Friction: .5})
	if err != nil {
		panic(err)
	}
	game.torch = game.NewSpotLight(game.level, engine.NewVec3(p.x, p.y, p.z), engine.NewVec3(3, 2.7, 2.16), 18, .25, .50)
	// Camera-local axes are right, forward and up: hold the torch low on the right.
	game.torch.AttachTo(game.camera.Ref(), engine.NewVec3(.35, .20, -.30))
	game.torch.SetEnabled(game.torchEnabled)
	// Spotlight shadow maps require an ordinary Euclidean world. The loop
	// keeps the live beam while leaving its shadows disabled.
	game.torch.SetShadows(false)
	game.torchAim = lightAim{yaw: float64(p.yaw), pitch: float64(p.pitch), cameraYaw: float64(p.yaw), cameraPitch: float64(p.pitch)}
	game.fps.SetCamera(game.camera, body)
	// The generated hook applies the host's accepted pose to camera.Settings.
	// Every walking proposal starts from that pose, including on the stairs.
	if _, err := game.WatchPose(game.camera.Ref(), game.acceptTorchPose); err != nil {
		game.status = err.Error()
	} else {
		game.status = p.name
	}
	game.view = index
	game.cameraMenuOpen = false
	if game.panel != nil {
		game.panel.Invalidate()
	}
}

func (game *Game) key(key engine.Key, down bool) {
	game.fps.Key(key, down)
	if key != engine.KeyV && key != engine.KeyH && key != engine.KeyF {
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
	if key == engine.KeyF {
		game.toggleTorch()
	}
	// Point-light demo paused:
	// if key == engine.KeyG {
	// 	game.launchLight()
	// }
	if key == engine.KeyH {
		if game.panel != nil {
			game.panel.Close()
			game.panel = nil
		} else {
			game.showPanel()
		}
	}
}

func (game *Game) update(frame engine.Frame) {
	game.updateElevator()
	game.fps.Update(frame)
	game.updateTorch(frame.Number)
	if game.audioPending {
		game.createAudio()
		game.audioPending = false
	}
	game.updateAudio(frame.Number)
	// game.bolt.update() // Point-light demo paused.
}

func (game *Game) stop() {
	game.stopAudio()
	game.fps.Close()
	game.clearLights()
	for _, crate := range game.crates {
		if crate != nil {
			crate.Destroy()
		}
	}
	clear(game.crates[:])
	if game.camera != nil {
		game.camera.Destroy()
	}
	if game.panel != nil {
		game.panel.Close()
	}
}
