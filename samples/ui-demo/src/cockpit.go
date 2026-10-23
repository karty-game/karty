package main

import "example.com/ui-demo/.karty/engine"

type cockpit struct {
	view     *engine.UI
	quantity int
}

func (cockpit *cockpit) consumePotion() bool {
	if cockpit.quantity <= 0 {
		return false
	}
	cockpit.quantity--
	return true
}

func (cockpit *cockpit) invalidate() {
	if cockpit.view != nil {
		cockpit.view.Invalidate()
	}
}

func (cockpit *cockpit) close() {
	if cockpit.view != nil {
		cockpit.view.Close()
	}
}
