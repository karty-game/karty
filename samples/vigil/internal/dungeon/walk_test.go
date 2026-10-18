package dungeon

import (
	"math"
	"testing"
)

func TestWalkAndHastenCadenceFollowAcceptedTravel(t *testing.T) {
	for _, speed := range []float64{MoveSpeed, HastenSpeed} {
		var walk WalkCycle
		accepted, previous, wraps := 0.0, 0, 0
		for update := range 180 {
			if update%13 == 0 && walk.Advance(0) != previous {
				t.Fatal("pause changed the retained animation phase", speed, update)
			}
			distance := speed / 60
			accepted += distance
			pose := walk.Advance(distance)
			want := int(math.Floor(accepted/WalkStride*WalkFrames+1e-9)) % WalkFrames
			if pose != want {
				t.Fatal("pose drifted from accepted travel", speed, update, pose, want)
			}
			// A fast movement update can legitimately cross more than one
			// rendered pose. Bound the jump by the authored travel per cycle.
			maxDelta := int(math.Ceil(distance / WalkStride * WalkFrames))
			if delta := (pose - previous + WalkFrames) % WalkFrames; delta > maxDelta {
				t.Fatal("walk/hasten cadence jumped beyond accepted travel", speed, update, previous, pose)
			}
			if pose < previous {
				wraps++
			}
			previous = pose
		}
		if wraps < 2 {
			t.Fatal("cadence fixture did not cross enough cycle seams", speed, wraps)
		}
	}
}

func TestWalkCycleKeepsPhaseAcrossLargeAcceptedSteps(t *testing.T) {
	var walk WalkCycle
	unit := WalkStride / WalkFrames
	if pose := walk.Advance((WalkFrames - .5) * unit); pose != WalkFrames-1 {
		t.Fatal("missing final pose", pose)
	}
	// A larger accepted update must advance its true distance, crossing the
	// seam without replaying intermediate poses or restarting the cycle.
	if pose := walk.Advance(2.25 * unit); pose != 1 {
		t.Fatal("large accepted step lost phase across the seam", pose)
	}
	if pose := walk.Advance(0); pose != 1 {
		t.Fatal("pause reset the cycle", pose)
	}
	if pose := walk.Advance(2*WalkStride + .5*unit); pose != 2 {
		t.Fatal("multiple complete strides changed the remaining phase", pose)
	}
}

func TestPredictedWalkPhaseDoesNotAcceptProposedTravelTwice(t *testing.T) {
	var walk WalkCycle
	walk.Advance(WalkStride - .01)
	before := walk.Distance
	phase := walk.PhaseAfter(.02)
	if math.Abs(phase-.01/WalkStride*WalkFrames) > 1e-8 || walk.Distance != before {
		t.Fatal("predicted phase did not wrap without mutation", phase, walk.Distance, before)
	}
	walk.Advance(.02)
	if math.Abs(walk.PhaseAfter(0)-phase) > 1e-8 {
		t.Fatal("accepted travel advanced a second time", walk.PhaseAfter(0), phase)
	}
}
