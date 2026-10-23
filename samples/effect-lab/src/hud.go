package main

import (
	"example.com/effect-lab/.karty/engine"
	"example.com/effect-lab/.karty/ui"
)

type effectHUD struct {
	panel  *engine.UI
	status string
}

func (hud *effectHUD) show(host *engine.Game) {
	hud.status = "Loading effect lab…"
	hud.panel = ui.Hud(ui.HudProps{Status: func() string { return hud.status }})
	hud.drawCrosshair(host)
}

func (hud *effectHUD) setStatus(value string) {
	hud.status = value
	if hud.panel != nil {
		hud.panel.Invalidate()
	}
}

func (hud *effectHUD) close() {
	if hud.panel != nil {
		hud.panel.Close()
		hud.panel = nil
	}
}

func (hud *effectHUD) drawCrosshair(host *engine.Game) {
	for _, offset := range [...]engine.Vec2{engine.NewVec2(-6, 0), engine.NewVec2(0, -6)} {
		shape := engine.Line(engine.NewVec2(12, 0))
		if offset.Y != 0 {
			shape = engine.Line(engine.NewVec2(0, 12))
		}
		cross := host.NewShape2D(shape, engine.NewVec2(640+offset.X, 360+offset.Y))
		cross.SetStroke(engine.RGBA(235, 244, 255, 180), 1)
		cross.SetLayer(22)
	}
}
