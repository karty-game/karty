package controls

import (
	"math"
	"testing"
)

func TestKeyRepeatMultipleBindingsAndMouseConsumption(t *testing.T) {
	bindings := map[string]Action{"w": Forward, "shift-left": Run, "shift-right": Run}
	in := New(bindings)
	bindings["w"] = Backward
	in.Key("w", true)
	in.Key("w", true)
	in.Key("shift-left", true)
	in.Key("shift-right", true)
	in.Key("shift-left", false)
	in.Mouse(3, 4)
	in.Mouse(7, -2)
	first := in.Sample(72, .15)
	if first.Forward != 1 || !first.Run || first.MouseX != 10 || first.MouseY != 2 {
		t.Fatalf("bad input: %+v", first)
	}
	second := in.Sample(72, .15)
	if second.MouseX != 0 || second.MouseY != 0 || second.Forward != 1 {
		t.Fatalf("input not consumed correctly: %+v", second)
	}
	in.Key("shift-right", false)
	in.Key("w", false)
	if next := in.Sample(72, .15); next.Forward != 0 || next.Run {
		t.Fatalf("stuck action: %+v", next)
	}
	if in.Key("unknown", true) {
		t.Fatal("unknown key consumed")
	}
}

func TestTwoGamesCanUseDifferentKeyTypesAndBindings(t *testing.T) {
	type firstKey uint8
	type secondKey string
	first := New(map[firstKey]Action{7: Forward})
	second := New(map[secondKey]Action{"up": Forward})
	first.Key(7, true)
	second.Key("up", true)
	if first.Sample(72, .15) != second.Sample(72, .15) {
		t.Fatal("input depends on game key identity")
	}
	first.Reset()
	if second.Sample(72, .15).Forward != 1 {
		t.Fatal("reset leaked across games")
	}
}

func TestTouchOwnershipIndependentReleaseAndReset(t *testing.T) {
	in := New(map[int]Action{})
	if !in.BeginTouch(0, 11, 20, 20) || !in.BeginTouch(1, 22, 200, 20) {
		t.Fatal("could not claim independent sticks")
	}
	if in.BeginTouch(1, 33, 0, 0) || in.BeginTouch(0, 22, 0, 0) || in.BeginTouch(2, 44, 0, 0) {
		t.Fatal("gesture ownership violated")
	}
	in.MoveTouch(11, 20, -52)
	in.MoveTouch(22, 272, 20)
	intent := in.Sample(72, .15)
	if intent.Forward != 1 || intent.Turn != 1 {
		t.Fatalf("bad sticks: %+v", intent)
	}
	in.EndTouch(11)
	intent = in.Sample(72, .15)
	if intent.Forward != 0 || intent.Turn != 1 || in.Stick(1).ID != 22 {
		t.Fatalf("partial release lost other finger: %+v", intent)
	}
	in.Reset()
	if intent = in.Sample(72, .15); intent.Forward != 0 || intent.Turn != 0 || in.Stick(1).ID != 0 {
		t.Fatal("reset retained touch")
	}
}

func TestDeadzoneBoundsAndNonfiniteInput(t *testing.T) {
	in := New(map[int]Action{})
	in.BeginTouch(0, 1, 0, 0)
	in.MoveTouch(1, 5, 5)
	if in.Sample(72, .15).Strafe != 0 {
		t.Fatal("deadzone failed")
	}
	in.MoveTouch(1, 1000000, 0)
	if in.Sample(72, .15).Strafe != 1 {
		t.Fatal("touch axis not clamped")
	}
	if in.Sample(0, .15).Strafe != 0 || in.Sample(72, 1).Strafe != 0 {
		t.Fatal("invalid touch settings accepted")
	}
	in.Mouse(float32(math.NaN()), 4)
	if in.Sample(72, .15).MouseX != 0 {
		t.Fatal("NaN admitted")
	}
}

func TestSampleDoesNotAllocate(t *testing.T) {
	in := New(map[int]Action{1: Forward})
	in.Key(1, true)
	if allocations := testing.AllocsPerRun(100, func() { in.Sample(72, .15) }); allocations != 0 {
		t.Fatalf("allocations per sample: %v", allocations)
	}
}
