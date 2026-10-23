package main

import "example.com/willowbrook/.karty/engine"

type villageHUD struct {
	view  *engine.UI
	title string
}

func (h *villageHUD) setTitle(title string) {
	h.title = title
	h.refresh()
}

func (h *villageHUD) refresh() {
	if h.view != nil {
		h.view.Invalidate()
	}
}
