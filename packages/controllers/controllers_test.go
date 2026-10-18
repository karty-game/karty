package controllers

import (
	"errors"
	"math"
	"testing"
)

func TestFirstPersonAcceptedHeadingAndInputLifetime(t *testing.T) {
	c, err := NewFirstPerson(DefaultFirstPersonOptions())
	if err != nil {
		t.Fatal(err)
	}
	c.Key(Forward, true)
	c.Key(Right, true)
	m := c.Step(0, 0)
	near(t, math.Hypot(float64(m.X), float64(m.Y)), 3.3/60)
	c.Key(RunLeft, true)
	c.Key(RunRight, true)
	c.Key(RunLeft, false)
	m = c.Step(math.Pi/2, 0)
	near(t, math.Hypot(float64(m.X), float64(m.Y)), 6.6/60)
	if m.X <= 0 || m.Y >= 0 {
		t.Fatalf("did not use accepted heading: %+v", m)
	}
	c.Reset()
	c.Mouse(100, -80)
	m = c.Step(0, 0)
	near(t, float64(m.Yaw), .25)
	near(t, float64(m.Pitch), .2)
	if c.Step(m.Yaw, m.Pitch) != m {
		t.Fatal("mouse displacement consumed more than once")
	}
	c.Mouse(0, -10000)
	near(t, float64(c.Step(0, 0).Pitch), 1.35)
	c.Reset()
	if got := c.Step(0, 0); got != (Motion{}) {
		t.Fatalf("reset retained input: %+v", got)
	}
}

func TestFirstPersonPointerOwnershipAndUICancellation(t *testing.T) {
	c, err := NewFirstPerson(DefaultFirstPersonOptions())
	if err != nil {
		t.Fatal(err)
	}
	down := Pointer{Phase: PointerDown, Primary: true, X: 140, Y: 520}
	if r := c.Pointer(down, false); r.Capture {
		t.Fatal("UI click captured mouse")
	}
	if r := c.Pointer(down, true); !r.Capture {
		t.Fatal("scene click did not capture")
	}
	c.Pointer(Pointer{Phase: PointerMove, Captured: true, DeltaX: 20}, false)
	if c.Step(0, 0).Yaw <= 0 {
		t.Fatal("captured mouse did not turn")
	}
	if r := c.Pointer(Pointer{Phase: PointerCancel}, false); !r.Release {
		t.Fatal("cancel did not request release")
	}
	down.Touch, down.ID = true, 1
	if r := c.Pointer(down, false); r.TouchStarted || c.Sticks[0].ID != 0 {
		t.Fatal("UI touch claimed stick")
	}
	if r := c.Pointer(down, true); !r.TouchStarted {
		t.Fatal("touch overlay not requested")
	}
	down.ID, down.X = 2, 1100
	c.Pointer(down, true)
	if c.BeginTouch(0, 3, 140, 520) || c.BeginTouch(1, 1, 1100, 520) {
		t.Fatal("gesture ownership stolen")
	}
	c.Pointer(Pointer{Touch: true, Phase: PointerMove, ID: 1, X: 140, Y: 300}, false)
	c.Pointer(Pointer{Touch: true, Phase: PointerMove, ID: 2, X: 1300, Y: 520}, false)
	m := c.Step(0, 0)
	if m.Y <= 0 || m.Yaw <= 0 {
		t.Fatalf("dual-stick movement missing: %+v", m)
	}
	if math.Hypot(float64(c.Sticks[1].DX), float64(c.Sticks[1].DY)) > 96.00001 {
		t.Fatal("stick escaped radius")
	}
	c.Pointer(Pointer{Touch: true, Phase: PointerCancel, ID: 1}, false)
	m = c.Step(0, 0)
	if m.X != 0 || m.Y != 0 || m.Yaw <= 0 {
		t.Fatalf("partial cancellation affected other finger: %+v", m)
	}
	c.Reset()
	if c.Step(0, 0) != (Motion{}) || c.Sticks[1].ID != 0 {
		t.Fatal("focus reset retained touch input")
	}
	if !c.TouchMode() {
		t.Fatal("reset discarded persistent overlay mode")
	}
}

func TestFirstPersonOptionsAndInvalidInput(t *testing.T) {
	c, err := NewFirstPerson(DefaultFirstPersonOptions())
	if err != nil {
		t.Fatal(err)
	}
	before := c.Options()
	for _, invalid := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		o := before
		o.Speed = invalid
		if !errors.Is(c.Configure(o), ErrOptions) || c.Options() != before {
			t.Fatal("invalid configuration changed state")
		}
	}
	c.Key(Control(255), true)
	c.Mouse(float32(math.Inf(1)), 0)
	if c.BeginTouch(0, 0, 0, 0) || c.BeginTouch(2, 1, 0, 0) || c.BeginTouch(0, 1, float32(math.NaN()), 0) {
		t.Fatal("accepted invalid finger")
	}
	if c.Step(0, 0) != (Motion{}) {
		t.Fatal("invalid input moved camera")
	}
	o := before
	o.Speed, o.MouseSensitivity = 0, 0
	if err := c.Configure(o); err != nil {
		t.Fatal(err)
	}
	c.Key(Forward, true)
	c.Mouse(100, 0)
	if c.Step(0, 0) != (Motion{}) {
		t.Fatal("explicit zero configuration ignored")
	}
}

