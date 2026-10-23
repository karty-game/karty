package main

import (
	"example.com/terrain/.karty/engine"
	"example.com/terrain/.karty/ui"
)

type terrainHUD struct {
	ui     *engine.UI
	status string
}

func (hud *terrainHUD) show() {
	hud.status = "Loading cove terrain..."
	hud.ui = ui.TerrainHud(ui.TerrainHUDProps{Status: func() string { return hud.status }})
}

func (hud *terrainHUD) setStatus(status string) {
	hud.status = status
	if hud.ui != nil {
		hud.ui.Invalidate()
	}
}

func (hud *terrainHUD) close() { hud.ui.Close() }
