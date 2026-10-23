package main

import (
	"example.com/vigil/.karty/engine"
	"example.com/vigil/internal/dungeon"
)

type vigilHUD struct {
	view                 *engine.UI
	help, helpPinned     bool
	spellReady           bool
	status, area, prompt string
}

func (h *vigilHUD) update(frame uint64, ready bool) {
	if ready != h.spellReady {
		h.spellReady = ready
		h.refresh()
	}
	if h.help && !h.helpPinned && frame >= 600 {
		h.help = false
		h.refresh()
	}
}

func (h *vigilHUD) location(p dungeon.Point, nearShrine bool) {
	area := "The Narthex"
	switch {
	case p.Y > 18:
		area = "Chapel of the Last Light"
	case p.X < 6:
		area = "Transept of Memory"
	case p.X > 18:
		area = "The Brothers' Rest"
	case p.Y > 8:
		area = "The Crossing"
	case p.Y > 4.5:
		area = "Nave of Ash"
	}
	prompt := ""
	if nearShrine {
		prompt = "E / SPACE  ·  Rekindle vigil light"
	}
	if area != h.area || prompt != h.prompt {
		h.area, h.prompt = area, prompt
		h.refresh()
	}
}

func (h *vigilHUD) toggleHelp() {
	h.helpPinned = true
	h.help = !h.help
	h.refresh()
}

func (h *vigilHUD) refresh() {
	if h.view != nil {
		h.view.Invalidate()
	}
}

func (h *vigilHUD) setStatus(status string) {
	h.status = status
	h.refresh()
}