func TestTopDownAliasesTargetsAndFollowing(t *testing.T) {
	o := DefaultTopDown2DOptions()
	o.FollowOffset = Vec2{Y: -110}
	c, err := NewTopDown2D(o)
	if err != nil {
		t.Fatal(err)
	}
	c.Key(Forward, true)
	c.Key(ForwardAlt, true)
	c.Key(Forward, false)
	d := c.Step(Vec2{})
	near(t, d.Y, -108.8/60)
	c.Key(RunLeft, true)
	c.Key(RunRight, true)
	c.Key(RunLeft, false)
	near(t, c.Step(Vec2{}).Y, -176.0/60)
	c.Reset()
	c.SetTarget(Vec2{X: .75})
	d = c.Step(Vec2{})
	near(t, d.X, .75)
	if c.Step(Vec2{X: .75}) != (Vec2{}) || c.HasTarget() {
		t.Fatal("target overshot or retained after arrival")
	}
	c.SetTarget(Vec2{X: 20})
	c.Key(Left, true)
	if c.Step(Vec2{}).X >= 0 || c.HasTarget() {
		t.Fatal("keyboard did not take over target")
	}
	c.Reset()
	if got := c.Follow(Vec2{}, Vec2{X: 100, Y: 200}, true); got != (Vec2{100, 90}) {
		t.Fatalf("snap camera: %+v", got)
	}
	got := c.Follow(Vec2{}, Vec2{X: 100, Y: 200}, false)
	near(t, got.X, 12)
	near(t, got.Y, 10.8)
	ground, ok := c.ScreenToGround(Vec2{640, 360}, Vec2{100, 90}, 1280, 720)
	if !ok || ground != (Vec2{100, 90}) {
		t.Fatalf("screen center: %+v", ground)
	}
}

func TestIsometricBasisProjectionAndZoom(t *testing.T) {
	c, err := NewIsometric(DefaultIsometricOptions())
	if err != nil {
		t.Fatal(err)
	}
	c.Key(Forward, true)
	d := c.Step(Vec2{})
	near(t, d.X, 4.0/60/math.Sqrt2)
	near(t, d.Y, d.X)
	c.Key(Right, true)
	d = c.Step(Vec2{})
	near(t, d.X, 4.0/60)
	near(t, d.Y, 0)
	for _, factor := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		near(t, c.ZoomBy(factor), 13.2)
	}
	near(t, c.ZoomBy(100), 22)
	near(t, c.ZoomBy(.001), 10)
	for _, size := range []Vec2{{1280, 720}, {720, 1280}, {2560, 720}} {
		focus := Vec2{12, 4}
		ground, ok := c.ScreenToGround(Vec2{size.X/2 + size.Y/10, size.Y/2 - size.Y/20}, focus, size.X, size.Y)
		if !ok {
			t.Fatal("valid viewport rejected")
		}
		near(t, ground.X, focus.X+math.Sqrt2)
		near(t, ground.Y, focus.Y)
	}
	if _, ok := c.ScreenToGround(Vec2{}, Vec2{}, 0, 720); ok {
		t.Fatal("zero viewport accepted")
	}
	position := c.CameraPosition(Vec2{12, 4})
	if position != (Vec3{-6, -14, 14.696938}) {
		t.Fatalf("camera offset: %+v", position)
	}
	o := c.Options()
	o.Pitch = 0
	if !errors.Is(c.Configure(o), ErrOptions) || c.Options().Pitch == 0 {
		t.Fatal("singular projection accepted")
	}
}

// Keep this serial: Go 1.27 rejects AllocsPerRun inside parallel tests.
func TestSteadyUpdatesDoNotAllocate(t *testing.T) {
	fps, err := NewFirstPerson(DefaultFirstPersonOptions())
	if err != nil {
		t.Fatal(err)
	}
	iso, err := NewIsometric(DefaultIsometricOptions())
	if err != nil {
		t.Fatal(err)
	}
	fps.BeginTouch(0, 1, 140, 520)
	fps.Key(Forward, true)
	iso.Key(Right, true)
	if got := testing.AllocsPerRun(100, func() {
		fps.MoveTouch(1, 150, 400)
		fps.Mouse(1, 1)
		fps.Step(0, 0)
		iso.Step(Vec2{})
		iso.Follow(Vec2{}, Vec2{1, 2}, false)
		iso.ScreenToGround(Vec2{640, 360}, Vec2{}, 1280, 720)
	}); got != 0 {
		t.Fatalf("steady update allocated: %v", got)
	}
}

func near(t *testing.T, got, want float64) {
	t.Helper()
	if !finite(got) || math.Abs(got-want) > .00001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
