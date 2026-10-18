package village

import (
	"math"
	"testing"

	"github.com/karty-game/tiled"
)

func TestHeroWalkCadenceTracksDistanceAtWalkingAndRunningSpeeds(t *testing.T) {
	for _, test := range []struct {
		name    string
		speed   float64
		running bool
		steps   int
	}{
		{"walking", 108.8, false, 68},
		{"running", 176, true, 88},
	} {
		t.Run(test.name, func(t *testing.T) {
			stride := heroStride(nil, test.running)
			phase := 0.0
			previous := WalkFrame(phase, true)
			changes := 0
			for range 600 { // Ten seconds at the real 60 Hz update rate.
				phase = AdvanceWalkPhase(phase, test.speed/60, stride)
				frame := WalkFrame(phase, true)
				if frame != previous {
					changes++
				}
				previous = frame
			}
			// One pose boundary can land exactly on the final simulation tick.
			if changes < test.steps-1 || changes > test.steps+1 {
				t.Fatalf("cadence changed: %d pose steps in 10 s; want about %d", changes, test.steps)
			}
			if phase < 0 || phase >= 1 {
				t.Fatal("walk phase grew outside one bounded cycle", phase)
			}
		})
	}
}

func TestHeroWalkStrideAndNeutralRest(t *testing.T) {
	phase := 0.0
	for _, want := range []int{1, 2, 1, 0} {
		phase = AdvanceWalkPhase(phase, 16, 64)
		if got := WalkFrame(phase, true); got != want {
			t.Fatalf("64 px stride pose: got %d, want %d", got, want)
		}
	}
	phase = AdvanceWalkPhase(.7, 0, 64)
	if phase != 0 || WalkFrame(phase, false) != 1 {
		t.Fatal("idle character did not return to a neutral stance", phase)
	}
	for _, distance := range []float64{-1, math.NaN(), math.Inf(1)} {
		if got := AdvanceWalkPhase(.7, distance, 64); got != 0 {
			t.Fatal("invalid/stationary distance animated the hero", got)
		}
	}
}

func TestHeroWalkStrideReadsTiledPropertiesWithSafeFallbacks(t *testing.T) {
	for _, test := range []struct {
		properties tiled.Properties
		walking    float64
		running    float64
	}{
		{nil, 64, 80},
		{tiled.Properties{{Name: "heroWalkStride", Value: float64(96)}, {Name: "heroRunStride", Value: float64(112)}}, 96, 112},
		{tiled.Properties{{Name: "heroWalkStride", Value: float64(0)}, {Name: "heroRunStride", Value: float64(-80)}}, 64, 80},
		{tiled.Properties{{Name: "heroWalkStride", Value: "fast"}, {Name: "heroRunStride", Value: math.Inf(1)}}, 64, 80},
	} {
		if got := heroStride(test.properties, false); got != test.walking {
			t.Fatal("walking stride property/fallback", got, test.walking)
		}
		if got := heroStride(test.properties, true); got != test.running {
			t.Fatal("running stride property/fallback", got, test.running)
		}
	}
}
