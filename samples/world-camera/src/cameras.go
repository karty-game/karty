package main

import (
	"math"

	"example.com/world-camera/.karty/config"
	"example.com/world-camera/.karty/engine"
)

// cameraController owns continuous movement. Accepted poses arrive through its
// entity hooks; UI and actor followers subscribe to those changes separately.
type cameraController struct {
	camera, orbitCamera                            *engine.EntityCamera3D
	fps, orbitPose                                 engine.CameraSettings
	path, orbit                                    float32
	touchMove, touchStrafe, touchOrbit, touchPitch float32
	left, right, orbitLeft, orbitRight             bool
	forward, backward, strafeLeft, strafeRight     bool
	courtyard, isometric                           bool
	output                                         engine.CameraOutput
	onMoved                                        func(engine.Transform3D)
	onOrbitMoved                                   func()
}

func (game *cameraController) Update() {
	if game.camera == nil {
		return
	}
	game.updateTour()
	orbitDirection := game.updateWalking()
	game.updateOrbit(orbitDirection)
}
func (game *cameraController) updateTour() {
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
		if game.path == previousPath {
			return
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
}
func (game *cameraController) updateWalking() float32 {
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
	return orbitDirection
}
func (game *cameraController) updateOrbit(orbitDirection float32) {
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
}

func (controller *cameraController) OnTransformChanged(pose engine.Transform3D) {
	if controller.camera == nil {
		return
	}
	controller.fps = controller.camera.Settings
	if controller.onMoved != nil {
		controller.onMoved(pose)
	}
}
func (controller *cameraController) OnOrbitTransformChanged(engine.Transform3D) {
	if controller.orbitCamera == nil {
		return
	}
	controller.orbitPose = controller.orbitCamera.Settings
	if controller.onOrbitMoved != nil {
		controller.onOrbitMoved()
	}
}
func (controller *cameraController) Stop() {
	controller.touchMove, controller.touchStrafe, controller.touchOrbit, controller.touchPitch = 0, 0, 0, 0
	controller.left, controller.right, controller.orbitLeft, controller.orbitRight = false, false, false, false
	controller.forward, controller.backward, controller.strafeLeft, controller.strafeRight = false, false, false, false
}
func (controller *cameraController) Close() {
	controller.Stop()
	if controller.camera != nil {
		controller.camera.Destroy()
	}
	if controller.orbitCamera != nil {
		controller.orbitCamera.Destroy()
	}
	controller.camera, controller.orbitCamera = nil, nil
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

func (game *cameraController) routeEnd() float32 {
	if game.courtyard {
		return float32(len(courtyardPath) - 1)
	}
	return float32(len(cameraPath) - 1)
}

func (game *cameraController) routeCamera(progress float32) engine.CameraSettings {
	if game.courtyard {
		return pathCamera(courtyardPath[:], progress)
	}
	return perspectiveCamera(progress)
}

func (game *cameraController) orbitSettings() engine.CameraSettings {
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

func (game *cameraController) outputName() string {
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
