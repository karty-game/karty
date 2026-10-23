package main

import (
	"example.com/particle-lab/.karty/engine"
	"example.com/particle-lab/.karty/ui"
)

type effectControls struct {
	panel  *engine.UI
	status string
	hidden bool
}

func (controls *effectControls) setStatus(status string) {
	controls.status = status
	if controls.panel != nil {
		controls.panel.Invalidate()
	}
}

func (controls *effectControls) setControlsHidden(hidden bool) {
	controls.hidden = hidden
	if controls.panel != nil {
		controls.panel.Invalidate()
	}
}

func (controls *effectControls) show(props ui.ControlsProps) {
	props.Status = func() string { return controls.status }
	props.Hidden = func() bool { return controls.hidden }
	props.Show = func() { controls.setControlsHidden(false) }
	props.Hide = func() { controls.setControlsHidden(true) }
	controls.panel = ui.Controls(props)
}
