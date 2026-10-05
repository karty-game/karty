package main

import (
	"math"
	"strconv"

	"example.com/world-camera/.karty/config"
	"example.com/world-camera/.karty/engine"
	"example.com/world-camera/.karty/ui"
)

type Game struct {
	engine.Game

	request      engine.LevelRequestID
	release      engine.LevelRequestID
	tagQuery     engine.LevelRequestID
	spinQuery    engine.LevelRequestID
	markerQuery  engine.LevelRequestID
	level        engine.LevelHandle
	spinner      engine.WorldActorID
	playerMarker engine.WorldActorID
	camera       *engine.EntityCamera3D
	orbitCamera  *engine.EntityCamera3D
	controls     *engine.UI
	status       string
	cameraInfo   string
	touchMove    float32
	touchStrafe  float32
	touchOrbit   float32
	touchPitch   float32
	courtyard    bool
	inspecting   bool
	hidControls  bool
	infoBuffer   [1024]byte
	path         float32
	fps          engine.CameraSettings
	orbit        float32
	left         bool
	right        bool
	orbitLeft    bool
	orbitRight   bool
	forward      bool
	backward     bool
	strafeLeft   bool
	strafeRight  bool
	isometric    bool
	output       engine.CameraOutput
	channelHeld  bool
	actorCount   uint8
}

type cameraPoint struct{ x, y, z, yaw float32 }

var cameraPath = [...]cameraPoint{
	{x: 0.1, y: 1, z: 1.72, yaw: 1.5707963},
	{x: 4.5, y: 1, z: 1.9, yaw: 1.5707963},
	{x: 8.5, y: 1, z: 1.9, yaw: 1.5707963},
	{x: 12.5, y: 1, z: 1.9, yaw: 1.5707963},
	{x: 17, y: 1, z: 1.99, yaw: 0},
	{x: 17, y: 3.5, z: 2.23, yaw: 0},
	{x: 17, y: 8, z: 2.79, yaw: 0},
	{x: 17, y: 11, z: 2.79, yaw: -1.5707963},
}

// Keep the walking loop on the paving around the raised basin.
var courtyardPath = [...]cameraPoint{
	{x: -5, y: 2, z: 1.7, yaw: -0.9827937},
	{x: -5, y: 1, z: 1.7, yaw: -1.5707963},
	{x: -11, y: 1, z: 1.7, yaw: 0},
	{x: -11, y: 7, z: 1.7, yaw: 1.5707963},
	{x: -5, y: 7, z: 1.7, yaw: 3.1415927},
	{x: -5, y: 2, z: 1.7, yaw: -0.9827937},
}

//go:wasmexport karty_register
func main() { engine.Run(&Game{}) }

func (game *Game) Initialize() {
	game.status = "Loading world..."
	game.showControls()
	game.request = game.RequestLevel("levels.camera-showcase")
}

