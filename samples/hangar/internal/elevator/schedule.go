// Package elevator describes the observation-room platform's fixed schedule.
package elevator

// Floor pauses three seconds at each landing and travels in four seconds.
// The fixed 60 Hz game update keeps platform motion independent of draw rate.
func Floor(frame uint64) float32 {
	phase := frame % 840
	switch {
	case phase < 180:
		return .2
	case phase < 420:
		return .2 + elevatorTravel(float32(phase-180)/240)
	case phase < 600:
		return 3.4
	default:
		return 3.4 - elevatorTravel(float32(phase-600)/240)
	}
}

// Ease acceleration at each stop so a descending floor retains its rider.
func elevatorTravel(t float32) float32 { return 3.2 * t * t * (3 - 2*t) }
