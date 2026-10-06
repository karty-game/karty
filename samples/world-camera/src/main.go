package main

import (
	"example.com/world-camera/.karty/engine"
	"example.com/world-camera/.karty/ui"
)

type Game struct {
	engine.Game
	cameraController
	cameraHUD
	marker                                             markerFollower
	spinner                                            spinnerAnimation
	request, release, tagQuery, spinQuery, markerQuery engine.LevelRequestID
	level                                              engine.LevelHandle
	controls                                           *engine.UI
	status                                             string
	inspecting, hidControls, channelHeld               bool
	actorCount                                         uint8
}

//go:wasmexport karty_register
func main() {
	game := &Game{}
	engine.Run(game.Hooks())
}

func (game *Game) Hooks() engine.Hooks {
	return engine.Hooks{
		OnStart: game.Initialize, OnUpdate: game.advanceMotion, OnStop: game.Shutdown,
		OnLevelReady: game.onLevelReady, OnActorQueryResult: game.onActorQuery,
		OnLevelReleased: game.onLevelReleased, OnLevelFailed: game.onLevelFailed,
		OnKeyDown:         func(key engine.Key) { game.onKey(key, true) },
		OnKeyUp:           func(key engine.Key) { game.onKey(key, false) },
		OnPointerReleased: game.onPointerReleased,
		OnSequenceFailed:  func(failure engine.SequenceFailure) { game.setStatus(failure.Err.Error()) },
	}
}

func (game *Game) Initialize() {
	game.cameraController.onMoved = func(pose engine.Transform3D) {
		game.marker.Follow(pose)
		game.showCameraInfo()
	}
	game.cameraController.onOrbitMoved = game.showCameraInfo
	game.status = "Loading world..."
	game.showControls()
	game.request = game.RequestLevel("levels.camera-showcase")
}

