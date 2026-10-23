package main

import (
	"example.com/particle-lab/.karty/engine"
)

// key handles review shortcuts once per press while the camera retains movement holds.
func (game *Game) key(key engine.Key, down bool) {
	if !game.camera.pressed(key, down) {
		return
	}

	switch {
	case key == engine.KeyV:
		game.visit((game.room.view + 1) % len(viewpoints))
	case key == engine.KeyM:
		game.visit(smokeComparisonView)
	case key == engine.KeyDigit0 && game.room.mode == smokeMode:
		game.visit(smokeComparisonView)
	case key == engine.KeyB || key == engine.KeyDigit0:
		game.visit(fireComparisonView)
	case game.room.mode == smokeMode && key >= engine.KeyDigit1 && key <= engine.KeyDigit2:
		game.visit(thinSmokeView + int(key-engine.KeyDigit1))
	case game.room.mode == fireMode && key >= engine.KeyDigit1 && key <= engine.KeyDigit3:
		game.selectFire(int(key - engine.KeyDigit1))
	case key >= engine.KeyDigit1 && key <= engine.KeyDigit9:
		game.visit(int(key - engine.KeyDigit1))
	case key == engine.KeyF:
		if game.room.mode == fireMode {
			game.selectView(collectionView)
		} else {
			game.selectView(combinedFireView)
		}
	case key == engine.KeyR:
		game.restart()
	case key == engine.KeyEscape:
		game.camera.releasePointer(&game.Game)
	case key == engine.KeyH:
		if game.controls.panel == nil {
			game.showControls()
		} else {
			game.controls.panel.Close()
			game.controls.panel = nil
		}
	}
}
