// Package controls supplies optional input bookkeeping independently of cameras.
package controls

import (
	"math"

	"github.com/karty-game/stdlib-spike/fps"
)

type Action uint8

const (
	Forward Action = iota
	Backward
	Left
	Right
	TurnLeft
	TurnRight
	LookUp
	LookDown
	Run
	actionCount
)

type Stick struct {
	ID           uint32
	X, Y, DX, DY float32
}

// Input accepts any game's key type. Bindings are copied at construction;
// multiple keys for one action retain that action until all keys are released.
type Input[K comparable] struct {
	bindings       map[K]Action
	held           map[K]bool
	counts         [actionCount]int
	mouseX, mouseY float32
	sticks         [2]Stick
}

func New[K comparable](bindings map[K]Action) *Input[K] {
	input := &Input[K]{bindings: make(map[K]Action, len(bindings)), held: make(map[K]bool, len(bindings))}
	for key, action := range bindings {
		input.bindings[key] = action
	}
	return input
}

func (input *Input[K]) Key(key K, down bool) bool {
	action, ok := input.bindings[key]
	if !ok || action >= actionCount {
		return false
	}
	if input.held[key] == down {
		return true
	}
	input.held[key] = down
	if down {
		input.counts[action]++
	} else {
		input.counts[action]--
	}
	return true
}

func (input *Input[K]) Mouse(dx, dy float32) {
	if finite(dx) && finite(dy) && finite(input.mouseX+dx) && finite(input.mouseY+dy) {
		input.mouseX += dx
		input.mouseY += dy
	}
}

// BeginTouch claims one slot for the complete gesture. Hit testing and slot
// selection belong to the game, so a UI can intercept input before this call.
func (input *Input[K]) BeginTouch(slot int, id uint32, x, y float32) bool {
	if slot < 0 || slot >= len(input.sticks) || id == 0 || !finite(x) || !finite(y) || input.sticks[slot].ID != 0 {
		return false
	}
	for _, stick := range input.sticks {
		if stick.ID == id {
			return false
		}
	}
	input.sticks[slot] = Stick{ID: id, X: x, Y: y}
	return true
}

func (input *Input[K]) MoveTouch(id uint32, x, y float32) {
	if id == 0 || !finite(x) || !finite(y) {
		return
	}
	for i := range input.sticks {
		s := &input.sticks[i]
		if s.ID == id && finite(x-s.X) && finite(y-s.Y) {
			s.DX, s.DY = x-s.X, y-s.Y
		}
	}
}

func (input *Input[K]) EndTouch(id uint32) {
	if id == 0 {
		return
	}
	for i := range input.sticks {
		if input.sticks[i].ID == id {
			input.sticks[i] = Stick{}
		}
	}
}

func (input *Input[K]) Stick(slot int) Stick {
	if slot < 0 || slot >= len(input.sticks) {
		return Stick{}
	}
	return input.sticks[slot]
}

// Sample consumes mouse displacement once. A caller can modify or replace the
// returned intent before passing it to a controller. Radius/deadzone are explicit.
func (input *Input[K]) Sample(radius, deadzone float32) fps.Intent {
	x, y := analog(input.sticks[0], radius, deadzone)
	lookX, lookY := analog(input.sticks[1], radius, deadzone)
	intent := fps.Intent{
		Forward: input.axis(Forward, Backward) - y, Strafe: input.axis(Right, Left) + x,
		Turn: input.axis(TurnRight, TurnLeft) + lookX, Look: input.axis(LookUp, LookDown) - lookY,
		MouseX: input.mouseX, MouseY: input.mouseY, Run: input.counts[Run] != 0,
	}
	input.mouseX, input.mouseY = 0, 0
	return intent
}

func (input *Input[K]) Reset() {
	clear(input.held)
	clear(input.counts[:])
	input.mouseX, input.mouseY = 0, 0
	input.sticks = [2]Stick{}
}

func (input *Input[K]) axis(positive, negative Action) float32 {
	var value float32
	if input.counts[positive] != 0 {
		value++
	}
	if input.counts[negative] != 0 {
		value--
	}
	return value
}

func analog(stick Stick, radius, deadzone float32) (float32, float32) {
	if stick.ID == 0 || !finite(radius) || radius <= 0 || !finite(deadzone) || deadzone < 0 || deadzone >= 1 {
		return 0, 0
	}
	length := math.Hypot(float64(stick.DX), float64(stick.DY))
	magnitude := length / float64(radius)
	if magnitude <= float64(deadzone) {
		return 0, 0
	}
	scale := min(1, (magnitude-float64(deadzone))/(1-float64(deadzone))) / length
	return float32(float64(stick.DX) * scale), float32(float64(stick.DY) * scale)
}

func finite(value float32) bool { return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0) }
