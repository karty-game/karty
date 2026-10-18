package dungeon

import "math"

// WalkCycle advances by accepted travel, keeping foot cadence consistent when
// hastening or sliding. Waypoint pauses and turns retain the loop's phase.
type WalkCycle struct{ Distance float64 }

const (
	WalkDirections   = 8
	WalkFrames       = 24
	WalkTextureCount = WalkDirections * WalkFrames
	MoveSpeed        = 4.0
	HastenSpeed      = 5.2
	WalkStride       = .70 / .44 * 1.12 // authored support travel; about .445 seconds at the default pace
)

func (w *WalkCycle) Advance(distance float64) int {
	if distance > .00001 {
		w.Distance += distance
	}
	for w.Distance >= WalkStride {
		w.Distance -= WalkStride
	}
	return int(w.Distance/WalkStride*WalkFrames+1e-9) % WalkFrames
}

// PhaseAfter predicts the submitted pose without accepting that travel twice.
// The submitted whole-body pose includes the next move without accepting it twice.
func (w *WalkCycle) PhaseAfter(distance float64) float64 {
	return math.Mod((w.Distance+math.Max(0, distance))/WalkStride*WalkFrames+1e-9, WalkFrames)
}
