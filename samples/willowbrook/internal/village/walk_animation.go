package village

import (
	"math"

	"github.com/karty-game/tiled"
)

// HeroStride is the world-pixel distance for one four-pose walk cycle.
// Running uses a longer stride, so travel speed does not produce a frantic gait.
func HeroStride(running bool) float64 { return heroStride(Level.Properties, running) }

func heroStride(properties tiled.Properties, running bool) float64 {
	name, fallback := "heroWalkStride", 64.0
	if running {
		name, fallback = "heroRunStride", 80
	}
	stride := properties.Number(name, fallback)
	if stride <= 0 || math.IsNaN(stride) || math.IsInf(stride, 0) {
		return fallback
	}
	return stride
}

// AdvanceWalkPhase advances a normalized cycle by actual distance travelled.
// A stationary or blocked character returns to the start of its next stride.
func AdvanceWalkPhase(phase, distance, stride float64) float64 {
	if distance <= 0 || math.IsNaN(distance) || math.IsInf(distance, 0) {
		return 0
	}
	if math.IsNaN(phase) || math.IsInf(phase, 0) || phase < 0 {
		phase = 0
	}
	if stride <= 0 || math.IsNaN(stride) || math.IsInf(stride, 0) {
		stride = 64
	}
	return math.Mod(phase+distance/stride, 1)
}

// WalkFrame selects the two passing poses and neutral pose without time-based
// motion when idle. Texture order is left step, neutral, right step, neutral.
func WalkFrame(phase float64, walking bool) int {
	if !walking {
		return 1
	}
	return [4]int{0, 1, 2, 1}[int(phase*4)%4]
}
