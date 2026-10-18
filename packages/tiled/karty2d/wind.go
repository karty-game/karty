package karty2d

import (
	"math"

	"github.com/karty-game/tiled"
)

// WindTransform rotates around the rendered rectangle's bottom center instead of letting
// plant roots slide as the image center rotates. Amplitude is in degrees.
func WindTransform(s tiled.Sprite, seconds, amplitude, period float64) (x, y, scaleX, scaleY, rotation float64) {
	x, y = tiled.Center(s)
	scaleX, scaleY, rotation = tiled.Transform(s)
	if amplitude == 0 || period <= 0 || math.IsNaN(amplitude) || math.IsInf(amplitude, 0) || math.IsNaN(period) || math.IsInf(period, 0) ||
		math.IsNaN(seconds) ||
		math.IsInf(seconds, 0) {
		return
	}
	amplitude = max(-2, min(2, amplitude))
	// Nearby plants share the breeze, with a small spatial phase variation.
	a := amplitude * math.Pi / 180 * math.Sin(math.Mod(seconds, period)/period*2*math.Pi+s.X*.007+s.Y*.003)
	// Diagonal flips exchange the scaled rectangle's width and height. The
	// bottom attachment follows the authored rotation, independently of flips.
	halfHeight := s.Height / 2
	if s.Diagonal {
		halfHeight = s.Width / 2
	}
	b := s.Rotation * math.Pi / 180
	x += halfHeight * (math.Sin(b+a) - math.Sin(b))
	y -= halfHeight * (math.Cos(b+a) - math.Cos(b))
	rotation += a
	return
}
