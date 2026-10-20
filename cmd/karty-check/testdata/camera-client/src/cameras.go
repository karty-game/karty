package main

import (
	"math"

	"example.com/camera-fixture/.karty/config"
	"example.com/camera-fixture/.karty/engine"
)

// cameraController owns continuous movement. Accepted poses arrive through its
// entity hooks; UI and actor followers subscribe to those changes separately.
type cameraController struct {
	camera, orbitCamera                            *engine.Camera3D
	fps, orbitPose                                 engine.Camera3DSettings
	path, orbit                                    float32
	touchMove, touchStrafe, touchOrbit, touchPitch float32
	left, right, orbitLeft, orbitRight             bool
	forward, backward, strafeLeft, strafeRight     bool
	lookLeft, lookRight, lookUp, lookDown          bool
	runLeft, runRight, zoomIn, zoomOut             bool
	mouseX, mouseY, panX, panY                     float32
	orthoHeight, offsetX, offsetY                  float32
	mouseDragging                                  bool
	pointers                                       [2]cameraPointer
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
	// Z/X retain the authored diagnostic tour. Normal walking uses the
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
	turn := game.touchOrbit + axis(game.orbitRight, game.orbitLeft)
	forward := game.touchMove + axis(game.forward, game.backward)
	strafe := game.touchStrafe + axis(game.strafeRight, game.strafeLeft)
	if game.isometric {
		game.panX += (strafe + axis(game.lookRight, game.lookLeft)) * .12
		game.panY += (forward + axis(game.lookUp, game.lookDown)) * .12
		game.mouseX, game.mouseY = 0, 0
		return turn
	}
	settings := game.camera.Settings()
	settings.Yaw += (turn+axis(game.lookRight, game.lookLeft))*.025 + game.mouseX*.0025
	settings.Pitch = max(-1.35, min(1.35, settings.Pitch+(game.touchPitch+axis(game.lookUp, game.lookDown))*.025-game.mouseY*.0025))
	game.mouseX, game.mouseY = 0, 0
	next := walkingCamera(settings, forward, strafe)
	if game.runLeft || game.runRight {
		next.Position.X += next.Position.X - settings.Position.X
		next.Position.Y += next.Position.Y - settings.Position.Y
	}
	if next != game.camera.Settings() {
		game.camera.SetSettings(next)
	}
	return turn
}
func (game *cameraController) updateOrbit(direction float32) {
	if !game.isometric || game.orbitCamera == nil {
		return
	}
	zoom := axis(game.zoomOut, game.zoomIn)
	if direction == 0 && game.panX == 0 && game.panY == 0 && zoom == 0 {
		return
	}
	game.orbit = float32(math.Mod(float64(game.orbit+direction*.012)+2*math.Pi, 2*math.Pi))
	sine, cosine := float32(math.Sin(float64(game.orbit))), float32(math.Cos(float64(game.orbit)))
	game.offsetX = max(-40, min(40, game.offsetX+game.panX*cosine+game.panY*sine))
	game.offsetY = max(-40, min(40, game.offsetY-game.panX*sine+game.panY*cosine))
	game.panX, game.panY = 0, 0
	game.orthoHeight = max(8, min(60, game.orbitSettings().OrthoHeight*(1+zoom*.02)))
	game.orbitCamera.SetSettings(game.orbitSettings())
}
func (game *cameraController) Zoom(factor float32) {
	if !game.isometric || game.orbitCamera == nil {
		return
	}
	game.orthoHeight = max(8, min(60, game.orbitSettings().OrthoHeight*factor))
	game.orbitCamera.SetSettings(game.orbitSettings())
}
func (game *cameraController) RotateOrbit(direction float32) {
	if !game.isometric || game.orbitCamera == nil {
		return
	}
	game.orbit = float32(math.Mod(float64(game.orbit+direction*.2)+2*math.Pi, 2*math.Pi))
	game.orbitCamera.SetSettings(game.orbitSettings())
}

func (game *cameraController) ResetOrbit() {
	game.offsetX, game.offsetY, game.orthoHeight, game.orbit = 0, 0, config.CameraOrthoHeight, .9
	if game.orbitCamera != nil {
		game.orbitCamera.SetSettings(game.orbitSettings())
	}
}
func axis(positive, negative bool) float32 {
	var value float32
	if positive {
		value++
	}
	if negative {
		value--
	}
	return value
}

