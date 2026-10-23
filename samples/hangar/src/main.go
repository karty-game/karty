package main

import (
	"example.com/hangar/.karty/assets"
	"example.com/hangar/.karty/config"
	"example.com/hangar/.karty/engine"
	fpscontroller "example.com/hangar/.karty/processed/code/controllers-3d.sdk/controller"
	"example.com/hangar/.karty/ui"
)

type Game struct {
	engine.Game
	scene hangarScene
	tour  hangarTour
	panel explorerPanel
	audio hangarAudio
}

//go:wasmexport karty_register
func main() {
	game := &Game{}
	engine.Run(game.Hooks())
}

func (game *Game) Hooks() engine.Hooks {
	return engine.Hooks{
		OnStart:       game.start,
		OnUpdate:      game.update,
		OnStop:        game.stop,
		OnKeyDown:     func(key engine.Key) { game.key(key, true) },
		OnKeyUp:       func(key engine.Key) { game.key(key, false) },
		OnInputReset:  game.panel.resetKeys,
		OnLevelFailed: func(event engine.LevelFailed) { game.panel.setStatus("Level failed: " + event.Diagnostic) },
	}
}

func (game *Game) start() {
	game.panel.status = "Preparing the hangar..."
	game.tour.torchEnabled = true
	game.showPanel()
	game.LoadLevel(assets.LevelHangar, game.ready)
}

func (game *Game) update(frame engine.Frame) {
	game.scene.updateElevator(&game.Game)
	game.tour.updateTorch(frame.Number)
	if game.audio.pending {
		game.audio.create(&game.Game, game.scene.level, &game.scene.crates)
		game.audio.pending = false
	}
	game.audio.update(&game.Game, game.tour.camera, frame.Number)
	// game.tour.bolt.update() // Point-light demo paused.
}

func (game *Game) stop() {
	game.audio.stop(&game.Game)
	game.tour.clearLights()
	game.scene.clearCrates()
	if game.tour.camera != nil {
		game.tour.camera.Destroy()
	}
	if game.panel.ui != nil {
		game.panel.hide()
	}
}

func (game *Game) ready(event engine.LevelReady) {
	game.scene.ready(&game.Game, event)
	game.visit(0)
	// Start ambience after this level-ready callback, during the next update.
	game.audio.pending = true
}

// Replace the player camera without treating held V/F keys as new presses.
func (game *Game) visit(index int) {
	if game.scene.level == 0 || index < 0 || index >= len(viewpoints) {
		return
	}
	game.panel.preserveViewKeys()
	options := fpscontroller.DefaultFirstPersonOptions()
	options.Width, options.Height = float32(config.ResolutionWidth), float32(config.ResolutionHeight)
	options.ArrowKeysLook, options.TurnKeys = true, false
	options.Touch.StickRadius, options.Touch.InsetY = 72, min(200, options.Height)
	options.Touch.BaseRadius, options.Touch.ThumbRadius, options.Touch.AlignYOnFirstTouch = 72, 23, true
	options.Body.Friction = .5
	options.OnTouchMode = game.panel.refresh
	game.panel.status = game.tour.visit(&game.Game, game.scene.level, index, options)
	game.panel.cameraMenuOpen = false
	game.panel.refresh()
}

func (game *Game) showPanel() {
	game.panel.show(ui.ExplorerProps{
		Controls:     game.tour.controls,
		Visit:        game.visit,
		TorchEnabled: func() bool { return game.tour.torchEnabled },
		ToggleTorch:  game.toggleTorch,
	})
}

func (game *Game) toggleTorch() {
	game.tour.toggleTorch()
	game.panel.refresh()
}

func (game *Game) key(key engine.Key, down bool) {
	if !game.panel.pressed(key, down) {
		return
	}
	switch key {
	case engine.KeyV:
		game.visit((game.tour.view + 1) % len(viewpoints))
	case engine.KeyF:
		game.toggleTorch()
	case engine.KeyH:
		if game.panel.ui != nil {
			game.panel.hide()
		} else {
			game.showPanel()
		}
	}
}
