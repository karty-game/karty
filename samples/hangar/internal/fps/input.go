// Package fps contains the Hangar controller's input state and walking math.
package fps

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
	controlCount
)

const StickRadius = float32(72)

// Stick owns one finger for its entire gesture. Other fingers cannot steal it.
type Stick struct {
	ID           uint32
	X, Y, DX, DY float32
}

type Input struct {
	keys           [controlCount]bool
	mouseX, mouseY float32
	Sticks         [2]Stick
}

func New(width, height float32) Input {
	return Input{Sticks: [2]Stick{{X: 140, Y: height - 200}, {X: width - 140, Y: height - 200}}}
}

func (input *Input) Key(control Control, down bool) { input.keys[control] = down }
func (input *Input) Mouse(dx, dy float32)           { input.mouseX += dx; input.mouseY += dy }

func (input *Input) BeginTouch(side int, id uint32, x, y float32) bool {
	if side < 0 || side >= len(input.Sticks) || id == 0 || input.Sticks[side].ID != 0 {
		return false
	}
	for _, stick := range input.Sticks {
		if stick.ID == id {
			return false
		}
	}
	input.Sticks[side] = Stick{ID: id, X: x, Y: y}
	return true
}

func (input *Input) MoveTouch(id uint32, x, y float32) {
	if id == 0 {
		return
	}
	for index := range input.Sticks {
		stick := &input.Sticks[index]
		if stick.ID != id {
			continue
		}
		stick.DX, stick.DY = x-stick.X, y-stick.Y
		if length := hypot(stick.DX, stick.DY); length > StickRadius {
			stick.DX *= StickRadius / length
			stick.DY *= StickRadius / length
		}
	}
}

func (input *Input) EndTouch(id uint32) {
	if id == 0 {
		return
	}
	for index := range input.Sticks {
		stick := &input.Sticks[index]
		if stick.ID == id {
			stick.ID, stick.DX, stick.DY = 0, 0, 0
		}
	}
}

func (input *Input) Reset() {
	clear(input.keys[:])
	input.mouseX, input.mouseY = 0, 0
	for index := range input.Sticks {
		input.Sticks[index].ID, input.Sticks[index].DX, input.Sticks[index].DY = 0, 0, 0
	}
}

type Motion struct{ X, Y, Yaw, Pitch float32 }

// Step starts from the host's accepted pose, so blocked movement never accumulates.
// Karty updates at a fixed 60 Hz; mouse displacement is consumed once per update.
func (input *Input) Step(yaw, pitch float32) Motion {
	strafe, forward := analog(input.Sticks[0])
	strafe += input.axis(Right, Left)
	forward = -forward + input.axis(Forward, Backward)
	if length := hypot(forward, strafe); length > 1 {
		forward /= length
		strafe /= length
	}
	lookX, lookY := analog(input.Sticks[1])
	yaw += input.axis(LookRight, LookLeft)*.022 + lookX*.03 + input.mouseX*.0025
	pitch += input.axis(LookUp, LookDown)*.015 - lookY*.024 - input.mouseY*.0025
	input.mouseX, input.mouseY = 0, 0
	pitch = max(-1.35, min(1.35, pitch))
	speed := float32(.055)
	if input.keys[RunLeft] || input.keys[RunRight] {
		speed *= 2
	}
	sine, cosine := float32(math.Sin(float64(yaw))), float32(math.Cos(float64(yaw)))
	return Motion{X: (forward*sine + strafe*cosine) * speed, Y: (forward*cosine - strafe*sine) * speed, Yaw: yaw, Pitch: pitch}
}

func (input *Input) axis(positive, negative Control) float32 {
	var value float32
	if input.keys[positive] {
		value++
	}
	if input.keys[negative] {
		value--
	}
	return value
}

func analog(stick Stick) (float32, float32) {
	if stick.ID == 0 {
		return 0, 0
	}
	length := hypot(stick.DX, stick.DY) / StickRadius
	const deadzone = float32(.15)
	if length <= deadzone {
		return 0, 0
	}
	scale := min(1, (length-deadzone)/(1-deadzone)) / (length * StickRadius)
	return stick.DX * scale, stick.DY * scale
}

func hypot(x, y float32) float32 { return float32(math.Hypot(float64(x), float64(y))) }
