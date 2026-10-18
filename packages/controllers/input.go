// Package controllers provides first-person, top-down 2D and isometric presets.
// Games route input and apply the returned motion through their own SDK bindings.
package controllers

import "math"

type Control uint8

const (
	Forward Control = iota
	Backward
	Left
	Right
	LookLeft
	LookRight
	LookUp
	LookDown
	RunLeft
	RunRight
	SwimUp
	SwimDown
	ForwardAlt
	BackwardAlt
	LeftAlt
	RightAlt
	controlCount
)

// Stick owns one finger until release or reset. Coordinates are logical pixels.
type Stick struct {
	ID           uint32
	X, Y, DX, DY float32
}

// Input is embedded in each preset; it needs no separate package or install.
// Distinct controls for alternate keys preserve independently held keys.
type Input struct {
	keys             [controlCount]bool
	mouseX, mouseY   float32
	Sticks           [2]Stick
	radius, deadzone float32
}

func (in *Input) Key(control Control, down bool) {
	if control < controlCount {
		in.keys[control] = down
	}
}

func (in *Input) Mouse(dx, dy float32) {
	if finite(float64(dx)) && finite(float64(dy)) && finite(float64(in.mouseX+dx)) && finite(float64(in.mouseY+dy)) {
		in.mouseX += dx
		in.mouseY += dy
	}
}

func (in *Input) BeginTouch(side int, id uint32, x, y float32) bool {
	if side < 0 || side >= len(in.Sticks) || id == 0 || !finite(float64(x)) || !finite(float64(y)) || in.Sticks[side].ID != 0 {
		return false
	}
	for _, s := range in.Sticks {
		if s.ID == id {
			return false
		}
	}
	in.Sticks[side] = Stick{ID: id, X: x, Y: y}
	return true
}

func (in *Input) MoveTouch(id uint32, x, y float32) {
	if id == 0 || !finite(float64(x)) || !finite(float64(y)) {
		return
	}
	for i := range in.Sticks {
		s := &in.Sticks[i]
		if s.ID != id {
			continue
		}
		dx, dy := float64(x)-float64(s.X), float64(y)-float64(s.Y)
		if length := math.Hypot(dx, dy); length > float64(in.radius) {
			dx *= float64(in.radius) / length
			dy *= float64(in.radius) / length
		}
		s.DX, s.DY = float32(dx), float32(dy)
	}
}

func (in *Input) EndTouch(id uint32) {
	if id == 0 {
		return
	}
	for i := range in.Sticks {
		if in.Sticks[i].ID == id {
			in.Sticks[i].ID = 0
			in.Sticks[i].DX = 0
			in.Sticks[i].DY = 0
		}
	}
}

func (in *Input) Reset() {
	clear(in.keys[:])
	in.mouseX, in.mouseY = 0, 0
	for i := range in.Sticks {
		in.Sticks[i].ID = 0
		in.Sticks[i].DX = 0
		in.Sticks[i].DY = 0
	}
}

func (in *Input) Running() bool { return in.keys[RunLeft] || in.keys[RunRight] }
func (in *Input) KeyboardMoving() bool {
	x, y := in.keyboardAxes()
	return x != 0 || y != 0
}

func (in *Input) axis(positive, negative Control) float32 {
	var v float32
	if in.keys[positive] {
		v++
	}
	if in.keys[negative] {
		v--
	}
	return v
}

func (in *Input) keyboardAxes() (x, y float32) {
	if in.keys[Right] || in.keys[RightAlt] {
		x++
	}
	if in.keys[Left] || in.keys[LeftAlt] {
		x--
	}
	if in.keys[Forward] || in.keys[ForwardAlt] {
		y--
	}
	if in.keys[Backward] || in.keys[BackwardAlt] {
		y++
	}
	return
}

func (in *Input) analog(side int) (float32, float32) {
	s := in.Sticks[side]
	if s.ID == 0 || in.radius <= 0 {
		return 0, 0
	}
	length := float32(math.Hypot(float64(s.DX), float64(s.DY))) / in.radius
	if length <= in.deadzone {
		return 0, 0
	}
	scale := min(1, (length-in.deadzone)/(1-in.deadzone)) / (length * in.radius)
	return s.DX * scale, s.DY * scale
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
