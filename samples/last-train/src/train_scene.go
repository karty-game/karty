package main

import (
	"fmt"

	"example.com/last-train/.karty/engine"
	"example.com/last-train/internal/art"
	"example.com/last-train/internal/presentation"
	"example.com/last-train/internal/stage"
	"github.com/karty-game/dialogue"
)

type trainScene struct {
	transition  presentation.State
	motion      *dialogue.Stage
	visual      *stage.Renderer
	composition presentation.Composition
}

func (view *trainScene) initialize(game *engine.Game, artwork *art.Loader) {
	view.motion = dialogue.NewStage("mara", "elias", "iris")
	view.visual = stage.New(game, view.motion, artwork)
}

func (view *trainScene) setScene(model *dialogue.Model, scene string) error {
	switch scene {
	case "carriage":
		model.Location = "NORTHBOUND · THE PASSENGER CARRIAGE"
	case "dining":
		model.Location = "NORTHBOUND · THE LAST LIT LAMP"
	case "platform":
		model.Location = "NORTH QUAY · BEFORE THE FINAL STOP"
		if !view.transition.Busy {
			model.Location = "WESTBRIDGE · 23:47"
		}
	default:
		return fmt.Errorf("unknown scene %s", scene)
	}
	model.Scene = scene
	model.Portrait = ""
	view.composition.Frame, view.composition.Ensemble = "wide", false
	view.motion.SetScene(scene)
	_ = view.motion.SetPortrait("")
	_ = view.motion.SetFraming("wide")
	view.motion.SetEnsemble(false)
	view.motion.Settle()
	view.visual.Draw()
	return nil
}

// Settle immediately so a loaded checkpoint never shows the previous scene.
func (view *trainScene) restore(model *dialogue.Model, reducedMotion bool) {
	view.motion.SetScene(model.Scene)
	_ = view.motion.SetPortrait(model.Portrait)
	_ = view.motion.SetFraming(view.composition.Frame)
	view.motion.SetEnsemble(view.composition.Ensemble)
	view.motion.SetReducedMotion(reducedMotion)
	_ = view.visual.SetExpression("mara", view.composition.Mara)
	_ = view.visual.SetExpression("elias", view.composition.Elias)
	_ = view.visual.SetExpression("iris", view.composition.Iris)
	view.motion.Settle()
	view.visual.Draw()
}

func (view *trainScene) travel(scene string) error {
	var chapter, subtitle string
	switch scene {
	case "carriage":
		chapter, subtitle = "I · THE LAST NORTHBOUND", "A warm carriage. An unexpected request."
	case "dining":
		chapter, subtitle = "II · THE LAST LIT LAMP", "Twelve minutes to decide what matters."
	case "platform":
		chapter, subtitle = "NORTH QUAY", "Some promises outlive the journey."
	default:
		return fmt.Errorf("unknown destination %s", scene)
	}
	view.transition.Travel(scene, chapter, subtitle)
	return nil
}

func (view *trainScene) setPortrait(model *dialogue.Model, character string) error {
	switch character {
	case "mara", "elias", "iris":
		model.Portrait = character
	case "none":
		model.Portrait = ""
	default:
		return fmt.Errorf("unknown portrait %s", character)
	}
	return view.motion.SetPortrait(model.Portrait)
}

func (view *trainScene) setFraming(frame string) error {
	if err := view.motion.SetFraming(frame); err != nil {
		return err
	}
	view.composition.Frame = frame
	return nil
}

func (view *trainScene) setEnsemble(enabled bool) {
	view.composition.Ensemble = enabled
	view.motion.SetEnsemble(enabled)
}

func (view *trainScene) setExpression(character, mood string) error {
	if err := view.composition.SetExpression(character, mood); err != nil {
		return err
	}
	return view.visual.SetExpression(character, mood)
}
