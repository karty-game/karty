package tiled

// FrameAt follows the editor's millisecond durations, including uneven holds.
// All instances share a clock so adjacent animated water tiles stay seamless.
func FrameAt(frames []Frame, milliseconds int64) int {
	if len(frames) > MaxAnimationFrames {
		return -1
	}
	var period int64
	for _, f := range frames {
		if f.Duration <= 0 || f.Duration > 60000 {
			return -1
		}
		period += int64(f.Duration)
	}
	if period <= 0 {
		return -1
	}
	t := milliseconds % period
	if t < 0 {
		t += period
	}
	for i, f := range frames {
		t -= int64(f.Duration)
		if t < 0 {
			return i
		}
	}
	return 0
}
