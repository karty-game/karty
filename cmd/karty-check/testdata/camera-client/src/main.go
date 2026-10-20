package main

import (
	"example.com/camera-fixture/.karty/engine"
	"example.com/camera-fixture/.karty/ui"
)

type Game struct {
	engine.Game
	cameraController
	cameraHUD
	marker                                             markerFollower
	spinner                                            spinnerAnimation
	request, release, tagQuery, spinQuery, markerQuery engine.RequestID
	level                                              engine.LevelHandle
	controls                                           *engine.UI
	status                                             string
	inspecting, touchMode, controlsHidden, channelHeld bool
	actorCount                                         uint8
	moveStick                                          moveStickOverlay
}

//go:wasmexport karty_register
func main() {
	game := &Game{}
	engine.Run(game.Hooks())
}

func (game *Game) Hooks() engine.Hooks {
	return engine.Hooks{
		OnStart: game.Initialize, OnUpdate: game.advanceMotion, OnStop: game.Shutdown,
		OnLevelReady: game.onLevelReady, OnEntityQueryResult: game.onActorQuery,
		OnLevelReleased: game.onLevelReleased, OnLevelFailed: game.onLevelFailed,
		OnKeyDown:        func(key engine.Key) { game.onKey(key, true) },
		OnKeyUp:          func(key engine.Key) { game.onKey(key, false) },
		OnPointer:        game.onPointer,
		OnInputReset:     game.resetInput,
		OnSequenceFailed: func(failure engine.SequenceFailure) { game.setStatus(failure.Err.Error()) },
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
		if _, err := game.WatchPose(game.camera.Ref(), game.cameraController.OnTransformChanged); err != nil {
			game.setStatus(err.Error())
			return
		}
		orbitSettings := isometricCamera(game.orbit)
		orbitSettings.Output = game.output
		game.orbitCamera = game.NewCamera3D(game.level, orbitSettings)
		game.orbitCamera.SetLayer(-10)
		game.orbitPose = orbitSettings
		if _, err := game.WatchPose(game.orbitCamera.Ref(), game.cameraController.OnOrbitTransformChanged); err != nil {
			game.setStatus(err.Error())
			return
		}
		game.orbitCamera.SetAlpha(0)
		game.tagQuery = game.World(game.level).FindEntitiesByTag("interactive", 8)
		game.spinQuery = game.World(game.level).FindEntitiesByTag("spinner", 1)
		game.markerQuery = game.World(game.level).FindEntitiesByTag("player-marker", 1)
		game.showCameraInfo()
		game.setStatus("Discovering actors...")
		if _, err := StartAuthoredSequence(game.level, "levels.camera-showcase", "welcome"); err != nil {
			game.setStatus(err.Error())
		}
	}
}
func (game *Game) onActorQuery(event engine.EntityQueryResult) {
	if event.Handle != game.level || game.release != 0 {
		return
	}
	if event.RequestID == game.markerQuery {
		if len(event.Entities) != 0 {
			game.marker.Bind(event.Entities[0])
			game.marker.Follow(cameraTransform(game.fps))
		}
	} else if event.RequestID == game.spinQuery {
		if len(event.Entities) != 0 {
			game.spinner.Bind(event.Entities[0])
		}
	} else if event.RequestID == game.tagQuery {
		game.actorCount = uint8(len(event.Entities))
		for _, actorID := range event.Entities {
			actorID.SetTags("interactive", "verified")
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
	if key == engine.KeyEscape && down {
		game.SetCursorCaptured(false)
		game.stopMotion()
	}
	if key == engine.KeyC && down {
		if game.controls == nil {
			game.showControls()
		} else {
			game.hideControls()
		}
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
	if key == engine.KeyI && down {
		game.inspecting = !game.inspecting
		game.stopMotion()
		game.invalidateControls()
	}
	if key == engine.KeyR && down {
		game.reload()
	}
	if key == engine.KeyHome && down {
		game.ResetOrbit()
	}
	if key == engine.KeyDigit1 && down {
		game.visit(true)
	}
	if key == engine.KeyDigit2 && down {
		game.visit(false)
	}
	switch key {
	case engine.KeyZ:
		game.left = down
	case engine.KeyX:
		game.right = down
	case engine.KeyLeft:
		game.lookLeft = down
	case engine.KeyRight:
		game.lookRight = down
	case engine.KeyArrowUp:
		game.lookUp = down
	case engine.KeyArrowDown:
		game.lookDown = down
	case engine.KeyA:
		game.strafeLeft = down
	case engine.KeyD:
		game.strafeRight = down
	case engine.KeyW:
		game.forward = down
	case engine.KeyS:
		game.backward = down
	case engine.KeyQ:
		game.orbitLeft = down
	case engine.KeyE:
		game.orbitRight = down
	case engine.KeyShiftLeft:
		game.runLeft = down
	case engine.KeyShiftRight:
		game.runRight = down
	case engine.KeyEqual:
		game.zoomIn = down
	case engine.KeyMinus:
		game.zoomOut = down
	}
	game.invalidateControls()
}
func (game *Game) advanceMotion(frame engine.Frame) {
	game.cameraController.Update()
	game.spinner.Update(frame.Number)
}

func (game *Game) showCameraInfo() {
	if game.cameraHUD.Refresh(&game.cameraController) && game.inspecting {
		game.invalidateControls()
	}
}

func (game *Game) Shutdown() {
	game.SetCursorCaptured(false)
	if game.moveStick.base != nil {
		game.moveStick.base.Destroy()
		game.moveStick.thumb.Destroy()
		game.moveStick = moveStickOverlay{}
	}
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
	settings := game.camera.Settings()
	settings.Output = game.output
	game.camera.SetSettings(settings)
	game.fps.Output = output
	game.orbitPose.Output = output
	settings = game.orbitCamera.Settings()
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
	game.setStatus(view)
}

func (game *Game) showControls() {
	game.controlsHidden = false
	game.showCameraInfo()
	game.controls = ui.Controls(ui.ControlsProps{
		Ready:      func() bool { return game.camera != nil },
		Help:       game.controlHelp,
		Inspecting: func() bool { return game.inspecting },
		Zoom:       game.Zoom,
		Reset:      game.ResetOrbit,
		Status:     func() string { return game.status },
		Pose:       func() string { return game.cameraInfo },
		Isometric:  func() bool { return game.isometric },
		Courtyard:  func() bool { return game.courtyard },
		View:       game.setView,
		Visit:      game.visit,
		Orbit:      game.RotateOrbit,
		Hide:       game.hideControls,
	})
}

func (game *Game) hideControls() {
	game.controlsHidden = true
	game.SetCursorCaptured(false)
	game.stopMotion()
	game.inspecting = false
	if game.controls != nil {
		game.controls.Close()
	}
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

func (game *Game) resetInput() {
	game.channelHeld = false
	game.stopMotion()
}

func (game *Game) stopMotion() {
	game.cameraController.Stop()
	game.hideMoveStick()
	game.invalidateControls()
}

func (game *Game) setView(isometric bool) {
	if game.camera == nil {
		return
	}
	game.SetCursorCaptured(false)
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
	game.SetCursorCaptured(false)
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
	game.SetCursorCaptured(false)
	game.courtyard, game.path, game.orbit = courtyard, 0, 0.9
	game.offsetX, game.offsetY, game.orthoHeight = 0, 0, 0
	game.camera.Destroy()
	game.fps = game.routeCamera(0)
	game.fps.Output = game.output
	game.camera = game.NewCamera3D(game.level, game.fps)
	game.camera.SetLayer(-10)
	if _, err := game.WatchPose(game.camera.Ref(), game.cameraController.OnTransformChanged); err != nil {
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
func tagAuthoredActor(context engine.ActionContext, actor engine.EntityRef, tag string) error {
	actor.SetTags(tag)
	return nil
}

//karty:condition actor.ready
func authoredActorReady(context engine.ActionContext, actor engine.EntityRef) (bool, error) {
	return actor.ID() != 0, nil
}