func (game *Game) onLevelReady(event engine.LevelReady) {
	if event.RequestID == game.request && game.camera == nil {
		game.level = engine.LevelHandle(event.Handle)
		game.path = 0
		game.orbit = 0.9
		game.isometric = false
		game.courtyard = false
		game.stopMotion()
		game.fps = perspectiveCamera(game.path)
		if game.output == 0 {
			game.output = engine.CameraOutputFull
		}
		game.fps.Output = game.output
		game.camera = game.NewCamera3D(game.level, game.fps)
		game.camera.SetLayer(-10)
		if _, err := game.WatchTransform(game.camera, game.cameraController.OnTransformChanged); err != nil {
			game.setStatus(err.Error())
			return
		}
		orbitSettings := isometricCamera(game.orbit)
		orbitSettings.Output = game.output
		game.orbitCamera = game.NewCamera3D(game.level, orbitSettings)
		game.orbitCamera.SetLayer(-10)
		game.orbitPose = orbitSettings
		if _, err := game.WatchTransform(game.orbitCamera, game.cameraController.OnOrbitTransformChanged); err != nil {
			game.setStatus(err.Error())
			return
		}
		game.orbitCamera.SetAlpha(0)
		game.tagQuery = game.QueryWorldTag(game.level, "interactive", 8)
		game.spinQuery = game.QueryWorldTag(game.level, "spinner", 1)
		game.markerQuery = game.QueryWorldTag(game.level, "player-marker", 1)
		game.showCameraInfo()
		game.setStatus("Discovering actors...")
		if _, err := StartAuthoredSequence(game.level, "levels.camera-showcase", "welcome"); err != nil {
			game.setStatus(err.Error())
		}
	}
}
func (game *Game) onActorQuery(event engine.ActorQueryResult) {
	if event.Handle != game.level || game.release != 0 {
		return
	}
	if event.RequestID == game.markerQuery {
		if len(event.Actors) != 0 {
			game.marker.Bind(game.WorldActor(event.Actors[0]))
			game.marker.Follow(cameraTransform(game.fps))
		}
	} else if event.RequestID == game.spinQuery {
		if len(event.Actors) != 0 {
			game.spinner.Bind(game.WorldActor(event.Actors[0]))
		}
	} else if event.RequestID == game.tagQuery {
		game.actorCount = uint8(len(event.Actors))
		for _, actorID := range event.Actors {
			game.WorldActor(actorID).SetTags("interactive", "verified")
		}
		game.showStatus()
	}
}
func (game *Game) onLevelReleased(event engine.LevelReleased) {
	if event.RequestID == game.release {
		game.level = 0
		game.release = 0
		game.tagQuery = 0
		game.spinQuery = 0
		game.markerQuery = 0
		game.marker.Reset()
		game.spinner.Reset()
		game.actorCount = 0
		game.request = game.RequestLevel("levels.camera-showcase")
		game.setStatus("Remounting world...")
	}
}
func (game *Game) onLevelFailed(event engine.LevelFailed) {
	game.setStatus("Level failed: " + event.Diagnostic)
}
func (game *Game) onKey(key engine.Key, down bool) {
	if key == engine.KeyEscape && down && game.controls != nil {
		game.hideControls()
	}
	if key == engine.KeyC && down && game.controls == nil {
		game.stopMotion()
		game.showControls()
	}
	if key == engine.KeyV && down && game.camera != nil {
		game.setView(!game.isometric)
	}
	if key == engine.KeyG {
		if down && !game.channelHeld && game.camera != nil {
			game.cycleOutput()
		}
		game.channelHeld = down
	}
	if key == engine.KeyLeft {
		game.left = down && game.controls == nil
	}
	if key == engine.KeyRight {
		game.right = down && game.controls == nil
	}
	if key == engine.KeyA {
		game.strafeLeft = down
	}
	if key == engine.KeyD {
		game.strafeRight = down
	}
	if key == engine.KeyW {
		game.forward = down
	}
	if key == engine.KeyS {
		game.backward = down
	}
	if key == engine.KeyQ {
		game.orbitLeft = down
	}
	if key == engine.KeyE {
		game.orbitRight = down
	}
	game.invalidateControls()
}
func (game *Game) onPointerReleased(event engine.PointerEvent) {
	if event.Button == engine.PointerPrimary && game.controls == nil && !game.hidControls {
		game.stopMotion()
		game.showControls()
		return
	}
	if event.Button == engine.PointerSecondary && game.controls == nil && game.camera != nil && game.level != 0 {
		game.reload()
	}
}
func (game *Game) advanceMotion(frame engine.Frame) {
	game.hidControls = false
	game.cameraController.Update()
	game.spinner.Update(frame.Number)
}

func (game *Game) showCameraInfo() {
	if game.cameraHUD.Refresh(&game.cameraController) && game.inspecting {
		game.invalidateControls()
	}
}

func (game *Game) Shutdown() {
	game.cameraController.Close()
	game.marker.Reset()
	game.spinner.Reset()
	if game.controls != nil {
		game.controls.Close()
	}
}

func (game *Game) cycleOutput() {
	output := game.output + 1
	if output > engine.CameraOutputDepth {
		output = engine.CameraOutputFull
	}
	game.setOutput(output)
}

func (game *Game) setOutput(output engine.CameraOutput) {
	if game.camera == nil {
		return
	}
	game.output = output
	settings := game.camera.Settings
	settings.Output = game.output
	game.camera.SetSettings(settings)
	game.fps.Output = output
	game.orbitPose.Output = output
	settings = game.orbitCamera.Settings
	settings.Output = game.output
	game.orbitCamera.SetSettings(settings)
	game.showCameraInfo()
	game.showStatus()
}

func (game *Game) showStatus() {
	view := "Gallery / FPS"
	if game.courtyard {
		view = "Roman court / FPS"
	}
	if game.isometric {
		view = "Gallery / Isometric"
		if game.courtyard {
			view = "Roman court / Isometric"
		}
	}
	game.setStatus(view + " / " + game.outputName())
}

