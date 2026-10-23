package main

import (
	"strconv"

	"example.com/last-train/.karty/engine"
	"example.com/last-train/internal/persistence"
	"example.com/last-train/internal/presentation"
	"github.com/karty-game/dialogue"
	"github.com/karty-game/savegame"
)

type playerProgress struct {
	store    savegame.Store
	profile  persistence.Profile
	err      error
	controls persistence.Controls
}

func (progress *playerProgress) initialize(game *engine.Game) {
	progress.store = savegame.Store{Backend: game}
	progress.profile = persistence.DefaultProfile()
	progress.controls.Slot = "1"
}

func (progress *playerProgress) read() {
	data, err := progress.store.ReadProfile()
	if err == engine.SaveNotFound {
		return
	}
	if err != nil {
		progress.err = err
		return
	}
	profile, err := persistence.ReadProfile(data)
	if err != nil {
		progress.err = err
		return
	}
	progress.profile = profile
}

// Run loads keep the current preferences and shared ending unlocks.
func (progress *playerProgress) sync(model *dialogue.Model) {
	model.Music, model.Ambience = progress.profile.Music, progress.profile.Ambience
	progress.controls.Endings = strconv.Itoa(progress.profile.Count())
	progress.controls.Motion = "Motion: Full"
	if progress.profile.ReducedMotion {
		progress.controls.Motion = "Motion: Reduced"
	}
}

func (progress *playerProgress) persist() error {
	// Preserve an unreadable profile for recovery instead of replacing unknown data.
	if progress.err != nil {
		return progress.err
	}
	return progress.store.WriteProfile(progress.profile.Save())
}

// Save the story, visible dialogue and scene direction as one checkpoint.
func (progress *playerProgress) writeCheckpoint(story []byte, model *dialogue.Model, composition presentation.Composition) error {
	view, err := model.SaveRun()
	if err != nil {
		return err
	}
	direction, err := composition.Save()
	if err != nil {
		return err
	}
	checkpoint := persistence.Checkpoint{Story: story, View: view, Direction: direction}
	data, err := checkpoint.Save()
	if err != nil {
		return err
	}
	return progress.store.WriteRun(progress.controls.Slot, data)
}

func (progress *playerProgress) readRun(game *engine.Game) ([]byte, error) {
	data, err := progress.store.ReadRun(progress.controls.Slot)
	// The original sample stored one checkpoint, now treated as slot 1.
	if err == engine.SaveNotFound && progress.controls.Slot == "1" {
		return game.ReadSave("checkpoint")
	}
	return data, err
}

func (progress *playerProgress) nextSlot(model *dialogue.Model) {
	switch progress.controls.Slot {
	case "1":
		progress.controls.Slot = "2"
	case "2":
		progress.controls.Slot = "3"
	default:
		progress.controls.Slot = "1"
	}
	model.Status = "Slot " + progress.controls.Slot + " selected · Save or Load this journey."
}
