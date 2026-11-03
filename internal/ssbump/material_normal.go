package ssbump

import (
	"image"
	"math"
)

func materialNormal(source image.Image, column, row int, strength float64) (vector, error) {
	red, green, _, alpha := source.At(source.Bounds().Min.X+column, source.Bounds().Min.Y+row).RGBA()
	if alpha != opaque16 {
		return vector{}, ErrInput
	}

	const neutral, scale = 128.0, 127.0

	x := min(max((float64(red)/257-neutral)/scale, -1), 1) * strength
	y := min(max((float64(green)/257-neutral)/scale, -1), 1) * strength
	length := math.Sqrt(max(1, x*x+y*y))
	x, y = x/length, y/length

	return vector{x, y, math.Sqrt(max(0, 1-x*x-y*y))}, nil
}
