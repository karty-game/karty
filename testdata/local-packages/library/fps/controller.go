// Package fps implements an opt-in, fixed-60-Hz first-person controller.
// It has no SDK imports, global hooks, scene loading, or entity ownership.
package fps

import (
	"errors"
	"math"
)

// Body is implemented by a game's small SDK adapter. Heading must read the
// host's accepted pose. Move sets horizontal metres/second, retaining gravity.
// The game owns the camera, collider, transform subscription and destruction.
type Body interface {
	Heading() (yaw, pitch float32)
	Look(yaw, pitch float32)
	Move(x, y float32)
}

// Intent can come from keyboard/touch input, a replay, AI, or game code.
// Axes are normalized; mouse values are relative pixels since the last update.
type Intent struct {
	Forward, Strafe, Turn, Look float32
	MouseX, MouseY              float32
	Run                         bool
}

type Options struct {
	Speed, RunMultiplier, LookSensitivity float32
	TurnSpeed, PitchSpeed                 float32
	MinPitch, MaxPitch                    float32
}

func DefaultOptions() Options {
	return Options{Speed: 3.3, RunMultiplier: 2, LookSensitivity: .0025,
		TurnSpeed: 1.5, PitchSpeed: 1.2, MinPitch: -1.35, MaxPitch: 1.35}
}

var ErrOptions = errors.New("invalid FPS controller options")

type Controller struct {
	body    Body
	options Options
	enabled bool
}

// New creates a detached controller. Attach and Update are explicit.
func New(options Options) (*Controller, error) {
	c := &Controller{enabled: true}
	if err := c.Configure(options); err != nil {
		return nil, err
	}
	return c, nil
}

// Configure is transactional. Explicit zero speed/sensitivity is preserved.
func (c *Controller) Configure(options Options) error {
	for _, value := range [...]float32{options.Speed, options.RunMultiplier, options.LookSensitivity,
		options.TurnSpeed, options.PitchSpeed, options.MinPitch, options.MaxPitch} {
		if !finite(value) {
			return ErrOptions
		}
	}
	if options.Speed < 0 || options.Speed > 100 || options.RunMultiplier < 1 || options.RunMultiplier > 10 ||
		options.LookSensitivity < 0 || options.LookSensitivity > 1 || options.TurnSpeed < 0 || options.TurnSpeed > 100 ||
		options.PitchSpeed < 0 || options.PitchSpeed > 100 || options.MinPitch > options.MaxPitch ||
		options.MinPitch <= -math.Pi/2 || options.MaxPitch >= math.Pi/2 {
		return ErrOptions
	}
	c.options = options
	return nil
}

func (c *Controller) Options() Options { return c.options }

// Attach stops the previous body without destroying it. Passing nil detaches.
func (c *Controller) Attach(body Body) { c.Reset(); c.body = body }

// Reset stops movement. Input reset/cursor release are owned by the caller.
func (c *Controller) Reset() {
	if c.body != nil {
		c.body.Move(0, 0)
	}
}

func (c *Controller) SetEnabled(enabled bool) {
	c.enabled = enabled
	if !enabled {
		c.Reset()
	}
}

// Update is called once per ordinary 60-Hz host update, never per draw.
func (c *Controller) Update(intent Intent) {
	if c.body == nil || !c.enabled {
		return
	}
	for _, value := range [...]float32{intent.Forward, intent.Strafe, intent.Turn, intent.Look, intent.MouseX, intent.MouseY} {
		if !finite(value) {
			c.Reset()
			return
		}
	}
	yaw, pitch := c.body.Heading()
	if !finite(yaw) || !finite(pitch) {
		c.Reset()
		return
	}
	o := c.options
	nextYaw := float32(math.Remainder(float64(yaw+axis(intent.Turn)*o.TurnSpeed/60+intent.MouseX*o.LookSensitivity), 2*math.Pi))
	nextPitch := max(o.MinPitch, min(o.MaxPitch, pitch+axis(intent.Look)*o.PitchSpeed/60-intent.MouseY*o.LookSensitivity))
	if nextYaw != yaw || nextPitch != pitch {
		c.body.Look(nextYaw, nextPitch)
	}
	forward, strafe := axis(intent.Forward), axis(intent.Strafe)
	if length := float32(math.Hypot(float64(forward), float64(strafe))); length > 1 {
		forward, strafe = forward/length, strafe/length
	}
	speed := o.Speed
	if intent.Run {
		speed *= o.RunMultiplier
	}
	sine, cosine := float32(math.Sin(float64(nextYaw))), float32(math.Cos(float64(nextYaw)))
	// Sending zero on release is required to stop the host-owned physics body.
	c.body.Move((forward*sine+strafe*cosine)*speed, (forward*cosine-strafe*sine)*speed)
}

func axis(value float32) float32 { return max(-1, min(1, value)) }
func finite(value float32) bool  { return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0) }
