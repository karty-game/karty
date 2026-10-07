package fps

import (
	"math"
	"testing"
)

func TestDesktopMovementAndMouse(t *testing.T) {
	input := New(1280, 720)
	input.Key(Forward, true)
	input.Key(Right, true)
	motion := input.Step(0, 0)
	if math.Abs(float64(hypot(motion.X, motion.Y)-.055)) > .00001 {
		t.Fatalf("diagonal walk changed speed: %+v", motion)
	}
	input.Key(RunLeft, true)
	motion = input.Step(0, 0)
	if speed := hypot(motion.X, motion.Y); math.Abs(float64(speed-.11)) > .00001 {
		t.Fatalf("run speed: %v", speed)
	}
	input.Reset()
	input.Mouse(100, -80)
	motion = input.Step(0, 0)
	if motion.Yaw != .25 || math.Abs(float64(motion.Pitch-.2)) > .00001 {
		t.Fatalf("mouse look direction: %+v", motion)
	}
	if next := input.Step(motion.Yaw, motion.Pitch); next != motion {
		t.Fatalf("mouse displacement was applied twice: %+v", next)
	}
	input.Mouse(0, -10000)
	if got := input.Step(0, 0).Pitch; got != 1.35 {
		t.Fatalf("pitch clamp: %v", got)
	}
}

func TestTwoIndependentTouchSticks(t *testing.T) {
	input := New(1280, 720)
	if !input.BeginTouch(0, 1, 140, 520) || !input.BeginTouch(1, 2, 1140, 520) {
		t.Fatal("both sticks must accept simultaneous fingers")
	}
	if input.BeginTouch(0, 3, 200, 520) || input.BeginTouch(1, 1, 1140, 520) {
		t.Fatal("a finger stole an occupied stick")
	}
	input.MoveTouch(1, 140, 500)  // Beyond the deadzone, below full speed.
	input.MoveTouch(2, 1500, 520) // Clamps at the rim.
	if got := hypot(input.Sticks[1].DX, input.Sticks[1].DY); got != StickRadius {
		t.Fatalf("thumb escaped base: %v", got)
	}
	motion := input.Step(0, 0)
	if motion.Y <= 0 || motion.Yaw <= 0 {
		t.Fatalf("simultaneous walk and look: %+v", motion)
	}
	input.EndTouch(2)
	motion = input.Step(0, 0)
	if motion.Y <= 0 || motion.Yaw != 0 {
		t.Fatalf("releasing look stopped walking: %+v", motion)
	}
	input.EndTouch(1)
	if motion := input.Step(0, 0); motion != (Motion{}) {
		t.Fatalf("release left stale input: %+v", motion)
	}
}

func TestDeadzoneAndReset(t *testing.T) {
	input := New(1280, 720)
	input.BeginTouch(0, 1, 140, 520)
	input.MoveTouch(1, 145, 525)
	if motion := input.Step(0, 0); motion != (Motion{}) {
		t.Fatalf("deadzone moved camera: %+v", motion)
	}
	input.MoveTouch(1, 140, 400)
	input.BeginTouch(1, 2, 1140, 520)
	input.MoveTouch(2, 1200, 520)
	input.Key(Forward, true)
	input.Mouse(50, 50)
	input.Reset()
	if motion := input.Step(.7, -.3); motion != (Motion{Yaw: .7, Pitch: -.3}) {
		t.Fatalf("focus/viewpoint reset left stale input: %+v", motion)
	}
	if !input.BeginTouch(0, 3, 140, 520) {
		t.Fatal("reset retained a finger owner")
	}
}