func (game *Game) Update(frame engine.Frame) {
	hiddenThisFrame := game.hidControls
	game.hidControls = false
	for _, event := range frame.Events {
		switch event.Type {
		case engine.EventLevelReady:
			if event.RequestID == uint32(game.request) && game.camera == nil {
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
				orbitSettings := isometricCamera(game.orbit)
				orbitSettings.Output = game.output
				game.orbitCamera = game.NewCamera3D(game.level, orbitSettings)
				game.orbitCamera.SetLayer(-10)
				game.orbitCamera.SetAlpha(0)
				game.tagQuery = game.QueryWorldTag(game.level, "interactive", 8)
				game.spinQuery = game.QueryWorldTag(game.level, "spinner", 1)
				game.markerQuery = game.QueryWorldTag(game.level, "player-marker", 1)
				game.setStatus("Discovering actors...")
			}
		case engine.EventAuthoritativeTransform:
			if game.camera != nil && game.camera.ApplyAuthoritativeTransform(event) {
				game.fps = game.camera.Settings
			}
		case engine.EventWorldTagQueryResult:
			if event.RequestID == uint32(game.markerQuery) {
				if event.ActorCount != 0 {
					game.playerMarker = event.ActorIDs[0]
				}
			} else if event.RequestID == uint32(game.spinQuery) {
				if event.ActorCount != 0 {
					game.spinner = event.ActorIDs[0]
				}
			} else if event.RequestID == uint32(game.tagQuery) {
				game.actorCount = event.ActorCount
				for _, actorID := range event.ActorIDs[:event.ActorCount] {
					game.WorldActor(actorID).SetTags("interactive", "verified")
				}
				game.showStatus()
			}
		case engine.EventLevelReleased:
			if event.RequestID == uint32(game.release) {
				game.level = 0
				game.release = 0
				game.tagQuery = 0
				game.spinQuery = 0
				game.markerQuery = 0
				game.playerMarker = 0
				game.spinner = 0
				game.actorCount = 0
				game.request = game.RequestLevel("levels.camera-showcase")
				game.setStatus("Remounting world...")
			}
		case engine.EventLevelFailed:
			game.setStatus("Level failed: " + event.Diagnostic)
		case engine.EventKeyDown, engine.EventKeyUp:
			down := event.Type == engine.EventKeyDown
			if event.Key == engine.KeyEscape && down && game.controls != nil {
				game.hideControls()
			}
			if event.Key == engine.KeyC && down && game.controls == nil {
				game.stopMotion()
				game.showControls()
			}
			if event.Key == engine.KeyV && down && game.camera != nil {
				game.setView(!game.isometric)
			}
			if event.Key == engine.KeyG {
				if down && !game.channelHeld && game.camera != nil {
					game.cycleOutput()
				}
				game.channelHeld = down
			}
			if event.Key == engine.KeyLeft {
				game.left = down && game.controls == nil
			}
			if event.Key == engine.KeyRight {
				game.right = down && game.controls == nil
			}
			if event.Key == engine.KeyA {
				game.strafeLeft = down
			}
			if event.Key == engine.KeyD {
				game.strafeRight = down
			}
			if event.Key == engine.KeyW {
				game.forward = down
			}
			if event.Key == engine.KeyS {
				game.backward = down
			}
			if event.Key == engine.KeyQ {
				game.orbitLeft = down
			}
			if event.Key == engine.KeyE {
				game.orbitRight = down
			}
			game.invalidateControls()
		case engine.EventPointerUp:
			if event.Button == engine.PointerPrimary && game.controls == nil && !hiddenThisFrame {
				game.stopMotion()
				game.showControls()
				continue
			}
			if event.Button == engine.PointerSecondary && game.controls == nil && game.camera != nil && game.level != 0 {
				game.reload()
			}
		}
	}

	// Arrow keys retain the authored diagnostic tour. Normal walking uses the
	// independent yaw-relative controls below and is not clamped to this path.
	var direction float32
	if game.left {
		direction--
	}
	if game.right {
		direction++
	}
	if direction != 0 && game.camera != nil {
		previousPath := game.path
		game.path += direction * 0.025
		if game.path < 0 {
			game.path = 0
		} else if game.path > game.routeEnd() {
			game.path = game.routeEnd()
		}
		from, to := game.routeCamera(previousPath), game.routeCamera(game.path)
		angle := float64(game.fps.Yaw - from.Yaw)
		sine, cosine := float32(math.Sin(angle)), float32(math.Cos(angle))
		deltaX, deltaY := to.Position.X-from.Position.X, to.Position.Y-from.Position.Y
		proposed := game.fps
		proposed.Position.X += cosine*deltaX - sine*deltaY
		proposed.Position.Y += sine*deltaX + cosine*deltaY
		proposed.Position.Z += to.Position.Z - from.Position.Z
		proposed.Yaw += to.Yaw - from.Yaw
		game.camera.SetSettings(proposed)
	}

	orbitDirection := game.touchOrbit
	if game.orbitLeft {
		orbitDirection--
	}
	if game.orbitRight {
		orbitDirection++
	}
	if game.isometric {
		if game.strafeLeft {
			orbitDirection--
		}
		if game.strafeRight {
			orbitDirection++
		}
	}
	forward, strafe := game.touchMove, game.touchStrafe
	if game.forward {
		forward++
	}
	if game.backward {
		forward--
	}
	if !game.isometric {
		if game.strafeLeft {
			strafe--
		}
		if game.strafeRight {
			strafe++
		}
	}
	if game.camera != nil && (forward != 0 || strafe != 0 || (!game.isometric && (orbitDirection != 0 || game.touchPitch != 0))) {
		settings := game.camera.Settings
		if !game.isometric {
			settings.Yaw += orbitDirection * .025
			settings.Pitch = min(max(settings.Pitch+game.touchPitch*.025, -1.45), 1.45)
		}
		game.camera.SetSettings(walkingCamera(settings, forward, strafe))
	}
	if orbitDirection != 0 && game.isometric && game.orbitCamera != nil {
		game.orbit += orbitDirection * 0.012
		if game.orbit < 0 {
			game.orbit += 2 * math.Pi
		} else if game.orbit >= 2*math.Pi {
			game.orbit -= 2 * math.Pi
		}
		settings := game.orbitSettings()
		settings.Output = game.output
		game.orbitCamera.SetSettings(settings)
	}

	if game.playerMarker != 0 && game.camera != nil {
		// Keep the marker behind the FPS eye, including its pitch. Isometric
		// orbit changes only the viewing camera, leaving the FPS marker in place.
		const behind = 0.16
		yaw, pitch := float64(game.fps.Yaw), float64(game.fps.Pitch)
		position := game.fps.Position
		position.X -= behind * float32(math.Sin(yaw)*math.Cos(pitch))
		position.Y -= behind * float32(math.Cos(yaw)*math.Cos(pitch))
		position.Z -= behind * float32(math.Sin(pitch))
		game.WorldActor(game.playerMarker).SetTransform(engine.NewTransform3D(position))
	}

	if game.spinner != 0 {
		transform := engine.NewTransform3D(engine.NewVec3D(6, 0.65, 0.33))
		transform.Yaw = float32(frame.Number%314) * 0.02
		game.WorldActor(game.spinner).SetTransform(transform)
	}
	game.showCameraInfo()
}

