package main

import (
	"strconv"

	"example.com/crate-room/.karty/engine"
	"example.com/crate-room/.karty/ui"
)

type roomHUD struct {
	panel  *engine.UI
	status string
}

func (hud *roomHUD) show(host *engine.Game, blast, reset func()) {
	hud.status = "Loading crate room..."
	hud.panel = ui.Hud(ui.HudProps{
		Status: func() string { return hud.status },
		Blast:  blast,
		Reset:  reset,
	})
	hud.drawWeaponControls(host)
}

func (hud *roomHUD) setStatus(value string) {
	hud.status = value
	if hud.panel != nil {
		hud.panel.Invalidate()
	}
}

func (hud *roomHUD) refresh(shots, blasts uint64) {
	hud.setStatus(
		"CRATE ROOM · 80 boxes · Shots " + strconv.FormatUint(shots, 10) + " · Blasts " + strconv.FormatUint(blasts, 10),
	)
}

func (hud *roomHUD) drawWeaponControls(host *engine.Game) {
	// SHOOT stays passive so raw down/up events preserve held fire; BLAST and RESET are UI buttons.
	drawButton(host, shootX-105, shootY-37.5, 210, 75)
	crossX := host.NewShape2D(engine.Line(engine.NewVec2(18, 0)), engine.NewVec2(width/2-9, height/2))
	crossX.SetStroke(engine.RGBA(255, 235, 184, 235), 2)
	crossX.SetLayer(35)
	crossY := host.NewShape2D(engine.Line(engine.NewVec2(0, 18)), engine.NewVec2(width/2, height/2-9))
	crossY.SetStroke(engine.RGBA(255, 235, 184, 235), 2)
	crossY.SetLayer(35)
}

func drawButton(host *engine.Game, x, y, w, h float32) {
	base := host.NewShape2D(engine.Rectangle(w, h), engine.NewVec2(x, y))
	base.SetFill(engine.RGBA(19, 32, 43, 210))
	base.SetStroke(engine.RGBA(220, 180, 109, 220), 2)
	base.SetLayer(30)
}
