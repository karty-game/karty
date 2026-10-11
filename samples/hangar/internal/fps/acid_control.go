package fps

import "math"

// WalkAcidControl follows the authored observation-room stairs. The host still
// accepts or blocks the proposed segment against the mounted world geometry.
func WalkAcidControl(px, py, pz, x, y float32) (float32, float32, float32) {
	// Window bays are viewing apertures; keep walking on the room side of them.
	if px >= 28 && px <= 40 && py >= 10.5 && py <= 15.45 && x >= 28 && x <= 40 && y > 15.45 {
		y = 15.45
	}
	before, wasInside := acidControlFloor(px, py)
	after, isInside := acidControlFloor(x, y)
	if !isInside {
		return x, y, pz
	}
	if !wasInside {
		return x, y, after + 1.7
	}
	return x, y, pz + after - before
}

// Keep these footprints and 0.2-metre risers aligned with world.yaml.
func acidControlFloor(x, y float32) (float32, bool) {
	switch {
	case x >= 28 && x <= 40 && y >= 10.5 && y <= 15.55:
		return 3.4, true
	case x >= 18 && x < 20 && y >= 12 && y <= 38-x:
		return .2, true
	case x >= 46 && x <= 50 && y >= 12 && y <= 20:
		return .2, true
	case x >= 20 && x < 28 && y >= 12 && y <= 15.55:
		return .4 + float32(math.Floor(float64((x-20)/.5)))*.2, true
	case x > 40 && x < 46 && y >= 12 && y <= 15.55:
		return .4 + float32(math.Floor(float64((46-x)/.375)))*.2, true
	default:
		return 0, false
	}
}
