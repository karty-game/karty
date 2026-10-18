package controllers

import (
	"errors"
	"math"
)

var ErrOptions = errors.New("invalid controller options")

// Speeds are metres/second or radians/second. Mouse sensitivity is radians/pixel.
// Defaults match the samples' fixed 60 Hz update rate, not their draw rate.
type FirstPersonOptions struct {
	Width, Height, StickRadius, StickInsetX, StickInsetY, Deadzone float32
	Speed, RunMultiplier, MouseSensitivity                         float32
	KeyYawSpeed, KeyPitchSpeed, StickYawSpeed, StickPitchSpeed     float32
	MinPitch, MaxPitch                                             float32
}

func DefaultFirstPersonOptions() FirstPersonOptions {
	return FirstPersonOptions{Width: 1280, Height: 720, StickRadius: 96, StickInsetX: 140, StickInsetY: 140, Deadzone: .15,
		Speed: 3.3, RunMultiplier: 2, MouseSensitivity: .0025,
		KeyYawSpeed: 1.32, KeyPitchSpeed: .9, StickYawSpeed: 1.8, StickPitchSpeed: 1.44, MinPitch: -1.35, MaxPitch: 1.35}
}

type FirstPerson struct {
	Input
	options             FirstPersonOptions
	captured, touchMode bool
}

func NewFirstPerson(options FirstPersonOptions) (*FirstPerson, error) {
	c := &FirstPerson{}
	if err := c.Configure(options); err != nil {
		return nil, err
	}
	c.Sticks = [2]Stick{
		{X: options.StickInsetX, Y: options.Height - options.StickInsetY},
		{X: options.Width - options.StickInsetX, Y: options.Height - options.StickInsetY},
	}
	return c, nil
}

// Configure validates before changing state. Explicit zero speeds are retained.
// Configure releases input; it does not move live overlay entities or resize a camera.
func (c *FirstPerson) Configure(o FirstPersonOptions) error {
	for _, v := range [...]float32{o.Width, o.Height, o.StickRadius, o.StickInsetX, o.StickInsetY, o.Deadzone, o.Speed, o.RunMultiplier, o.MouseSensitivity, o.KeyYawSpeed, o.KeyPitchSpeed, o.StickYawSpeed, o.StickPitchSpeed, o.MinPitch, o.MaxPitch} {
		if !finite(float64(v)) {
			return ErrOptions
		}
	}
	if o.Width <= 0 || o.Height <= 0 || o.StickRadius <= 0 || o.StickInsetX < 0 || o.StickInsetX > o.Width/2 || o.StickInsetY < 0 ||
		o.StickInsetY > o.Height ||
		o.Deadzone < 0 ||
		o.Deadzone >= 1 ||
		o.Speed < 0 ||
		o.Speed > 100 ||
		o.RunMultiplier < 1 ||
		o.RunMultiplier > 10 ||
		o.MouseSensitivity < 0 ||
		o.MouseSensitivity > 1 ||
		o.KeyYawSpeed < 0 ||
		o.KeyYawSpeed > 100 ||
		o.KeyPitchSpeed < 0 ||
		o.KeyPitchSpeed > 100 ||
		o.StickYawSpeed < 0 ||
		o.StickYawSpeed > 100 ||
		o.StickPitchSpeed < 0 ||
		o.StickPitchSpeed > 100 ||
		o.MinPitch > o.MaxPitch ||
		o.MinPitch <= -math.Pi/2 ||
		o.MaxPitch >= math.Pi/2 {
		return ErrOptions
	}
	c.options = o
	c.radius, c.deadzone = o.StickRadius, o.Deadzone
	c.Reset()
	return nil
}

func (c *FirstPerson) Options() FirstPersonOptions { return c.options }

// Motion contains per-update displacement and camera heading. Apply X/Y * 60
// as horizontal physics velocity; Z is the optional vertical action axis.
// Always start Step from the host's accepted pose, including after collisions.
type Motion struct{ X, Y, Z, Yaw, Pitch float32 }

func (c *FirstPerson) Step(yaw, pitch float32) Motion {
	if !finite(float64(yaw)) || !finite(float64(pitch)) {
		c.Reset()
		return Motion{}
	}
	x, y := c.keyboardAxes()
	strafe, forward := c.analog(0)
	strafe += x
	forward = -forward - y
	if length := float32(math.Hypot(float64(forward), float64(strafe))); length > 1 {
		forward /= length
		strafe /= length
	}
	lookX, lookY := c.analog(1)
	o := c.options
	yaw += c.axis(LookRight, LookLeft)*(o.KeyYawSpeed/60) + lookX*(o.StickYawSpeed/60) + c.mouseX*o.MouseSensitivity
	pitch += c.axis(LookUp, LookDown)*(o.KeyPitchSpeed/60) - lookY*(o.StickPitchSpeed/60) - c.mouseY*o.MouseSensitivity
	c.mouseX, c.mouseY = 0, 0
	pitch = max(o.MinPitch, min(o.MaxPitch, pitch))
	speed := o.Speed / 60
	if c.Running() {
		speed *= o.RunMultiplier
	}
	sine, cosine := float32(math.Sin(float64(yaw))), float32(math.Cos(float64(yaw)))
	return Motion{
		X:     (forward*sine + strafe*cosine) * speed,
		Y:     (forward*cosine - strafe*sine) * speed,
		Z:     c.axis(SwimUp, SwimDown),
		Yaw:   yaw,
		Pitch: pitch,
	}
}
