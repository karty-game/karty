package ssbump_test

import (
	"math"
	"testing"

	"github.com/karty-game/karty/internal/ssbump"
)

func TestReprojectPreservesOcclusionAndDirectionalBasis(t *testing.T) {
	t.Parallel()

	want := [3]float64{.1, .2, .05}
	identity := [2][2]float64{{1, 0}, {0, 1}}

	got, err := ssbump.Reproject(want, identity)
	if err != nil {
		t.Fatal(err)
	}

	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatal("identity", got)
		}
	}

	angle := 2 * math.Pi / 3

	rotated, err := ssbump.Reproject(want, [2][2]float64{{math.Cos(angle), math.Sin(angle)}, {-math.Sin(angle), math.Cos(angle)}})
	if err != nil {
		t.Fatal(err)
	}
	// A +120 degree source-frame rotation maps B0->B1, B1->B2, B2->B0.
	for i := range want {
		if math.Abs(rotated[(i+1)%3]-want[i]) > 1e-12 {
			t.Fatal("basis permutation", rotated)
		}
	}

	for _, axes := range [][2][2]float64{{{0, -1}, {1, 0}}, {{-1, 0}, {0, 1}}, {{math.Cos(.7), math.Sin(.7)}, {-math.Sin(.7), math.Cos(.7)}}} {
		r, err := ssbump.Reproject(want, axes)
		if err != nil || math.Abs(r[0]+r[1]+r[2]-.35) > 1e-12 {
			t.Fatal("lost occlusion energy", r, err)
		}
	}

	if _, err := ssbump.Reproject(want, [2][2]float64{{1, 0}, {1, 0}}); err == nil {
		t.Fatal("degenerate tangent frame accepted")
	}
}
