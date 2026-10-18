package main

import (
	"strings"

	"example.com/last-train/.karty/engine"
	"example.com/last-train/internal/art"
	"example.com/last-train/internal/presentation"
	"github.com/karty-game/dialogue"
)

type pendingArt struct {
	packs    []string
	apply    func()
	reported bool
}

// waitForArt keeps the outgoing presentation and validated checkpoint intact.
// Ordinary reading never waits: lookahead starts at the title and carriage.
// This gate also covers instant skipping and cold late-checkpoint restoration.
func (g *game) waitForArt(packs []string, apply func()) {
	g.art.Request(packs...)
	if g.art.Ready(packs...) {
		apply()
		return
	}
	g.pendingArt = &pendingArt{packs: packs, apply: apply}
	g.model.Paused = true
	g.model.Status = "Preparing the next part of the journey…"
	g.invalidate()
}
func (g *game) retryArt() bool {
	if g.pendingArt == nil {
		return false
	}
	g.art.Retry(g.pendingArt.packs...)
	g.pendingArt.reported = false
	g.model.Paused = true
	g.model.Status = "Preparing the next part of the journey…"
	g.invalidate()
	return true
}
func (g *game) updateArt() bool {
	g.art.Step()
	// Investigation begins warming while the reader is still on the title.
	if g.art.Ready("opening") {
		g.art.Request("investigation")
	}
	p := g.pendingArt
	if p == nil {
		return false
	}
	if g.art.Ready(p.packs...) {
		g.pendingArt = nil
		g.model.Paused = false
		g.model.Status = ""
		p.apply()
		g.invalidate()
	} else if failure := g.art.Failure(p.packs...); failure != "" && !p.reported {
		p.reported = true
		// Enable Advance/Begin for an explicit retry; never discard the old view.
		g.model.Paused = false
		g.model.Status = "Could not prepare artwork. Advance or Begin to retry. " + failure
		g.invalidate()
	}
	return true
}
func (g *game) levelReady(e engine.LevelReady)   { g.art.Mounted(e); g.invalidate() }
func (g *game) levelFailed(e engine.LevelFailed) { g.art.Failed(e); g.invalidate() }

func (g *game) commandPacks(text string) []string {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return nil
	}
	switch fields[0] {
	case "travel", "scene":
		if fields[1] == "carriage" {
			g.art.Request("conclusion")
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
			if mood := g.visual.Expression(name); mood != "" {
				name += "-" + mood
			}
			return []string{art.Owner(name)}
		}
	}
	return nil
}

func checkpointPacks(view *dialogue.Model, c presentation.Composition) []string {
	packs := []string{"opening"}
	if view.Scene == "dining" {
		return []string{"opening", "investigation", "conclusion"}
	}
	if view.Scene == "carriage" || len(view.Choices) > 0 {
		packs = append(packs, "investigation")
	}
	expressions := map[string]string{"mara": c.Mara, "elias": c.Elias, "iris": c.Iris}
	for name, mood := range expressions {
		if !c.Ensemble && view.Portrait != name {
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
