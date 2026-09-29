package main

import (
	"strconv"

	"example.com/world-camera/.karty/engine"
)

type Game struct {
	engine.Game

	request    engine.LevelRequestID
	release    engine.LevelRequestID
	tagQuery   engine.LevelRequestID
	level      engine.LevelHandle
	camera     *engine.EntityCamera3D
	label      *engine.EntityText2D
	route      uint8
	actorCount uint8
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
				game.route = 0
				game.camera = game.NewCamera3D(game.level, perspectiveCamera(game.route))
				game.camera.SetLayer(-10)
				game.tagQuery = game.QueryWorldTag(game.level, "interactive", 8)
				game.label.SetText("S2 actors: discovering authored tags…")
			}
		case engine.EventWorldTagQueryResult:
			if event.RequestID == uint32(game.tagQuery) {
				game.actorCount = event.ActorCount
				for _, actorID := range event.ActorIDs[:event.ActorCount] {
					game.WorldActor(actorID).SetTags("interactive", "verified")
				}
				game.showStatus("Perspective / FPS")
			}
		case engine.EventLevelReleased:
			if event.RequestID == uint32(game.release) {
				game.level = 0
				game.release = 0
				game.tagQuery = 0
				game.actorCount = 0
				game.request = game.RequestLevel("levels.camera-showcase")
				game.label.SetText("Remounting YAML world…")
			}
		case engine.EventLevelFailed:
			game.label.SetText("Level failed: " + event.Diagnostic)
		case engine.EventKeyDown:
			if game.camera == nil {
				continue
			}
			if event.Key == engine.KeyLeft {
				game.camera.SetSettings(perspectiveCamera(game.route))
				game.showStatus("Perspective / FPS")
			}
			if event.Key == engine.KeyRight {
				game.camera.SetSettings(isometricCamera())
				game.showStatus("Orthographic / isometric")
			}
		case engine.EventPointerUp:
			if event.Button == engine.PointerSecondary && game.camera != nil {
				game.route = (game.route + 1) % 3
				game.camera.SetSettings(perspectiveCamera(game.route))
				game.showStatus("Perspective seam route")
			}
			if event.Button == engine.PointerPrimary && game.camera != nil && game.level != 0 {
				game.camera.Destroy()
				game.camera = nil
				game.release = game.ReleaseLevel(game.level)
				game.label.SetText("Releasing YAML world…")
			}
		}
	}
}

func (game *Game) Shutdown() {}

func (game *Game) showStatus(view string) {
	game.label.SetText(view + " · authored actors: " + strconv.Itoa(int(game.actorCount)) +
		" · modes: camera/upright/cross/fixed")
}

func perspectiveCamera(route uint8) engine.CameraSettings {
	settings := engine.CameraSettings{
		Projection: engine.CameraProjectionPerspective,
		Output:     engine.CameraOutputFull,
		Position:   engine.NewVec3D(1.5, 1, 1.685),
		Yaw:        1.5707963, Pitch: -0.08, FOVY: 1.024, OrthoHeight: 14,
		Near: 0.05, Far: 80, ViewportWidth: 960, ViewportHeight: 540,
	}
	if route == 1 {
		settings.Position = engine.NewVec3D(4.5, 1, 1.775)
	}
	if route == 2 {
		settings.Position = engine.NewVec3D(8.5, 1, 1.895)
	}

	return settings
}

func isometricCamera() engine.CameraSettings {
	return engine.CameraSettings{
		Projection: engine.CameraProjectionIsometric,
		Output:     engine.CameraOutputFull,
		Position:   engine.NewVec3D(-8, -6, 13),
		Yaw:        0.7853982, Pitch: -0.6154797, FOVY: 1.024, OrthoHeight: 15,
		Near: 0.05, Far: 80, ViewportWidth: 960, ViewportHeight: 540,
	}
}
