package main

import (
	"example.com/effect-lab/.karty/assets"
	"example.com/effect-lab/.karty/engine"
)

type Game struct {
	engine.Game
	level   engine.LevelHandle
	request engine.RequestID
	rig     cameraRig
	hud     effectHUD
}

//go:wasmexport karty_register
func main() {
	game := &Game{}
	engine.Run(engine.Hooks{
		OnStart:      game.start,
		OnStop:       game.stop,
		OnKeyDown:    func(key engine.Key) { game.key(key, true) },
		OnKeyUp:      func(key engine.Key) { game.key(key, false) },
		OnInputReset: game.rig.resetKeys,
		OnLevelFailed: func(event engine.LevelFailed) {
			if event.RequestID == game.request {
				game.request = 0
				game.hud.setStatus("Load failed: " + event.Diagnostic)
			}
		},
	})
}

func (game *Game) start() {
	game.hud.show(&game.Game)
	game.request = game.LoadLevel(assets.LevelEffectLab, game.ready)
}

func (game *Game) stop() {
	game.hud.close()
	if game.request != 0 {
		game.CancelLevelRequest(game.request)
	}
	if game.rig.entity != nil {
		game.rig.entity.Destroy()
	}
	if game.level != 0 {
		game.ReleaseLevel(game.level)
	}
}

// ready installs this request's room before selecting its first viewpoint.
func (game *Game) ready(event engine.LevelReady) {
	game.request, game.level = 0, event.Handle
	game.rig.mount(&game.Game, event.Handle)
	game.visit(0)
}

// visit keeps the HUD label in step with the chosen camera viewpoint.
func (game *Game) visit(index int) {
	if game.rig.entity == nil {
		return
	}
	game.hud.setStatus(game.rig.visit(&game.Game, game.level, index))
}
