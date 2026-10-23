// Package fps contains the cove's authored swimming rules.
package fps

import "math"

// Motion carries fixed-tick displacement and the current look direction.
type Motion struct{ X, Y, Z, Yaw, Pitch float32 }

// OceanSwimming uses the authored ocean's level and dry bunker footprint. The
// host still resolves the player's capsule against terrain, ramps and room walls.
// Entry occurs only when the head approaches the water, so shallow beach water
// retains walking and ordinary gravity.
func OceanSwimming(x, y, eyeZ, seaZ float32) bool {
	if x >= 24 && x <= 40 && y >= -56 && y <= -40 {
		return false
	}
	return eyeZ <= seaZ+.55
}

// SwimVelocity produces metres per second for a neutral-buoyancy swimmer.
// Forward follows the look direction, allowing both touch sticks to dive and
// surface without additional buttons. Space and C/Ctrl offer direct vertical
// movement on a keyboard. The physics body, rather than camera teleports,
// resolves every proposed movement against the seabed and solid geometry.
func SwimVelocity(motion Motion, eyeZ, seaZ float32) (x, y, z float32) {
	sine, cosine := float32(math.Sin(float64(motion.Yaw))), float32(math.Cos(float64(motion.Yaw)))
	forward := (motion.X*sine + motion.Y*cosine) * 60
	strafe := (motion.X*cosine - motion.Y*sine) * 60
	pitchSine, pitchCosine := float32(math.Sin(float64(motion.Pitch))), float32(math.Cos(float64(motion.Pitch)))
	x = (forward*pitchCosine*sine + strafe*cosine) * .8
	y = (forward*pitchCosine*cosine - strafe*sine) * .8
	z = forward*pitchSine*.8 + motion.Z*2.6
	if length := float32(math.Sqrt(float64(x*x + y*y + z*z))); length > 2.8 {
		x, y, z = x*2.8/length, y*2.8/length, z*2.8/length
	}
	// Float with the eye above the surface. A deliberate dive or looking down
	// while moving overrides this settling; releasing input below water holds
	// depth. Clamp upward travel so swimming cannot launch the body into the air.
	const eyeAboveSurface = float32(.35)
	if eyeZ > seaZ+eyeAboveSurface && z >= 0 {
		z = (seaZ + eyeAboveSurface - eyeZ) * 4
	}
	if z > 0 {
		z = min(z, max(0, (seaZ+eyeAboveSurface-eyeZ)*60))
	}
	return x, y, z
}
