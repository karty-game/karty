package main

import "example.com/effect-lab/.karty/engine"

// key handles scene shortcuts once per press; held movement stays with the rig.
func (game *Game) key(key engine.Key, down bool) {
	if !game.rig.pressed(key, down) {
		return
	}
	switch key {
	case engine.KeyEscape:
		game.rig.resetKeys()
	case engine.KeyR:
		game.visit(game.rig.view)
	case engine.KeyDigit1:
		game.visit(0)
	case engine.KeyDigit2:
		game.visit(1)
	case engine.KeyDigit3:
		game.visit(2)
	}
}
