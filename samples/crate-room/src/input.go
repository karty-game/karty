package main

import "example.com/crate-room/.karty/engine"

// roomInput owns weapon actions; the controller package owns move/look feedback.
type roomInput struct {
	shootID                                 uint32
	mouseHeld, spaceHeld                    bool
	pendingShot, pendingBlast, pendingReset bool
}

func (input *roomInput) key(key engine.Key, down bool) {
	if key == engine.KeySpace {
		input.spaceHeld = down
	}
	if !down {
		return
	}
	switch key {
	case engine.KeySpace:
		input.pendingShot = true
	case engine.KeyB:
		input.pendingBlast = true
	case engine.KeyR:
		input.pendingReset = true
	case engine.KeyEscape:
		input.reset()
	}
}

// The centered shoot region reserves a third finger while the package handles move/look.
func (input *roomInput) pointer(host *engine.Game, pointer engine.PointerInput) {
	if pointer.Kind == engine.PointerMouse {
		if pointer.Phase == engine.PointerDown {
			if !shootContains(pointer.X, pointer.Y) && pointer.Buttons&1 == 0 {
				host.SetCursorCaptured(true)
			}
			if !shootContains(pointer.X, pointer.Y) && pointer.Buttons&2 != 0 {
				input.pendingBlast = true
			} else {
				input.mouseHeld, input.pendingShot = true, true
			}
		}
		if pointer.Phase == engine.PointerUp || pointer.Phase == engine.PointerCancel {
			input.mouseHeld = false
		}
		return
	}
	if pointer.Kind != engine.PointerTouch {
		return
	}
	if pointer.Phase == engine.PointerDown && shootContains(pointer.X, pointer.Y) {
		input.shootID, input.pendingShot = pointer.ID, true
	}
	if (pointer.Phase == engine.PointerUp || pointer.Phase == engine.PointerCancel) && pointer.ID == input.shootID {
		input.shootID = 0
	}
}

// Clear held fire without discarding an already requested room reset.
func (input *roomInput) reset() {
	input.shootID = 0
	input.mouseHeld, input.spaceHeld = false, false
	input.pendingShot, input.pendingBlast = false, false
}

func shootContains(x, y float32) bool {
	return x > shootX-125 && x < shootX+125 && y > shootY-57.5 && y < shootY+57.5
}
