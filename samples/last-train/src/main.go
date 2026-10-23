package main

import (
	"example.com/last-train/.karty/engine"
	"github.com/karty-game/dialogue"
)

// game coordinates the story, its presentation and persistent progress.
type game struct {
	engine   engine.Game
	dialogue dialogueRun
	scene    trainScene
	artwork  artwork
	progress playerProgress
	sound    soundtrack
	screen   *engine.UI
}

func (g *game) start() {
	// Let the opening's first draws prepare its images and fonts before audio
	// starts. Otherwise those initial uploads can starve WebAudio on slow hosts.
	g.sound = soundtrack{game: &g.engine, warmup: 120}
	g.dialogue.model = dialogue.New()
	g.progress.initialize(&g.engine)
	g.artwork = newArtwork(&g.engine, g.dialogue.model, g.invalidate)
	g.artwork.loader.Request("opening")
	g.scene.initialize(&g.engine, g.artwork.loader)
	g.progress.read()
	g.syncProfile()
	g.showScreen()
	if err := g.dialogue.loadProgram(); err != nil {
		g.fail(err)
		return
	}
	g.restart()
	if g.progress.err != nil {
		g.dialogue.model.Status = "Could not read profile: " + g.progress.err.Error()
		g.invalidate()
	}
}

func (g *game) update(_ engine.Frame) {
	model := g.dialogue.model
	if g.artwork.step() {
		return
	}
	g.sound.step(model)
	if model.ShowHistory {
		return
	}
	g.scene.visual.Step()
	if g.scene.transition.Title {
		return
	}
	if g.scene.transition.Busy {
		scene, resume := g.scene.transition.Step()
		if scene != "" {
			_ = g.setScene(scene)
		}
		model.Paused = g.scene.transition.Busy
		g.invalidate()
		if resume {
			g.advance()
		}
		return
	}
	if model.StepReveal() {
		g.invalidate()
	}
}

func (g *game) stop() {
	g.sound.stop()
	if g.screen != nil {
		g.screen.Close()
	}
	g.artwork.loader.Close()
}

func (g *game) levelFailed(event engine.LevelFailed) {
	g.artwork.loader.Failed(event)
	g.invalidate()
}

//go:wasmexport karty_register
func main() {
	g := &game{}
	engine.Run(engine.Hooks{
		OnStart:       g.start,
		OnUpdate:      g.update,
		OnLevelFailed: g.levelFailed,
		OnStop:        g.stop,
	})
}
