package main

import (
	"example.com/last-train/.karty/engine"
	"example.com/last-train/internal/persistence"
	"example.com/last-train/internal/presentation"
	"github.com/karty-game/dialogue"
)

func (g *game) save() {
	if g.artwork.pending != nil || g.scene.transition.Title || g.scene.transition.Busy || g.dialogue.story == nil {
		return
	}
	state, err := g.dialogue.story.Save()
	if err != nil {
		g.fail(err)
		return
	}
	if err := g.progress.writeCheckpoint(state, g.dialogue.model, g.scene.composition); err != nil {
		g.dialogue.model.Status = "Could not save: " + err.Error()
	} else {
		g.dialogue.model.Status = "Checkpoint saved · Slot " + g.progress.controls.Slot + " · You can close and reopen the game."
	}
	g.invalidate()
}

// Validate the whole checkpoint before replacing the current journey.
func (g *game) load() {
	if g.scene.transition.Busy || g.dialogue.program == nil {
		return
	}
	data, err := g.progress.readRun(&g.engine)
	if err == engine.SaveNotFound {
		g.dialogue.model.Status = "No checkpoint in slot " + g.progress.controls.Slot + ". Choose Save to keep this moment."
		g.invalidate()
		return
	}
	if err != nil {
		g.dialogue.model.Status = "Could not load: " + err.Error()
		g.invalidate()
		return
	}
	checkpoint, err := persistence.ReadCheckpoint(data)
	if err != nil {
		g.dialogue.model.Status = "The checkpoint is incomplete or incompatible."
		g.invalidate()
		return
	}
	composition, err := presentation.ReadComposition(checkpoint.Direction)
	if err != nil {
		g.dialogue.model.Status = "The checkpoint direction is invalid."
		g.invalidate()
		return
	}
	view := dialogue.New()
	view.Music, view.Ambience = g.progress.profile.Music, g.progress.profile.Ambience
	if err := view.RestoreRun(checkpoint.View); err != nil {
		g.dialogue.model.Status = "The checkpoint presentation is invalid."
		g.invalidate()
		return
	}
	restored := dialogue.NewStory(g.dialogue.program)
	if err = restored.Restore(checkpoint.Story); err != nil {
		g.dialogue.model.Status = "Could not restore: " + err.Error()
		g.invalidate()
		return
	}
	if view.Scene == "carriage" {
		g.artwork.loader.Request("conclusion")
	}
	g.artwork.wait(checkpointPacks(view, composition), func() {
		g.restoreCheckpoint(restored, view, composition)
	})
}

// Install the validated run only after all required artwork is ready.
func (g *game) restoreCheckpoint(story *dialogue.Story, view *dialogue.Model, composition presentation.Composition) {
	g.scene.transition.Begin()
	g.dialogue.story = story
	*g.dialogue.model = *view
	g.scene.composition = composition
	g.scene.restore(g.dialogue.model, g.progress.profile.ReducedMotion)
	g.dialogue.model.ShowHistory = false
	g.dialogue.model.Status = "Checkpoint restored · Slot " + g.progress.controls.Slot + "."
	if g.progress.profile.Unlock(g.dialogue.model.Ending) {
		g.syncProfile()
		if err := g.progress.persist(); err != nil {
			g.dialogue.model.Status += " Profile not saved: " + err.Error()
		}
	}
	g.sound.apply(g.dialogue.model)
	g.invalidate()
}

func (g *game) nextSlot() {
	g.progress.nextSlot(g.dialogue.model)
	g.invalidate()
}
