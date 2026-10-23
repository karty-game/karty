package main

import "example.com/last-train/.karty/ui"

func (g *game) showScreen() {
	g.screen = ui.GameScreen(ui.TrainScreenProps{
		Model:    g.dialogue.model,
		Controls: &g.progress.controls,
		Stage:    &g.scene.transition,
		Visual:   g.scene.visual,
		Motion:   g.toggleMotion,
		Begin:    g.begin,
		Slot:     g.nextSlot,
		Advance:  g.advance,
		Choose:   g.choose,
		Save:     g.save,
		Load:     g.load,
		Log:      g.log,
		Music:    g.music,
		Ambience: g.ambience,
		Restart:  g.restart,
	})
}

func (g *game) log() {
	if g.scene.transition.Title || g.scene.transition.Busy {
		return
	}
	g.dialogue.model.ShowHistory = !g.dialogue.model.ShowHistory
	g.invalidate()
}

func (g *game) invalidate() {
	if g.screen != nil {
		g.screen.Invalidate()
	}
}

func (g *game) fail(err error) {
	g.dialogue.model.Status = err.Error()
	g.invalidate()
}