func (game *Game) Shutdown() {
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
	game.fps = settings
	settings = game.orbitCamera.Settings
	settings.Output = game.output
	game.orbitCamera.SetSettings(settings)
	game.showStatus()
}

func (game *Game) outputName() string {
	switch game.output {
	case engine.CameraOutputAlbedo:
		return "albedo"
	case engine.CameraOutputNormal:
		return "normal"
	case engine.CameraOutputDepth:
		return "depth"
	default:
		return "full"
	}
}

// Read the entity settings, including host-authoritative portal corrections.
// Reuse formatting storage and send text only when the displayed values change.
func (game *Game) showCameraInfo() {
	if game.camera == nil {
		return
	}
	settings := game.camera.Settings
	text := appendCameraPose(game.infoBuffer[:0], "FPS", settings)
	if game.isometric && game.orbitCamera != nil {
		settings = game.orbitCamera.Settings
		text = append(text, '\n')
		text = appendCameraPose(text, "ISO", settings)
	}
	text = append(text, "\nView: "...)
	if game.isometric {
		text = append(text, "isometric"...)
	} else {
		text = append(text, "perspective"...)
	}
	text = append(text, "\nChannel: "...)
	text = append(text, game.outputName()...)
	for _, field := range [...]struct {
		name  string
		value float32
	}{{"\nFOVy(rad)=", settings.FOVY}, {" ortho=", settings.OrthoHeight}, {"\nnear=", settings.Near}, {" far=", settings.Far}, {" path=", game.path}} {
		text = append(text, field.name...)
		text = strconv.AppendFloat(text, float64(field.value), 'g', -1, 32)
	}
	text = append(text, "\nviewport="...)
	text = strconv.AppendUint(text, uint64(settings.ViewportWidth), 10)
	text = append(text, 'x')
	text = strconv.AppendUint(text, uint64(settings.ViewportHeight), 10)
	if string(text) != game.cameraInfo {
		game.cameraInfo = string(text)
		if game.inspecting {
			game.invalidateControls()
		}
	}
}

func appendCameraPose(text []byte, name string, settings engine.CameraSettings) []byte {
	text = append(text, name...)
	for _, field := range [...]struct {
		name  string
		value float32
	}{{"\nx=", settings.Position.X}, {" y=", settings.Position.Y}, {"\nz=", settings.Position.Z}, {"\nyaw(rad)=", settings.Yaw}, {"\npitch(rad)=", settings.Pitch}} {
		text = append(text, field.name...)
		text = strconv.AppendFloat(text, float64(field.value), 'g', -1, 32)
	}
	return text
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
	game.touchMove, game.touchStrafe, game.touchOrbit, game.touchPitch = 0, 0, 0, 0
	game.left, game.right, game.orbitLeft, game.orbitRight = false, false, false, false
	game.forward, game.backward, game.strafeLeft, game.strafeRight = false, false, false, false
	game.invalidateControls()
}

