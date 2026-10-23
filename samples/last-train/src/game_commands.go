package main

import (
	"fmt"
	"strconv"
	"strings"

	"example.com/last-train/.karty/assets"
)

func (g *game) command(text string) error {
	model := g.dialogue.model
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return fmt.Errorf("empty story command")
	}
	switch parts[0] {
	case "scene":
		if len(parts) != 2 {
			return fmt.Errorf("scene command needs a location")
		}
		return g.setScene(parts[1])
	case "travel":
		if len(parts) != 2 {
			return fmt.Errorf("travel command needs a location")
		}
		return g.scene.travel(parts[1])
	case "pause":
		if len(parts) != 2 {
			return fmt.Errorf("pause needs a duration in updates")
		}
		ticks, err := strconv.Atoi(parts[1])
		if err != nil || ticks < 1 || ticks > 120 {
			return fmt.Errorf("invalid pause duration")
		}
		g.scene.transition.Pause(ticks)
	case "portrait":
		if len(parts) != 2 {
			return fmt.Errorf("portrait command needs a character")
		}
		return g.scene.setPortrait(model, parts[1])
	case "frame":
		if len(parts) != 2 {
			return fmt.Errorf("frame needs a composition")
		}
		return g.scene.setFraming(parts[1])
	case "ensemble":
		if len(parts) != 2 || parts[1] != "on" && parts[1] != "off" {
			return fmt.Errorf("ensemble needs on or off")
		}
		g.scene.setEnsemble(parts[1] == "on")
	case "expression":
		if len(parts) != 3 {
			return fmt.Errorf("expression needs a character and mood")
		}
		return g.scene.setExpression(parts[1], parts[2])
	case "gesture":
		if len(parts) != 3 {
			return fmt.Errorf("gesture needs a character and reaction")
		}
		return g.scene.motion.Gesture(parts[1], parts[2])
	case "ending":
		if len(parts) != 2 {
			return fmt.Errorf("ending command needs an ID")
		}
		model.Ending = parts[1]
		if g.progress.profile.Unlock(parts[1]) {
			g.syncProfile()
			if err := g.progress.persist(); err != nil {
				model.Status = "Could not save profile: " + err.Error()
			}
		}
	case "punch":
		if model.Ambience {
			g.engine.PlaySound(assets.SoundTicketPunch)
		}
	case "whistle":
		if model.Ambience {
			g.engine.PlaySound(assets.SoundStationWhistle)
		}
	default:
		return fmt.Errorf("unhandled story command %q", text)
	}
	return nil
}

func (g *game) setScene(scene string) error {
	if err := g.scene.setScene(g.dialogue.model, scene); err != nil {
		return err
	}
	g.sound.apply(g.dialogue.model)
	return nil
}