func (game *Game) showControls() {
	game.showCameraInfo()
	game.controls = ui.Controls(ui.ControlsProps{
		Ready:  func() bool { return game.camera != nil },
		Status: func() string { return game.status },
		Pose:   func() string { return game.cameraInfo },
		Inspect: func(show bool) {
			game.inspecting = show
			if show {
				game.stopMotion()
			}
		},
		Isometric: func() bool { return game.isometric },
		Courtyard: func() bool { return game.courtyard },
		Output:    func() int { return int(game.output) },
		Moving: func() bool {
			return game.touchMove != 0 || game.touchStrafe != 0 || game.touchOrbit != 0 || game.touchPitch != 0 ||
				game.forward || game.backward || game.strafeLeft || game.strafeRight || game.orbitLeft || game.orbitRight
		},
		View:    game.setView,
		Channel: func(output int) { game.setOutput(engine.CameraOutput(output)) },
		Visit:   game.visit,
		Move: func(forward, strafe float32) {
			game.touchMove, game.touchStrafe, game.touchOrbit, game.touchPitch = forward, strafe, 0, 0
			game.invalidateControls()
		},
		Orbit: func(direction float32) {
			game.touchOrbit, game.touchMove, game.touchStrafe, game.touchPitch = direction, 0, 0, 0
			game.invalidateControls()
		},
		Look: func(direction float32) {
			game.touchPitch, game.touchMove, game.touchStrafe, game.touchOrbit = direction, 0, 0, 0
			game.invalidateControls()
		},
		Stop:   game.stopMotion,
		Reload: game.reload,
		Hide:   game.hideControls,
	})
}

func (game *Game) hideControls() {
	game.stopMotion()
	game.inspecting = false
	game.hidControls = true
	game.controls.Close()
	game.controls = nil
}

func (game *Game) invalidateControls() {
	if game.controls != nil {
		game.controls.Invalidate()
	}
}

func (game *Game) setStatus(status string) {
	game.status = status
	game.invalidateControls()
}

func (game *Game) stopMotion() {
	game.cameraController.Stop()
	game.invalidateControls()
}

func (game *Game) setView(isometric bool) {
	if game.camera == nil {
		return
	}
	game.stopMotion()
	game.isometric = isometric
	if isometric {
		game.camera.SetAlpha(0)
		game.orbitCamera.SetAlpha(1)
	} else {
		game.orbitCamera.SetAlpha(0)
		game.camera.SetAlpha(1)
	}
	game.showCameraInfo()
	game.showStatus()
}

func (game *Game) reload() {
	if game.camera == nil || game.level == 0 {
		return
	}
	game.stopMotion()
	game.cameraController.Close()
	game.marker.Reset()
	game.spinner.Reset()
	game.cameraHUD.Clear()
	game.release = game.ReleaseLevel(game.level)
	game.setStatus("Releasing world...")
}

// Recreate the FPS entity for a viewpoint jump; walking still uses the host's
// accepted movement through portals, including the gallery's folded loop.
func (game *Game) visit(courtyard bool) {
	if game.camera == nil {
		return
	}
	game.stopMotion()
	game.courtyard, game.path, game.orbit = courtyard, 0, 0.9
	game.camera.Destroy()
	game.fps = game.routeCamera(0)
	game.fps.Output = game.output
	game.camera = game.NewCamera3D(game.level, game.fps)
	game.camera.SetLayer(-10)
	if _, err := game.WatchTransform(game.camera, game.cameraController.OnTransformChanged); err != nil {
		game.setStatus(err.Error())
		return
	}
	settings := game.orbitSettings()
	settings.Output = game.output
	game.orbitCamera.SetSettings(settings)
	game.orbitPose = settings
	game.marker.Follow(cameraTransform(game.fps))
	game.setView(game.isometric)
}

//karty:action actor.tag
func tagAuthoredActor(context engine.ActionContext, actor engine.WorldActorRef, tag string) error {
	actor.SetTags(tag)
	return nil
}

//karty:condition actor.ready
func authoredActorReady(context engine.ActionContext, actor engine.WorldActorRef) (bool, error) {
	return actor.ID != 0, nil
}
