package main

import (
	"math"
	"strconv"

	"example.com/world-camera/.karty/engine"
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
	label        *engine.EntityText2D
	path         float32
	fps          engine.CameraSettings
	orbit        float32
	left         bool
	right        bool
	orbitLeft    bool
	orbitRight   bool
	isometric    bool
	actorCount   uint8
}

type cameraPoint struct{ x, y, z, yaw float32 }

var cameraPath = [...]cameraPoint{
	{x: 1.5, y: 1, z: 1.72, yaw: 1.5707963},
	{x: 4.5, y: 1, z: 1.9, yaw: 1.5707963},
	{x: 8.5, y: 1, z: 1.9, yaw: 1.5707963},
	{x: 12.5, y: 1, z: 1.9, yaw: 1.5707963},
	{x: 17, y: 1, z: 1.99, yaw: 0},
	{x: 17, y: 3.5, z: 2.23, yaw: 0},
	{x: 17, y: 7, z: 2.79, yaw: 0},
}

//go:wasmexport karty_register
func main() { engine.Run(&Game{}) }

func (game *Game) Initialize() {
	game.label = game.NewText2D("Loading YAML world…", engine.NewVec2D(18, 18))
	game.label.SetSize(20)
	game.label.SetLayer(10)
	game.request = game.RequestLevel("levels.camera-showcase")
}

func (game *Game) Update(frame engine.Frame) {
	for _, event := range frame.Events {
		switch event.Type {
		case engine.EventLevelReady:
			if event.RequestID == uint32(game.request) && game.camera == nil {
				game.level = engine.LevelHandle(event.Handle)
				game.path = 0
				game.orbit = 0.9
				game.isometric = false
				game.fps = perspectiveCamera(game.path)
				game.camera = game.NewCamera3D(game.level, game.fps)
				game.camera.SetLayer(-10)
				game.orbitCamera = game.NewCamera3D(game.level, isometricCamera(game.orbit))
				game.orbitCamera.SetLayer(-10)
				game.orbitCamera.SetAlpha(0)
				game.tagQuery = game.QueryWorldTag(game.level, "interactive", 8)
				game.spinQuery = game.QueryWorldTag(game.level, "spinner", 1)
				game.markerQuery = game.QueryWorldTag(game.level, "player-marker", 1)
				game.label.SetText("S2 actors: discovering authored tags…")
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
				game.label.SetText("Remounting YAML world…")
			}
		case engine.EventLevelFailed:
			game.label.SetText("Level failed: " + event.Diagnostic)
		case engine.EventKeyDown, engine.EventKeyUp:
			down := event.Type == engine.EventKeyDown
			if event.Key == engine.KeyLeft {
				game.left = down
			}
			if event.Key == engine.KeyRight {
				game.right = down
			}
			if event.Key == engine.KeyA {
				game.orbitLeft = down
			}
			if event.Key == engine.KeyD {
				game.orbitRight = down
			}
		case engine.EventPointerUp:
			if event.Button == engine.PointerPrimary && game.camera != nil {
				game.isometric = !game.isometric
				if game.isometric {
					game.fps = game.camera.Settings
					game.camera.SetAlpha(0)
					game.orbitCamera.SetAlpha(1)
				} else {
					game.orbitCamera.SetAlpha(0)
					game.camera.SetAlpha(1)
				}
				game.showStatus()
			}
			if event.Button == engine.PointerSecondary && game.camera != nil && game.level != 0 {
				game.camera.Destroy()
				game.orbitCamera.Destroy()
				game.orbitCamera = nil
				game.camera = nil
				game.left = false
				game.right = false
				game.orbitLeft, game.orbitRight = false, false
				game.release = game.ReleaseLevel(game.level)
				game.label.SetText("Releasing YAML world…")
			}
		}
	}

	direction := float32(0)
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
		} else if game.path > float32(len(cameraPath)-1) {
			game.path = float32(len(cameraPath) - 1)
		}
		from, to := perspectiveCamera(previousPath), perspectiveCamera(game.path)
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

	orbitDirection := float32(0)
	if game.orbitLeft {
		orbitDirection--
	}
	if game.orbitRight {
		orbitDirection++
	}
	if orbitDirection != 0 && game.isometric && game.orbitCamera != nil {
		game.orbit += orbitDirection * 0.012
		if game.orbit < 0 {
			game.orbit += 2 * math.Pi
		} else if game.orbit >= 2*math.Pi {
			game.orbit -= 2 * math.Pi
		}
		game.orbitCamera.SetSettings(isometricCamera(game.orbit))
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
}

func (game *Game) Shutdown() {}

func (game *Game) showStatus() {
	view := "FPS · hold ←/→ to pan · click: isometric"
	if game.isometric {
		view = "Isometric · ←/→: move player · A/D: orbit · click: FPS"
	}
	game.label.SetText(view + " · actors: " + strconv.Itoa(int(game.actorCount)) + " · right-click: reload")
}

func perspectiveCamera(progress float32) engine.CameraSettings {
	segment := int(progress)
	if segment >= len(cameraPath)-1 {
		segment = len(cameraPath) - 2
		progress = float32(len(cameraPath) - 1)
	}
	amount := progress - float32(segment)
	from, to := cameraPath[segment], cameraPath[segment+1]
	settings := engine.CameraSettings{
		Projection: engine.CameraProjectionPerspective,
		Output:     engine.CameraOutputFull,
		Position: engine.NewVec3D(
			from.x+(to.x-from.x)*amount,
			from.y+(to.y-from.y)*amount,
			from.z+(to.z-from.z)*amount,
		),
		Yaw: from.yaw + (to.yaw-from.yaw)*amount, Pitch: -0.08, FOVY: 1.024, OrthoHeight: 14,
		Near: 0.05, Far: 80, ViewportWidth: 960, ViewportHeight: 540,
	}

	return settings
}

func isometricCamera(angle float32) engine.CameraSettings {
	const centerX, centerY, radius = 9, 2.5, 20
	x := centerX - float32(math.Sin(float64(angle)))*radius
	y := centerY - float32(math.Cos(float64(angle)))*radius

	return engine.CameraSettings{
		Projection: engine.CameraProjectionIsometric,
		Output:     engine.CameraOutputFull,
		Position:   engine.NewVec3D(x, y, 18),
		Yaw:        angle, Pitch: -0.6154797, FOVY: 1.024, OrthoHeight: 22,
		Near: 0.05, Far: 80, ViewportWidth: 960, ViewportHeight: 540,
	}
}
