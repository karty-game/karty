package controllers

// PointerPhase is independent of SDK bindings; the game's adapter maps phases.
type PointerPhase uint8

const (
	PointerDown PointerPhase = iota + 1
	PointerMove
	PointerUp
	PointerCancel
)

type Pointer struct {
	Phase                PointerPhase
	Touch                bool
	ID                   uint32
	X, Y, DeltaX, DeltaY float32
	Primary, Captured    bool
}

// PointerResult requests host capture and optional game-owned touch overlays.
type PointerResult struct{ Capture, Release, TouchStarted bool }

// Pointer handles the first-person preset's capture and dual-stick gestures.
// The game hit-tests its UI before passing scene=true on a new gesture; already
// owned fingers still receive move/up/cancel outside the scene. Touch begins in
// the bottom 55% of the logical viewport. Reset on focus loss and view changes.
func (c *FirstPerson) Pointer(p Pointer, scene bool) PointerResult {
	var result PointerResult
	if !p.Touch {
		switch p.Phase {
		case PointerDown:
			if p.Primary && scene && !c.captured {
				c.captured = true
				result.Capture = true
			}
		case PointerMove:
			if p.Captured {
				c.Mouse(p.DeltaX, p.DeltaY)
			}
		case PointerCancel:
			c.Reset()
			result.Release = true
		}
		return result
	}
	switch p.Phase {
	case PointerDown:
		if !scene || p.Y < c.options.Height*.45 {
			return result
		}
		side := 0
		if p.X >= c.options.Width*.5 {
			side = 1
		}
		if c.BeginTouch(side, p.ID, p.X, p.Y) && !c.touchMode {
			c.touchMode = true
			result.TouchStarted = true
		}
	case PointerMove:
		c.MoveTouch(p.ID, p.X, p.Y)
	case PointerUp, PointerCancel:
		c.EndTouch(p.ID)
	}
	return result
}

func (c *FirstPerson) TouchMode() bool { return c.touchMode }
func (c *FirstPerson) Reset()          { c.Input.Reset(); c.captured = false }
