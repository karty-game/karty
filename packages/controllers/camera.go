package controllers

import (
	"errors"
	"math"
)

// ErrCameraInput rejects a camera update without changing its tracking history.
var ErrCameraInput = errors.New("invalid camera input")

// CameraFramingOptions describes composition in screen-right/screen-down axes.
// Damping is the per-axis half-life in seconds; zero follows immediately.
// DeadZone and SoftZone are full sizes as fractions of the supplied view.
// Inside DeadZone no correction is needed. Between the zones movement is damped;
// outside SoftZone correction also keeps the target inside that outer rectangle.
// ScreenPosition is normalized, with (.5,.5) at the center.
type CameraFramingOptions struct {
	Damping, DeadZone, SoftZone, ScreenPosition Vec2
	LookaheadTime, LookaheadDamping             float64
	MaxLookahead                                float64
}

func DefaultCameraFramingOptions() CameraFramingOptions {
	return CameraFramingOptions{
		Damping: Vec2{.12, .18}, DeadZone: Vec2{.04, .06},
		SoftZone: Vec2{.24, .6}, ScreenPosition: Vec2{.5, .5},
		LookaheadTime: .15, LookaheadDamping: .1, MaxLookahead: 1,
	}
}

// CameraComposer tracks one shot without owning an entity or rendering output.
// Keep one per independently tracked shot. Configure and Reset clear velocity
// history; pass snap on spawn, teleport or a discontinuous portal transition.
type CameraComposer struct {
	options     CameraFramingOptions
	previous    Vec2
	velocity    Vec2
	initialized bool
}

func NewCameraComposer(options CameraFramingOptions) (*CameraComposer, error) {
	c := &CameraComposer{}
	if err := c.Configure(options); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *CameraComposer) Options() CameraFramingOptions { return c.options }

// Configure validates transactionally. Explicit zeros are never defaulted.
func (c *CameraComposer) Configure(o CameraFramingOptions) error {
	for _, axis := range [2][4]float64{
		{o.Damping.X, o.DeadZone.X, o.SoftZone.X, o.ScreenPosition.X},
		{o.Damping.Y, o.DeadZone.Y, o.SoftZone.Y, o.ScreenPosition.Y},
	} {
		for _, v := range axis {
			if !finite(v) {
				return ErrOptions
			}
		}
		if axis[0] < 0 || axis[0] > 60 || axis[1] < 0 || axis[2] < axis[1] || axis[2] > 1 ||
			axis[3]-axis[2]/2 < 0 || axis[3]+axis[2]/2 > 1 {
			return ErrOptions
		}
	}
	if !finite(o.LookaheadTime) || o.LookaheadTime < 0 || o.LookaheadTime > 10 ||
		!finite(o.LookaheadDamping) || o.LookaheadDamping < 0 || o.LookaheadDamping > 60 ||
		!finite(o.MaxLookahead) || o.MaxLookahead < 0 || o.MaxLookahead > 1000000 {
		return ErrOptions
	}
	c.options = o
	c.Reset()
	return nil
}

func (c *CameraComposer) Reset() {
	c.previous, c.velocity, c.initialized = Vec2{}, Vec2{}, false
}

// Compose returns a focus in camera-plane coordinates. viewSize is the visible
// extent in the same units; dt is elapsed simulation seconds in (0,1]. Target
// positions must be accepted movement, never an animated sprite or proposal.
// Lookahead is capped in these camera-plane units, independently of view size.
// snap places the target at ScreenPosition and clears all lookahead history.
func (c *CameraComposer) Compose(current, target, viewSize Vec2, dt float64, snap bool) (Vec2, error) {
	if !cameraVector(current) || !cameraVector(target) || !cameraVector(viewSize) ||
		viewSize.X <= 0 || viewSize.Y <= 0 || !finite(dt) || dt <= 0 || dt > 1 {
		return current, ErrCameraInput
	}
	o := c.options
	anchor := Vec2{target.X - (o.ScreenPosition.X-.5)*viewSize.X, target.Y - (o.ScreenPosition.Y-.5)*viewSize.Y}
	if !cameraVector(anchor) {
		return current, ErrCameraInput
	}
	if snap {
		c.previous, c.velocity, c.initialized = target, Vec2{}, true
		return anchor, nil
	}
	velocity := Vec2{}
	if c.initialized {
		velocity = Vec2{(target.X - c.previous.X) / dt, (target.Y - c.previous.Y) / dt}
	}
	rate := cameraResponse(dt, o.LookaheadDamping)
	velocity = Vec2{c.velocity.X + (velocity.X-c.velocity.X)*rate, c.velocity.Y + (velocity.Y-c.velocity.Y)*rate}
	lead := Vec2{velocity.X * o.LookaheadTime, velocity.Y * o.LookaheadTime}
	if length := math.Hypot(lead.X, lead.Y); length > o.MaxLookahead {
		lead.X, lead.Y = lead.X/length*o.MaxLookahead, lead.Y/length*o.MaxLookahead
	}
	result := Vec2{
		composeCameraAxis(current.X, anchor.X, lead.X, viewSize.X, o.DeadZone.X, o.SoftZone.X, cameraResponse(dt, o.Damping.X)),
		composeCameraAxis(current.Y, anchor.Y, lead.Y, viewSize.Y, o.DeadZone.Y, o.SoftZone.Y, cameraResponse(dt, o.Damping.Y)),
	}
	if !cameraVector(result) || !cameraVector(velocity) {
		return current, ErrCameraInput
	}
	c.previous, c.velocity, c.initialized = target, velocity, true
	return result, nil
}

func composeCameraAxis(current, target, lead, size, dead, soft, rate float64) float64 {
	error := target + lead - current
	halfDead, halfSoft := dead*size/2, soft*size/2
	result := current + (error-max(-halfDead, min(halfDead, error)))*rate
	// The real target takes priority over anticipation at the outer limits.
	return max(target-halfSoft, min(target+halfSoft, result))
}

func cameraResponse(dt, halfLife float64) float64 {
	if halfLife == 0 {
		return 1
	}
	return -math.Expm1(-math.Ln2 * dt / halfLife)
}

func cameraVector(v Vec2) bool {
	return finite(v.X) && finite(v.Y) && math.Abs(v.X) <= 1e9 && math.Abs(v.Y) <= 1e9
}
