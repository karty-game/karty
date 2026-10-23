package main

func (g *game) syncProfile() {
	g.progress.sync(g.dialogue.model)
	if g.scene.motion != nil {
		g.scene.motion.SetReducedMotion(g.progress.profile.ReducedMotion)
	}
}

func (g *game) toggleMotion() {
	g.progress.profile.ReducedMotion = !g.progress.profile.ReducedMotion
	g.syncProfile()
	g.scene.visual.Draw()
	if err := g.progress.persist(); err != nil {
		g.dialogue.model.Status = "Could not save motion preference: " + err.Error()
	}
	g.invalidate()
}

func (g *game) music() {
	g.progress.profile.Music = !g.progress.profile.Music
	g.syncProfile()
	g.sound.apply(g.dialogue.model)
	if g.dialogue.model.Music {
		g.dialogue.model.Status = "Music on"
	} else {
		g.dialogue.model.Status = "Music off"
	}
	if err := g.progress.persist(); err != nil {
		g.dialogue.model.Status += " · Could not save profile: " + err.Error()
	}
	g.invalidate()
}

func (g *game) ambience() {
	g.progress.profile.Ambience = !g.progress.profile.Ambience
	g.syncProfile()
	g.sound.apply(g.dialogue.model)
	if g.dialogue.model.Ambience {
		g.dialogue.model.Status = "Train ambience on"
	} else {
		g.dialogue.model.Status = "Train ambience off"
	}
	if err := g.progress.persist(); err != nil {
		g.dialogue.model.Status += " · Could not save profile: " + err.Error()
	}
	g.invalidate()
}
