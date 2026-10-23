package main

import (
	"example.com/particle-lab/.karty/engine"
	"example.com/particle-lab/.karty/ui"
)

// Game mounts the authored particles; it never sends a per-particle update.
type Game struct {
	engine.Game
	room     effectRoom
	camera   effectCamera
	controls effectControls
}

//go:wasmexport karty_register
func main() {
	game := &Game{}
	engine.Run(game.Hooks())
}

func (game *Game) Hooks() engine.Hooks {
	return engine.Hooks{
		OnStart:   game.start,
		OnUpdate:  game.update,
		OnStop:    game.stop,
		OnKeyDown: func(key engine.Key) { game.key(key, true) },
		OnKeyUp:   func(key engine.Key) { game.key(key, false) },
		OnPointer: func(pointer engine.PointerInput) {
			game.camera.pointer(&game.Game, pointer, game.room.view, game.room.release != 0)
		},
		OnInputReset: func() {
			game.camera.keys = [256]bool{}
			game.camera.releasePointer(&game.Game)
		},
		OnLevelFailed:   game.failed,
		OnLevelReleased: game.released,
	}
}

func (game *Game) start() {
	game.room.mode, game.room.view = fireMode, combinedFireView
	game.controls.status = "Loading effects"
	game.showControls()
	game.room.request = game.LoadLevel(game.room.levelName(), game.ready)
}

// ready activates the timeline owned by this load before restoring its station.
func (game *Game) ready(event engine.LevelReady) {
	game.room.request, game.room.level = 0, event.Handle
	game.visit(game.room.view)
}

func (game *Game) failed(event engine.LevelFailed) {
	if event.RequestID == game.room.request && game.room.request != 0 {
		game.room.request = 0
	} else if event.RequestID == game.room.release && game.room.release != 0 {
		game.room.release = 0
		game.visit(game.room.view)
	} else {
		return
	}
	game.controls.setStatus("Load failed · try Restart")
}

// released requests the replacement only after the matching old level is gone.
func (game *Game) released(event engine.LevelReleased) {
	if event.RequestID != game.room.release || game.room.release == 0 || event.Handle != game.room.level {
		return
	}
	game.room.level, game.room.release = 0, 0
	game.room.request = game.LoadLevel(game.room.levelName(), game.ready)
}

// restart remounts the room to rewind finite effects; release and load never overlap.
func (game *Game) restart() {
	if game.room.request != 0 || game.room.release != 0 {
		return
	}
	game.camera.releasePointer(&game.Game)
	if game.camera.entity != nil {
		game.camera.entity.Destroy()
		game.camera.entity = nil
	}
	game.controls.setStatus("Restarting effects")
	if game.room.level != 0 {
		game.room.release = game.ReleaseLevel(game.room.level)
	} else {
		game.room.request = game.LoadLevel(game.room.levelName(), game.ready)
	}
}

func (game *Game) stop() {
	game.camera.releasePointer(&game.Game)
	if game.room.request != 0 {
		game.CancelLevelRequest(game.room.request)
	}
	if game.camera.entity != nil {
		game.camera.entity.Destroy()
	}
	if game.room.level != 0 && game.room.release == 0 {
		game.ReleaseLevel(game.room.level)
	}
	if game.controls.panel != nil {
		game.controls.panel.Close()
	}
}

func (game *Game) showControls() {
	game.controls.show(ui.ControlsProps{
		Ready:       func() bool { return game.camera.entity != nil && game.room.request == 0 && game.room.release == 0 },
		Mode:        game.room.controlMode,
		SelectMode:  game.selectMode,
		SelectFire:  game.selectFire,
		SelectSmoke: game.selectSmoke,
		Visit:       game.visit,
		Rotate:      game.rotate,
		Zoom:        game.zoom,
		Center:      func() { game.visit(game.room.view) },
		Restart:     game.restart,
	})
}

func (game *Game) update(frame engine.Frame) {
	if game.room.release == 0 {
		game.camera.update(frame)
	}
}
