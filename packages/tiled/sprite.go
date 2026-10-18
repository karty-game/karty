package tiled

import (
	"fmt"
	"math"
)

// ImageKey identifies the source rectangle described by the tileset. Importers
// may resolve it to a Karty texture without exposing atlas coordinates to games.
func ImageKey(s Sprite) string {
	return fmt.Sprintf("%s#%d,%d,%d,%d", s.Image, s.SourceX, s.SourceY, s.SourceWidth, s.SourceHeight)
}

// Transform returns a centered sprite's scale and clockwise rotation. Tiled
// scales the source image before its diagonal axis swap, horizontal/vertical
// flips and authored rotation. Use Center for the matching position.
func Transform(s Sprite) (scaleX, scaleY, rotation float64) {
	h, v := 1.0, 1.0
	if s.FlipX {
		h = -1
	}
	if s.FlipY {
		v = -1
	}
	rotation = s.Rotation * math.Pi / 180
	if s.Diagonal {
		return v * s.Width / float64(s.SourceWidth), -h * s.Height / float64(s.SourceHeight), rotation + math.Pi/2
	}
	return h * s.Width / float64(s.SourceWidth), v * s.Height / float64(s.SourceHeight), rotation
}

// Center returns the rendered image center. Diagonal flips exchange the scaled
// image dimensions while retaining Tiled's bottom-left placement. The resulting
// center correction follows the object's authored rotation.
func Center(s Sprite) (x, y float64) {
	x, y = s.X+s.Width/2, s.Y+s.Height/2
	if s.Diagonal {
		d := (s.Height - s.Width) / 2
		a := s.Rotation * math.Pi / 180
		x += d * (math.Cos(a) - math.Sin(a))
		y += d * (math.Sin(a) + math.Cos(a))
	}
	return
}
