package main

import (
	"strings"

	"example.com/last-train/.karty/engine"
	"example.com/last-train/internal/art"
	"example.com/last-train/internal/presentation"
	"example.com/last-train/internal/stage"
	"github.com/karty-game/dialogue"
)

type artwork struct {
	loader     *art.Loader
	pending    *pendingArt
	model      *dialogue.Model
	invalidate func()
}

type pendingArt struct {
	packs    []string
	apply    func()
	reported bool
}

func newArtwork(game *engine.Game, model *dialogue.Model, invalidate func()) artwork {
	return artwork{loader: art.New(game, invalidate), model: model, invalidate: invalidate}
}

// Keep the current scene intact until replacement artwork is ready.
func (a *artwork) wait(packs []string, apply func()) {
	a.loader.Request(packs...)
	if a.loader.Ready(packs...) {
		apply()
		return
	}
	a.pending = &pendingArt{packs: packs, apply: apply}
	a.model.Paused = true
	a.model.Status = "Preparing the next part of the journey…"
	a.invalidate()
}

func (a *artwork) retry() bool {
	if a.pending == nil {
		return false
	}
	a.loader.Retry(a.pending.packs...)
	a.pending.reported = false
	a.model.Paused = true
	a.model.Status = "Preparing the next part of the journey…"
	a.invalidate()
	return true
}

// A pending gate consumes this update even when its artwork becomes ready.
func (a *artwork) step() bool {
	a.loader.Step()
	// Investigation begins warming while the reader is still on the title.
	if a.loader.Ready("opening") {
		a.loader.Request("investigation")
	}
	p := a.pending
	if p == nil {
		return false
	}
	if a.loader.Ready(p.packs...) {
		a.pending = nil
		a.model.Paused = false
		a.model.Status = ""
		p.apply()
		a.invalidate()
	} else if failure := a.loader.Failure(p.packs...); failure != "" && !p.reported {
		p.reported = true
		// Enable Advance/Begin for an explicit retry; never discard the old view.
		a.model.Paused = false
		a.model.Status = "Could not prepare artwork. Advance or Begin to retry. " + failure
		a.invalidate()
	}
	return true
}

func (a *artwork) commandPacks(text string, visual *stage.Renderer) []string {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return nil
	}
	switch fields[0] {
	case "travel", "scene":
		if fields[1] == "carriage" {
			a.loader.Request("conclusion")
		}
		if fields[1] == "dining" {
			return []string{"opening", "investigation", "conclusion"}
		}
		return []string{art.Owner(fields[1])}
	case "expression":
		if len(fields) == 3 {
			name := fields[1]
			if fields[2] != "neutral" {
				name += "-" + fields[2]
			}
			return []string{art.Owner(name)}
		}
	case "portrait":
		if fields[1] != "none" {
			name := fields[1]
			if mood := visual.Expression(name); mood != "" {
				name += "-" + mood
			}
			return []string{art.Owner(name)}
		}
	}
	return nil
}

func checkpointPacks(view *dialogue.Model, composition presentation.Composition) []string {
	packs := []string{"opening"}
	if view.Scene == "dining" {
		return []string{"opening", "investigation", "conclusion"}
	}
	if view.Scene == "carriage" || len(view.Choices) > 0 {
		packs = append(packs, "investigation")
	}
	expressions := map[string]string{"mara": composition.Mara, "elias": composition.Elias, "iris": composition.Iris}
	for name, mood := range expressions {
		if !composition.Ensemble && view.Portrait != name {
			continue
		}
		if mood != "neutral" {
			name += "-" + mood
		}
		owner := art.Owner(name)
		found := false
		for _, p := range packs {
			found = found || p == owner
		}
		if !found {
			packs = append(packs, owner)
		}
	}
	return packs
}
