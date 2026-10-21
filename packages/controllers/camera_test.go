package controllers

import (
	"math"
	"testing"
)

func framing(t *testing.T, o CameraFramingOptions) *CameraComposer {
	t.Helper()
	c, err := NewCameraComposer(o)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCameraDeadSoftZonesAndAxisDamping(t *testing.T) {
	o := DefaultCameraFramingOptions()
	o.LookaheadTime = 0
	o.DeadZone, o.SoftZone = Vec2{.2, .2}, Vec2{.8, .8}
	o.Damping = Vec2{.1, .2}
	c := framing(t, o)
	view := Vec2{100, 100}
	inside, err := c.Compose(Vec2{}, Vec2{9, -9}, view, .1, false)
	if err != nil || inside != (Vec2{}) {
		t.Fatal("dead zone moved camera", inside, err)
	}
	result, err := c.Compose(Vec2{}, Vec2{30, 30}, view, .1, false)
	if err != nil {
		t.Fatal(err)
	}
	near(t, result.X, 10)
	near(t, result.Y, 20*(1-math.Sqrt(.5)))
	result, err = c.Compose(Vec2{}, Vec2{100, -100}, view, .001, false)
	if err != nil {
		t.Fatal(err)
	}
	near(t, result.X, 60)
	near(t, result.Y, -60)
}

func TestCameraHalfLifeIndependentOfStepSize(t *testing.T) {
	o := DefaultCameraFramingOptions()
	o.DeadZone, o.SoftZone, o.Damping = Vec2{}, Vec2{1, 1}, Vec2{.25, .25}
	o.LookaheadTime = 0
	a, b := framing(t, o), framing(t, o)
	target, view := Vec2{100, -100}, Vec2{1000, 1000}
	one, err := a.Compose(Vec2{}, target, view, .25, false)
	if err != nil {
		t.Fatal(err)
	}
	many := Vec2{}
	for range 30 {
		many, err = b.Compose(many, target, view, .25/30, false)
		if err != nil {
			t.Fatal(err)
		}
	}
	near(t, one.X, 50)
	near(t, many.X, one.X)
	near(t, many.Y, one.Y)
}

func TestCameraLookaheadUsesAcceptedTravelCapsAndResets(t *testing.T) {
	o := DefaultCameraFramingOptions()
	o.Damping, o.DeadZone, o.SoftZone = Vec2{}, Vec2{}, Vec2{1, 1}
	o.LookaheadTime, o.LookaheadDamping, o.MaxLookahead = 1, 0, 2
	c := framing(t, o)
	view := Vec2{100, 100}
	_, _ = c.Compose(Vec2{}, Vec2{}, view, .1, true)
	result, err := c.Compose(Vec2{}, Vec2{3, 4}, view, .1, false)
	if err != nil {
		t.Fatal(err)
	}
	near(t, result.X, 4.2)
	near(t, result.Y, 5.6)
	stopped, _ := c.Compose(result, Vec2{3, 4}, view, .1, false)
	if stopped != (Vec2{3, 4}) {
		t.Fatal("stationary accepted position retained lookahead", stopped)
	}
	reversed, _ := c.Compose(stopped, Vec2{2, 4}, view, .1, false)
	near(t, reversed.X, 0)
	snap, _ := c.Compose(reversed, Vec2{900, -800}, view, .1, true)
	if snap != (Vec2{900, -800}) {
		t.Fatal("teleport retained lead", snap)
	}
	c.Reset()
	reset, _ := c.Compose(snap, Vec2{100, 200}, view, .1, false)
	if reset != (Vec2{100, 200}) {
		t.Fatal("reset inferred a velocity", reset)
	}
}

func TestCameraScreenPositionResizeAndHardLimitOverridesLead(t *testing.T) {
	o := DefaultCameraFramingOptions()
	o.ScreenPosition, o.SoftZone = Vec2{.25, .75}, Vec2{.5, .5}
	o.Damping, o.DeadZone = Vec2{}, Vec2{}
	o.LookaheadTime, o.LookaheadDamping, o.MaxLookahead = 1, 0, 100
	c := framing(t, o)
	for _, view := range []Vec2{{1280, 720}, {720, 1280}, {2560, 720}} {
		result, err := c.Compose(Vec2{}, Vec2{20, 30}, view, 1.0/60, true)
		if err != nil {
			t.Fatal(err)
		}
		near(t, (20-result.X)/view.X+.5, .25)
		near(t, (30-result.Y)/view.Y+.5, .75)
	}
	_, _ = c.Compose(Vec2{}, Vec2{}, Vec2{100, 100}, .1, true)
	result, _ := c.Compose(Vec2{}, Vec2{10, 0}, Vec2{100, 100}, .1, false)
	// Anchor is 35. Anticipation cannot push the true target outside SoftZone.
	near(t, result.X, 60)
}

func TestCameraLookaheadDampingAndIsometricScreenAxes(t *testing.T) {
	o := DefaultCameraFramingOptions()
	o.Damping, o.DeadZone, o.SoftZone = Vec2{}, Vec2{}, Vec2{1, 1}
	o.LookaheadTime, o.LookaheadDamping, o.MaxLookahead = 1, .1, 100
	c := framing(t, o)
	_, _ = c.Compose(Vec2{}, Vec2{}, Vec2{1000, 1000}, .1, true)
	lead, _ := c.Compose(Vec2{}, Vec2{1, 0}, Vec2{1000, 1000}, .1, false)
	near(t, lead.X, 6) // 10 units/s filtered by one half-life gives 5.
	stopped, _ := c.Compose(lead, Vec2{1, 0}, Vec2{1000, 1000}, .1, false)
	near(t, stopped.X, 3.5)

	planar, err := NewIsometric(DefaultIsometricOptions())
	if err != nil {
		t.Fatal(err)
	}
	o.Damping, o.LookaheadTime = Vec2{1.0 / 60, 2.0 / 60}, 0
	c = framing(t, o)
	// This world displacement projects to equal screen-right/down distances.
	target := Vec2{-1 / math.Sqrt2, -3 / math.Sqrt2}
	focus, err := planar.ComposeCamera(c, Vec2{}, target, 1280, 720, false)
	if err != nil {
		t.Fatal(err)
	}
	projected := planar.cameraPlane(focus)
	near(t, projected.X, .5)
	near(t, projected.Y, 1-math.Sqrt(.5))
}

func TestCameraInvalidConfigurationAndUpdatesAreTransactional(t *testing.T) {
	c := framing(t, DefaultCameraFramingOptions())
	_, _ = c.Compose(Vec2{}, Vec2{1, 2}, Vec2{100, 100}, .1, true)
	before := *c
	for _, change := range []func(*CameraFramingOptions){
		func(o *CameraFramingOptions) { o.Damping.X = math.NaN() },
		func(o *CameraFramingOptions) { o.DeadZone.X = -.1 },
		func(o *CameraFramingOptions) { o.SoftZone.Y = .01 },
		func(o *CameraFramingOptions) { o.ScreenPosition.Y = 1 },
		func(o *CameraFramingOptions) { o.MaxLookahead = math.Inf(1) },
		func(o *CameraFramingOptions) { o.LookaheadTime = -1 },
	} {
		o := c.Options()
		change(&o)
		if c.Configure(o) != ErrOptions || *c != before {
			t.Fatal("invalid options changed tracking")
		}
	}
	for _, dt := range []float64{0, -1, 2, math.NaN(), math.SmallestNonzeroFloat64} {
		if _, err := c.Compose(Vec2{}, Vec2{3, 4}, Vec2{100, 100}, dt, false); err != ErrCameraInput || *c != before {
			t.Fatal("invalid dt changed tracking", dt, err)
		}
	}
	for _, size := range []Vec2{{}, {-1, 2}, {math.Inf(1), 100}} {
		if _, err := c.Compose(Vec2{}, Vec2{}, size, .1, false); err != ErrCameraInput || *c != before {
			t.Fatal("invalid view changed tracking", size, err)
		}
	}
}

func TestPlanarCameraCompositionMatchesPickingAndZoom(t *testing.T) {
	for _, iso := range []bool{false, true} {
		o := DefaultTopDown2DOptions()
		if iso {
			o = DefaultIsometricOptions()
		}
		planar, err := newPlanar(o, iso)
		if err != nil {
			t.Fatal(err)
		}
		options := DefaultCameraFramingOptions()
		options.LookaheadTime = 0
		options.Damping, options.DeadZone = Vec2{}, Vec2{}
		options.ScreenPosition = Vec2{.6, .4}
		composer := framing(t, options)
		for _, size := range []Vec2{{1280, 720}, {720, 1280}, {2560, 720}} {
			for _, zoom := range []float64{.9, 1.1} {
				planar.ZoomBy(zoom)
				target := Vec2{12, 9}
				focus, err := planar.ComposeCamera(composer, Vec2{}, target, size.X, size.Y, true)
				if err != nil {
					t.Fatal(err)
				}
				picked, ok := planar.ScreenToGround(Vec2{size.X * .6, size.Y * .4}, focus, size.X, size.Y)
				if !ok {
					t.Fatal("picking rejected composed view")
				}
				near(t, picked.X, target.X)
				near(t, picked.Y, target.Y)
			}
		}
		before := *composer
		if _, err := planar.ComposeCamera(composer, Vec2{}, Vec2{1e8, 1e8}, 1280, 720, true); err != ErrCameraInput || *composer != before {
			t.Fatal("unbounded world focus changed history")
		}
		if _, err := planar.ComposeCamera(nil, Vec2{}, Vec2{}, 1280, 720, false); err != ErrCameraInput {
			t.Fatal("nil composer accepted")
		}
	}
}

func TestCameraCompositionSteadyUpdatesDoNotAllocate(t *testing.T) {
	planar, err := NewIsometric(DefaultIsometricOptions())
	if err != nil {
		t.Fatal(err)
	}
	composer := framing(t, DefaultCameraFramingOptions())
	focus, target := Vec2{}, Vec2{}
	if got := testing.AllocsPerRun(100, func() {
		target.X += .01
		focus, err = planar.ComposeCamera(composer, focus, target, 1280, 720, false)
	}); got != 0 || err != nil {
		t.Fatal("camera update allocated or failed", got, err)
	}
}
