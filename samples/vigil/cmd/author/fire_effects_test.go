package main

import (
	"math"
	"testing"
)

func TestFireTrailFollowsProjectedFacing(t *testing.T) {
	t.Parallel()

	// All eight images have their alpha mass behind the hot core, rather than
	// accidentally mirroring screen-up/down or the dimetric diagonals.
	for facing := range 8 {
		angle := float64(facing) * math.Pi / 4
		tailX, tailY := math.Sin(angle), -math.Cos(angle)
		var mass, sumX, sumY float64

		for y := range 96 {
			for x := range 96 {
				px, py := (float64(x)+.5)/48-1, (float64(y)+.5)/48-1
				alpha := float64(fireTrailPixel(px*tailX+py*tailY, -px*tailY+py*tailX).A)
				mass += alpha
				sumX += px * alpha
				sumY += py * alpha
			}
		}

		along, across := (sumX*tailX+sumY*tailY)/mass, (-sumX*tailY+sumY*tailX)/mass
		if mass == 0 || along < .13 || math.Abs(across) > .04 {
			t.Fatalf("trail %d loses travel direction: mass=%g along=%g across=%g", facing, mass, along, across)
		}
	}
}

func TestFireCoreAndSmokeHaveDistinctExposure(t *testing.T) {
	t.Parallel()

	core := fireCorePixel(0, 0)
	if core.R != 255 || core.G != 255 || core.B != 250 || core.A != 255 {
		t.Fatal("core must retain a compact near-white hot center", core)
	}

	dense, faint := fireSmokePixel(0, 0, 0), fireSmokePixel(0, 0, 1)
	if dense.A <= faint.A*2 || faint.A == 0 || dense.R > 100 {
		t.Fatal("smoke tail needs restrained color and fading exposure", dense, faint)
	}

	if fireCorePixel(1, 0).A != 0 || fireFlashPixel(1, 0).A != 0 || fireTrailPixel(1, 0).A != 0 {
		t.Fatal("hot artwork must clear its card boundary")
	}
}
