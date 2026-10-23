package main

import (
	"example.com/hangar/.karty/engine"
	"example.com/hangar/.karty/ui"
)

type explorerPanel struct {
	ui             *engine.UI
	status         string
	cameraMenuOpen bool
	keys           [256]bool
}

// The caller supplies game actions; this panel supplies its own display state.
func (panel *explorerPanel) show(props ui.ExplorerProps) {
	panel.cameraMenuOpen = false
	props.Status = func() string { return panel.status }
	props.Hide = panel.hide
	props.CameraOpen = func() bool { return panel.cameraMenuOpen }
	props.ToggleCamera = func() {
		panel.cameraMenuOpen = !panel.cameraMenuOpen
		panel.ui.Invalidate()
	}
	// LaunchLight is paused; the reusable demo lives in light_bolt.go.
	panel.ui = ui.Explorer(props)
}

func (panel *explorerPanel) hide() {
	panel.ui.Close()
	panel.ui = nil
}

func (panel *explorerPanel) refresh() {
	if panel.ui != nil {
		panel.ui.Invalidate()
	}
}

func (panel *explorerPanel) setStatus(status string) {
	panel.status = status
	panel.refresh()
}

// Fire shortcuts once per press, even when the host repeats key-down events.
func (panel *explorerPanel) pressed(key engine.Key, down bool) bool {
	if key != engine.KeyV && key != engine.KeyH && key != engine.KeyF {
		return false
	}
	held := panel.keys[key]
	panel.keys[key] = down
	return down && !held
}

func (panel *explorerPanel) resetKeys() { panel.keys = [256]bool{} }

// A held view/torch key must stay held when replacing the camera resets input.
func (panel *explorerPanel) preserveViewKeys() {
	held := panel.keys
	panel.resetKeys()
	for _, key := range [...]engine.Key{engine.KeyV, engine.KeyF} {
		panel.keys[key] = held[key]
	}
}
