package main

import (
	"example.com/particle-lab/.karty/assets"
	"example.com/particle-lab/.karty/engine"
)

type effectMode uint8

const (
	collectionMode effectMode = iota
	fireMode
	smokeMode
)

// effectRoom owns the mounted timeline and the selected review station.
type effectRoom struct {
	level            engine.LevelHandle
	request, release engine.RequestID
	mode             effectMode
	view             int
}

func (room *effectRoom) levelName() engine.LevelName {
	if room.mode == fireMode {
		return assets.LevelBrackeysFire
	}
	if room.mode == smokeMode {
		return assets.LevelSmoke
	}
	return assets.LevelLab
}

// controlMode follows the UI tab order: Fire, Smoke, then Collection.
func (room *effectRoom) controlMode() int {
	if room.mode == smokeMode {
		return 1
	}
	if room.mode == collectionMode {
		return 2
	}
	return 0
}