func (game *Game) setView(isometric bool) {
	if game.camera == nil {
		return
	}
	game.stopMotion()
	game.isometric = isometric
	game.fps = game.camera.Settings
	if isometric {
		game.camera.SetAlpha(0)
		game.orbitCamera.SetAlpha(1)
	} else {
		game.orbitCamera.SetAlpha(0)
		game.camera.SetAlpha(1)
	}
	game.showStatus()
}

func (game *Game) reload() {
	if game.camera == nil || game.level == 0 {
		return
	}
	game.stopMotion()
	game.camera.Destroy()
	game.orbitCamera.Destroy()
	game.orbitCamera, game.camera = nil, nil
	game.cameraInfo = ""
	game.release = game.ReleaseLevel(game.level)
	game.setStatus("Releasing world...")
}

// Recreate the FPS entity for a viewpoint jump; walking still uses the host's
// authoritative movement through portals, including the gallery's folded loop.
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
	settings := game.orbitSettings()
	settings.Output = game.output
	game.orbitCamera.SetSettings(settings)
	game.setView(game.isometric)
}

func (game *Game) routeEnd() float32 {
	if game.courtyard {
		return float32(len(courtyardPath) - 1)
	}
	return float32(len(cameraPath) - 1)
}

func (game *Game) routeCamera(progress float32) engine.CameraSettings {
	if game.courtyard {
		return pathCamera(courtyardPath[:], progress)
	}
	return perspectiveCamera(progress)
}

func (game *Game) orbitSettings() engine.CameraSettings {
	if game.courtyard {
		settings := centeredIsometricCamera(game.orbit, -8, 4)
		settings.Position.Z = 22
		return settings
	}
	return isometricCamera(game.orbit)
}

func perspectiveCamera(progress float32) engine.CameraSettings {
	return pathCamera(cameraPath[:], progress)
}

func walkingCamera(settings engine.CameraSettings, forward, strafe float32) engine.CameraSettings {
	if length := math.Hypot(float64(forward), float64(strafe)); length > 1 {
		forward /= float32(length)
		strafe /= float32(length)
	}
	sine, cosine := float32(math.Sin(float64(settings.Yaw))), float32(math.Cos(float64(settings.Yaw)))
	settings.Position.X += (forward*sine + strafe*cosine) * .04
	settings.Position.Y += (forward*cosine - strafe*sine) * .04

	return settings
}

func pathCamera(path []cameraPoint, progress float32) engine.CameraSettings {
	segment := int(progress)
	if segment >= len(path)-1 {
		segment = len(path) - 2
		progress = float32(len(path) - 1)
	}
	amount := progress - float32(segment)
	from, to := path[segment], path[segment+1]
	settings := engine.CameraSettings{
		Projection: engine.CameraProjectionPerspective,
		Output:     engine.CameraOutputFull,
		Position: engine.NewVec3D(
			from.x+(to.x-from.x)*amount,
			from.y+(to.y-from.y)*amount,
			from.z+(to.z-from.z)*amount,
		),
		Yaw: from.yaw + (to.yaw-from.yaw)*amount, Pitch: -0.08, FOVY: config.CameraFOVY, OrthoHeight: config.CameraOrthoHeight,
		Near: config.CameraNear, Far: config.CameraFar, ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	}

	return settings
}

func isometricCamera(angle float32) engine.CameraSettings {
	return centeredIsometricCamera(angle, 9, 2.5)
}

func centeredIsometricCamera(angle, centerX, centerY float32) engine.CameraSettings {
	const radius = 20
	x := centerX - float32(math.Sin(float64(angle)))*radius
	y := centerY - float32(math.Cos(float64(angle)))*radius

	return engine.CameraSettings{
		Projection: engine.CameraProjectionIsometric,
		Output:     engine.CameraOutputFull,
		Position:   engine.NewVec3D(x, y, 18),
		Yaw:        angle, Pitch: -0.6154797, FOVY: config.CameraFOVY, OrthoHeight: config.CameraOrthoHeight,
		Near: config.CameraNear, Far: config.CameraFar, ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	}
}