func (controller *cameraController) OnTransformChanged(pose engine.Transform3D) {
	if controller.camera == nil {
		return
	}
	controller.fps = controller.camera.Settings()
	if controller.onMoved != nil {
		controller.onMoved(pose)
	}
}
func (controller *cameraController) OnOrbitTransformChanged(engine.Transform3D) {
	if controller.orbitCamera == nil {
		return
	}
	controller.orbitPose = controller.orbitCamera.Settings()
	if controller.onOrbitMoved != nil {
		controller.onOrbitMoved()
	}
}
func (controller *cameraController) Stop() {
	controller.lookLeft, controller.lookRight, controller.lookUp, controller.lookDown = false, false, false, false
	controller.runLeft, controller.runRight, controller.zoomIn, controller.zoomOut = false, false, false, false
	controller.mouseX, controller.mouseY, controller.panX, controller.panY = 0, 0, 0, 0
	controller.mouseDragging = false
	controller.pointers = [2]cameraPointer{}
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

func (game *cameraController) routeCamera(progress float32) engine.Camera3DSettings {
	if game.courtyard {
		return pathCamera(courtyardPath[:], progress)
	}
	return perspectiveCamera(progress)
}

func (game *cameraController) orbitSettings() engine.Camera3DSettings {
	settings := isometricCamera(game.orbit)
	if game.courtyard {
		settings = centeredIsometricCamera(game.orbit, -8, 4)
		settings.Position.Z = 22
	}
	settings.Position.X += game.offsetX
	settings.Position.Y += game.offsetY
	if game.orthoHeight != 0 {
		settings.OrthoHeight = game.orthoHeight
	}
	settings.Output = game.output
	return settings
}

func perspectiveCamera(progress float32) engine.Camera3DSettings {
	return pathCamera(cameraPath[:], progress)
}

func walkingCamera(settings engine.Camera3DSettings, forward, strafe float32) engine.Camera3DSettings {
	if length := math.Hypot(float64(forward), float64(strafe)); length > 1 {
		forward /= float32(length)
		strafe /= float32(length)
	}
	sine, cosine := float32(math.Sin(float64(settings.Yaw))), float32(math.Cos(float64(settings.Yaw)))
	settings.Position.X += (forward*sine + strafe*cosine) * .04
	settings.Position.Y += (forward*cosine - strafe*sine) * .04

	return settings
}

func pathCamera(path []cameraPoint, progress float32) engine.Camera3DSettings {
	segment := int(progress)
	if segment >= len(path)-1 {
		segment = len(path) - 2
		progress = float32(len(path) - 1)
	}
	amount := progress - float32(segment)
	from, to := path[segment], path[segment+1]
	settings := engine.Camera3DSettings{
		Projection: engine.CameraProjectionPerspective,
		Output:     engine.CameraOutputFull,
		Position: engine.NewVec3(
			from.x+(to.x-from.x)*amount,
			from.y+(to.y-from.y)*amount,
			from.z+(to.z-from.z)*amount,
		),
		Yaw:            from.yaw + (to.yaw-from.yaw)*amount,
		Pitch:          -0.08,
		VerticalFOV:    config.CameraFOVY,
		OrthoHeight:    config.CameraOrthoHeight,
		NearClip:       config.CameraNear,
		FarClip:        config.CameraFar,
		ViewportWidth:  config.ResolutionWidth,
		ViewportHeight: config.ResolutionHeight,
	}

	return settings
}

func isometricCamera(angle float32) engine.Camera3DSettings {
	return centeredIsometricCamera(angle, 9, 2.5)
}

func centeredIsometricCamera(angle, centerX, centerY float32) engine.Camera3DSettings {
	const radius = 20
	x := centerX - float32(math.Sin(float64(angle)))*radius
	y := centerY - float32(math.Cos(float64(angle)))*radius

	return engine.Camera3DSettings{
		Projection:     engine.CameraProjectionIsometric,
		Output:         engine.CameraOutputFull,
		Position:       engine.NewVec3(x, y, 18),
		Yaw:            angle,
		Pitch:          -0.6154797,
		VerticalFOV:    config.CameraFOVY,
		OrthoHeight:    config.CameraOrthoHeight,
		NearClip:       config.CameraNear,
		FarClip:        config.CameraFar,
		ViewportWidth:  config.ResolutionWidth,
		ViewportHeight: config.ResolutionHeight,
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
