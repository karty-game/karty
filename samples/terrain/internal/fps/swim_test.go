package fps

import (
	"math"
	"testing"
)

func TestOceanSwimmingKeepsBeachAndBunkerDry(t *testing.T) {
	for _, test := range []struct {
		name         string
		x, y, z, sea float32
		wet          bool
	}{
		{"surface", -74, -70, .35, 0, true},
		{"underwater", -74, -70, -3, 0, true},
		{"beach walking", -74, -70, 1.1, 0, false},
		{"dry underground bunker", 32, -48, -3, 0, false},
		{"outside bunker", 23.9, -48, -3, 0, true},
		{"raised sea", -74, -70, 10.35, 10, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := OceanSwimming(test.x, test.y, test.z, test.sea); got != test.wet {
				t.Fatalf("swimming = %v, want %v", got, test.wet)
			}
		})
	}
}

func TestSwimmingFollowsPitchWithoutTurningStrafeIntoDive(t *testing.T) {
	input := New(1280, 720)
	input.Key(Forward, true)
	x, y, z := SwimVelocity(input.Step(0, -.6), -3, 0)
	if x != 0 || y <= 0 || z >= 0 {
		t.Fatalf("looking down must swim forwards and dive: %v %v %v", x, y, z)
	}
	input.Reset()
	input.Key(Right, true)
	x, y, z = SwimVelocity(input.Step(0, -.6), -3, 0)
	if x <= 0 || y != 0 || z != 0 {
		t.Fatalf("strafe gained vertical movement: %v %v %v", x, y, z)
	}
	input.Reset()
	input.BeginTouch(0, 1, 140, 520)
	input.MoveTouch(1, 140, 400)
	_, y, z = SwimVelocity(input.Step(0, .6), -3, 0)
	if y <= 0 || z <= 0 {
		t.Fatalf("touch forward and look up must surface: %v %v", y, z)
	}
}

func TestVerticalSwimmingAndSurfaceLimit(t *testing.T) {
	input := New(1280, 720)
	input.Key(SwimDown, true)
	_, _, z := SwimVelocity(input.Step(0, 0), .35, 0)
	if z >= 0 {
		t.Fatalf("dive failed at surface: %v", z)
	}
	input.Key(SwimUp, true)
	if motion := input.Step(0, 0); motion.Z != 0 {
		t.Fatalf("opposed vertical keys did not cancel: %+v", motion)
	}
	input.Key(SwimDown, false)
	input.Key(Forward, true)
	input.Key(RunLeft, true)
	x, y, z := SwimVelocity(input.Step(.8, 1.2), -3, 0)
	if speed := math.Sqrt(float64(x*x + y*y + z*z)); speed > 2.80001 || z <= 0 {
		t.Fatalf("swim speed or ascent invalid: %v, %v", speed, z)
	}
	_, _, z = SwimVelocity(input.Step(0, 0), .34, 0)
	if z <= 0 || z > .60001 {
		t.Fatalf("ascent overshoots surface: %v", z)
	}
	_, _, z = SwimVelocity(input.Step(0, 0), .35, 0)
	if z != 0 {
		t.Fatalf("surface ascent launched swimmer: %v", z)
	}
	input.Reset()
	_, _, z = SwimVelocity(input.Step(0, 0), -3, 0)
	if z != 0 {
		t.Fatalf("released input did not hold depth: %v", z)
	}
	_, _, z = SwimVelocity(input.Step(0, 0), .5, 0)
	if z >= 0 {
		t.Fatalf("entering swimmer did not settle to surface: %v", z)
	}
}
