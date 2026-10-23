package elevator

import (
	"math"
	"testing"
)

func TestStopsTravelBoundsAndAllocations(t *testing.T) {
	for _, test := range []struct {
		frame uint64
		z     float32
	}{{0, .2}, {179, .2}, {300, 1.8}, {420, 3.4}, {599, 3.4}, {720, 1.8}, {840, .2}} {
		if math.Abs(float64(Floor(test.frame)-test.z)) > .00001 {
			t.Fatal(test)
		}
	}
	for frame := uint64(0); frame < 840; frame++ {
		if z := Floor(frame); z < .2-.00001 || z > 3.4+.00001 {
			t.Fatal("outside shaft", frame, z)
		}
	}
	var z float32
	if allocations := testing.AllocsPerRun(30, func() { z = Floor(300) }); allocations != 0 {
		t.Fatal(allocations)
	}
	if math.Abs(float64(z-1.8)) > .00001 {
		t.Fatal("allocation check did not evaluate the elevator schedule", z)
	}
}
