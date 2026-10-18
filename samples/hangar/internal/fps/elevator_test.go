package fps

import (
	"math"
	"testing"
)

func TestElevatorStopsTravelAndLocalSupport(t *testing.T) {
	for _, test := range []struct {
		frame uint64
		z     float32
	}{{0, .2}, {179, .2}, {300, 1.8}, {420, 3.4}, {599, 3.4}, {720, 1.8}, {840, .2}} {
		if math.Abs(float64(ElevatorFloor(test.frame)-test.z)) > .00001 {
			t.Fatal(test)
		}
	}
	for frame := uint64(0); frame < 840; frame++ {
		if z := ElevatorFloor(frame); z < .2-.00001 || z > 3.4+.00001 {
			t.Fatal("outside shaft", frame, z)
		}
	}
	id := "acid-control-west-lift#0"
	defer SetFloorZ(id, .2)
	SetFloorZ(id, 3.4)
	if z := StepUp(24, 13.8, 5.1, 24.055, 13.8); z != 5.1 {
		t.Fatal("riding changed camera Z", z)
	}
	if z := StepUp(19.68, 13.8, 1.9, 19.735, 13.8); z != 1.9 {
		t.Fatal("climbed raised elevator from lower landing", z)
	}
	if allocations := testing.AllocsPerRun(30, func() { SetFloorZ(id, ElevatorFloor(300)) }); allocations != 0 {
		t.Fatal(allocations)
	}
}
