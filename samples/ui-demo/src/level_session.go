package main

import (
	"example.com/ui-demo/.karty/assets"
	"example.com/ui-demo/.karty/engine"
)

type levelSession struct {
	name             engine.LevelName
	handle           engine.LevelHandle
	request, release engine.RequestID
}

func (level *levelSession) mount(game *engine.Game, name engine.LevelName, onReady func(engine.LevelReady)) bool {
	if level.request != 0 || level.handle != 0 {
		return false
	}
	level.name = name
	level.request = game.LoadLevel(name, onReady)
	return true
}

func (level *levelSession) ready(event engine.LevelReady) {
	level.request = 0
	level.handle = event.Handle
}

// The release must match both its operation and the mounted content.
func (level *levelSession) released(event engine.LevelReleased) bool {
	if event.RequestID != level.release || level.release == 0 || event.Handle != level.handle {
		return false
	}
	level.handle, level.release = 0, 0
	return true
}

func (level *levelSession) destination() string {
	if level.name == assets.LevelSecond {
		return "Outer relay"
	}
	return "Orbital station"
}
