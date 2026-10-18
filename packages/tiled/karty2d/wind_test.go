package karty2d

import (
	"math"
	"testing"

	"github.com/karty-game/tiled"
)

func TestWindKeepsAuthoredRootStationary(t *testing.T) {
	for flags := 0; flags < 8; flags++ {
		for _, angle := range []float64{0, 15, -30} {
			s := tiled.Sprite{
				X:            100,
				Y:            200,
				Width:        40,
				Height:       90,
				SourceWidth:  20,
				SourceHeight: 40,
				Rotation:     angle,
				FlipX:        flags&1 != 0,
				FlipY:        flags&2 != 0,
				Diagonal:     flags&4 != 0,
			}
			// Destination root before authored rotation: bottom-center of
			// [100,140]x[200,290], or diagonal [100,190]x[250,290].
			rootX, rootY := 120.0, 290.0
			if s.Diagonal {
				rootX = 145
			}
			dx, dy := rootX-120, rootY-245
			authored := angle * math.Pi / 180
			rootX, rootY = 120+dx*math.Cos(authored)-dy*math.Sin(authored), 245+dx*math.Sin(authored)+dy*math.Cos(authored)
			// Locate the source point attached to that displayed root. GID
			// flips change which source edge forms the rectangle's bottom.
			px, py := 0.0, 20.0
			if s.Diagonal {
				px, py = 10, 0
			}
			if s.FlipY {
				px, py = -px, -py
			}
			for tick := range 600 {
				x, y, sx, sy, a := WindTransform(s, float64(tick)/60, .8, 5.5)
				actualX := x + px*sx*math.Cos(a) - py*sy*math.Sin(a)
				actualY := y + px*sx*math.Sin(a) + py*sy*math.Cos(a)
				if math.Abs(actualX-rootX) > 1e-8 || math.Abs(actualY-rootY) > 1e-8 {
					t.Fatalf(
						"wind moved rendered roots, flags=%d angle=%g tick=%d: (%g,%g) want (%g,%g)",
						flags,
						angle,
						tick,
						actualX,
						actualY,
						rootX,
						rootY,
					)
				}
				baseRotation := authored
				if s.Diagonal {
					baseRotation += math.Pi / 2
				}
				if math.Abs(a-baseRotation) > .8*math.Pi/180+1e-8 {
					t.Fatal("wind exceeded authored amplitude")
				}
			}
		}
	}
}

func TestWindExtremeClockAndSmallPositivePeriodRemainFinite(t *testing.T) {
	s := tiled.Sprite{X: 100, Y: 200, Width: 40, Height: 80, SourceWidth: 20, SourceHeight: 40}
	for _, seconds := range []float64{0, 1, 9e15, math.Inf(1), math.NaN()} {
		x, y, sx, sy, angle := WindTransform(s, seconds, 1, 1e-320)
		if !finite(x) || !finite(y) || !finite(sx) || !finite(sy) || !finite(angle) {
			t.Fatal("wind produced an invalid sprite state", seconds)
		}
	}
}
