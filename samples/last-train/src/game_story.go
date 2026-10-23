package main

import (
	"fmt"

	"example.com/last-train/internal/presentation"
	"github.com/karty-game/dialogue"
)

// Return to the title without changing saved runs or lifetime preferences.
func (g *game) restart() {
	if g.dialogue.program == nil {
		return
	}
	g.artwork.pending = nil
	g.artwork.loader.CancelConclusion()
	// UI and artwork retain this model, so replace its contents in place.
	*g.dialogue.model = *dialogue.New()
	g.syncProfile()
	if err := g.dialogue.start(); err != nil {
		g.fail(err)
		return
	}
	g.scene.composition = presentation.DefaultComposition()
	g.scene.restore(g.dialogue.model, g.progress.profile.ReducedMotion)
	g.scene.transition.Opening()
	g.sound.apply(g.dialogue.model)
	g.invalidate()
}

func (g *game) begin() {
	if !g.scene.transition.Title {
		return
	}
	if g.artwork.retry() {
		return
	}
	g.artwork.wait([]string{"opening"}, func() {
		g.scene.transition.Begin()
		g.dialogue.model.Status = ""
		g.advance()
	})
}

// One press finishes the current reveal; the next reads to a line or decision.
func (g *game) advance() {
	if g.artwork.retry() {
		return
	}
	model := g.dialogue.model
	if g.scene.transition.Title || g.scene.transition.Busy || g.dialogue.story == nil || model.Done || model.ShowHistory ||
		len(model.Choices) > 0 {
		return
	}
	if model.Revealing {
		model.FinishReveal()
		g.invalidate()
		return
	}
	for count := 0; count < 32; count++ {
		event, err := g.dialogue.story.Continue()
		if err != nil {
			g.fail(err)
			return
		}
		switch event.Kind {
		case "line":
			if err := model.Line(event.Line.ID, g.dialogue.presentSpeaker(event.Line.Text)); err != nil {
				g.fail(err)
				return
			}
			model.Reveal()
			g.invalidate()
			return
		case "choices":
			options, available := dialogueOptions(event.Choices)
			if !available {
				// Yarn uses -1 when every response in a group is unavailable.
				if err := g.dialogue.story.Choose(-1); err != nil {
					g.fail(err)
					return
				}
				continue
			}
			g.presentChoices(options)
			return
		case "command":
			if g.waitForCommand(event.Command) {
				return
			}
			if err := g.command(event.Command); err != nil {
				g.fail(err)
				return
			}
			if g.scene.transition.Busy {
				model.Paused = true
				g.invalidate()
				return
			}
		case "complete":
			model.Done = true
			model.Status = "End of the line · Restart to try another journey."
			if err := g.progress.persist(); err != nil {
				model.Status += " Profile not saved: " + err.Error()
			}
			g.invalidate()
			return
		}
	}
	g.fail(fmt.Errorf("too many dialogue commands without a line"))
}

func (g *game) choose(id int) {
	model := g.dialogue.model
	if g.artwork.pending != nil || g.scene.transition.Title || g.scene.transition.Busy || g.dialogue.story == nil || model.ShowHistory {
		return
	}
	if err := g.dialogue.story.Choose(id); err != nil {
		g.fail(err)
		return
	}
	model.Choices = nil
	g.advance()
}

func (g *game) presentChoices(options []dialogue.Option) {
	g.artwork.wait([]string{"opening", "investigation"}, func() {
		if err := g.dialogue.model.SetChoices(options); err != nil {
			g.fail(err)
		}
		g.invalidate()
	})
}

// Resume the same command after loading; story execution stays paused meanwhile.
func (g *game) waitForCommand(command string) bool {
	packs := g.artwork.commandPacks(command, g.scene.visual)
	if g.artwork.loader.Ready(packs...) {
		return false
	}
	g.artwork.wait(packs, func() {
		if err := g.command(command); err != nil {
			g.fail(err)
			return
		}
		if !g.scene.transition.Busy {
			g.advance()
		}
	})
	return true
}
