package main

import (
	"example.com/hangar/.karty/config"
	"example.com/hangar/.karty/engine"
	fpscontroller "example.com/hangar/.karty/processed/code/controllers-3d.sdk/controller"
)

type hangarTour struct {
	camera       *engine.Camera3D
	controller   *fpscontroller.FirstPersonController
	torch        *engine.Light3D
	torchAim     lightAim
	torchEnabled bool
	bolt         lightBolt
	view         int
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

// Recreate the camera, body and attached torch so a landmark jump is a teleport.
func (tour *hangarTour) visit(host *engine.Game, level engine.LevelHandle, index int, options fpscontroller.FirstPersonOptions) string {
	tour.clearLights()
	if tour.camera != nil {
		tour.camera.Destroy()
	}
	view := viewpoints[index]
	tour.camera = host.NewCamera3D(level, engine.Camera3DSettings{
		Projection: engine.CameraProjectionPerspective, Output: engine.CameraOutputFull,
		Position: engine.NewVec3(view.x, view.y, view.z), Yaw: view.yaw, Pitch: view.pitch,
		VerticalFOV: config.CameraFOVY, NearClip: config.CameraNear, FarClip: config.CameraFar,
		OrthoHeight:   config.CameraOrthoHeight,
		ViewportWidth: config.ResolutionWidth, ViewportHeight: config.ResolutionHeight,
	})
	tour.camera.SetLayer(-10)
	var err error
	tour.controller, err = fpscontroller.AttachFirstPerson(*host, tour.camera, options)
	if err != nil {
		panic(err)
	}
	tour.createTorch(host, level, view)
	// The generated hook applies the host's accepted pose to camera.Settings.
	// Every walking proposal starts from that pose, including on the stairs.
	var status string
	if _, err := host.WatchPose(tour.camera.Ref(), tour.acceptTorchPose); err != nil {
		status = err.Error()
	} else {
		status = view.name
	}
	tour.view = index
	return status
}

func (tour *hangarTour) controls() string {
	if tour.controller != nil && tour.controller.TouchMode() {
		return "Left stick walk and push crates · Right stick look · Torch toggles beam"
	}
	return "WASD walk and push crates · Mouse look · Esc release · Shift run · F torch · V view · H hide"
}
