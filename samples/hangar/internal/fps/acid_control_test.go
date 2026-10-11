package fps

import (
	"math"
	"testing"
)

func TestAcidControlStairRoundTrip(t *testing.T) {
	for _, route := range []struct{ start, end, step float32 }{{19, 31, .055}, {48, 37, -.055}} {
		x, y, z := route.start, float32(13.8), float32(1.9)
		for (route.end-x)*route.step > 0 {
			x, y, z = WalkAcidControl(x, y, z, x+route.step, y)
		}
		if math.Abs(float64(z-5.1)) > .0001 {
			t.Fatalf("control room eye height = %v", z)
		}
		for (x-route.start)*route.step > 0 {
			x, y, z = WalkAcidControl(x, y, z, x-route.step, y)
		}
		if math.Abs(float64(z-1.9)) > .0001 {
			t.Fatalf("tunnel eye height after descending = %v", z)
		}
	}
}

func TestAcidControlWindowsAndExistingWalk(t *testing.T) {
	x, y, z := WalkAcidControl(31, 15.4, 5.1, 31, 15.5)
	if x != 31 || y != 15.45 || z != 5.1 {
		t.Fatalf("window allowed walking out of observation room: %v,%v,%v", x, y, z)
	}
	x, y, z = WalkAcidControl(27, 23, 1.9, 27, 23.055)
	if x != 27 || y != 23.055 || z != 1.9 {
		t.Fatalf("existing walkway changed: %v,%v,%v", x, y, z)
	}
}
