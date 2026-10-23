package main

import (
	"example.com/last-train/.karty/assets"
	"example.com/last-train/.karty/engine"
	"github.com/karty-game/dialogue"
)

type soundtrack struct {
	game   *engine.Game
	warmup uint16
}

func (sound *soundtrack) step(model *dialogue.Model) {
	if sound.warmup > 0 {
		sound.warmup--
		if sound.warmup == 0 {
			sound.apply(model)
		}
	}
}

func (sound *soundtrack) apply(model *dialogue.Model) {
	if sound.warmup > 0 {
		return
	}
	if model.Music {
		sound.game.PlayMusic(assets.MusicLastCarriage, engine.StreamOptions{Loop: true, CrossfadeMS: 400})
	} else {
		sound.game.StopMusic(250)
	}
	if model.Ambience && model.Scene != "platform" {
		sound.game.PlayAmbience(engine.AmbienceChannel0, assets.EnvironmentNightTrain, engine.StreamOptions{Loop: true, CrossfadeMS: 400})
	} else {
		sound.game.StopAmbience(engine.AmbienceChannel0, 250)
	}
}

func (sound *soundtrack) stop() {
	sound.game.StopMusic(0)
	sound.game.StopAmbience(engine.AmbienceChannel0, 0)
}
