package fps

import (
	"math"
	"testing"
)

type body struct {
	yaw, pitch, x, y float32
	looks, moves     int
}

func (b *body) Heading() (float32, float32) { return b.yaw, b.pitch }
func (b *body) Look(yaw, pitch float32)     { b.yaw, b.pitch = yaw, pitch; b.looks++ }
func (b *body) Move(x, y float32)           { b.x, b.y = x, y; b.moves++ }

func controller(t *testing.T, b Body, options Options) *Controller {
	t.Helper()
	c, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
	c.Attach(b)
	return c
}

func near(t *testing.T, got, want float32) {
	t.Helper()
	if math.Abs(float64(got-want)) > .0001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMovementUsesAcceptedHeadingAndNormalizesDiagonal(t *testing.T) {
	b := &body{yaw: math.Pi / 2}
	c := controller(t, b, DefaultOptions())
	c.Update(Intent{Forward: 1, Strafe: 1})
	near(t, float32(math.Hypot(float64(b.x), float64(b.y))), 3.3)
	if b.x <= 0 || b.y >= 0 {
		t.Fatalf("wrong yaw-relative diagonal: %+v", b)
	}
	// Simulate a host collision/portal correction. The controller has no cached pose.
	b.yaw = 0
	c.Update(Intent{Forward: 1, Run: true})
	near(t, b.x, 0)
	near(t, b.y, 6.6)
	c.Update(Intent{})
	near(t, b.x, 0)
	near(t, b.y, 0)
}

func TestOptionsAndInputAreIndependentForTwoInstances(t *testing.T) {
	a, b := &body{}, &body{}
	opts := DefaultOptions()
	opts.Speed = 1.65
	opts.LookSensitivity = .005
	first, second := controller(t, a, DefaultOptions()), controller(t, b, opts)
	first.Update(Intent{Forward: 1})
	second.Update(Intent{Forward: 1})
	near(t, a.y, b.y*2)
	first.Update(Intent{MouseX: 20})
	second.Update(Intent{MouseX: 20})
	near(t, b.yaw, a.yaw*2)
	if first.Options().Speed != 3.3 {
		t.Fatal("options leaked across instances")
	}
}

func TestDisableDetachAndReattachStopWithoutOwningBody(t *testing.T) {
	a, b := &body{}, &body{yaw: math.Pi / 2}
	c := controller(t, a, DefaultOptions())
	c.Update(Intent{Forward: 1})
	c.SetEnabled(false)
	near(t, a.y, 0)
	moves := a.moves
	c.Update(Intent{Forward: 1, MouseX: 100})
	if a.moves != moves || a.yaw != 0 {
		t.Fatal("disabled controller emitted movement/look")
	}
	c.Attach(b)
	c.SetEnabled(true)
	c.Update(Intent{Forward: 1})
	near(t, b.x, 3.3)
	near(t, a.y, 0)
	c.Attach(nil)
	near(t, b.x, 0)
	c.Update(Intent{Forward: 1})
}

func TestLookClampInvalidInputAndTransactionalConfiguration(t *testing.T) {
	b := &body{}
	c := controller(t, b, DefaultOptions())
	c.Update(Intent{MouseY: -10000})
	near(t, b.pitch, c.Options().MaxPitch)
	c.Update(Intent{MouseY: 10000})
	near(t, b.pitch, c.Options().MinPitch)
	before := c.Options()
	invalid := before
	invalid.Speed = float32(math.NaN())
	if c.Configure(invalid) != ErrOptions || c.Options() != before {
		t.Fatal("invalid options changed controller")
	}
	c.Update(Intent{Forward: 1})
	c.Update(Intent{Forward: float32(math.Inf(1))})
	near(t, b.x, 0)
	near(t, b.y, 0)
	zero := before
	zero.Speed, zero.LookSensitivity = 0, 0
	if err := c.Configure(zero); err != nil {
		t.Fatal(err)
	}
	c.Update(Intent{Forward: 1, MouseX: 100})
	near(t, b.x, 0)
	near(t, b.y, 0)
	near(t, b.yaw, 0)
}

func TestUpdateDoesNotAllocate(t *testing.T) {
	b := &body{}
	c := controller(t, b, DefaultOptions())
	if allocations := testing.AllocsPerRun(100, func() { c.Update(Intent{Forward: 1, MouseX: 1}) }); allocations != 0 {
		t.Fatalf("allocations per update: %v", allocations)
	}
}
