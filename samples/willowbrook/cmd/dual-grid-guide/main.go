package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func dirt(m int, x, y float64) bool {
	n := 0
	for i := 0; i < 4; i++ {
		if m&(1<<i) != 0 {
			n++
		}
	}
	if n == 0 {
		return false
	}
	if n == 4 {
		return true
	}
	if n == 2 {
		if m == 3 {
			return y < .5
		}
		if m == 12 {
			return y >= .5
		}
		if m == 5 {
			return x < .5
		}
		if m == 10 {
			return x >= .5
		}
	}
	corner := func(i int) bool { cx, cy := float64(i%2), float64(i/2); return math.Hypot(x-cx, y-cy) <= .5 }
	if n == 3 {
		for i := 0; i < 4; i++ {
			if m&(1<<i) == 0 {
				return !corner(i)
			}
		}
	}
	for i := 0; i < 4; i++ {
		if m&(1<<i) != 0 && corner(i) {
			return true
		}
	}
	return false
}
func main() {
	const s = 256
	im := image.NewNRGBA(image.Rect(0, 0, 4*s, 4*s))
	for m := 0; m < 16; m++ {
		for y := 0; y < s; y++ {
			for x := 0; x < s; x++ {
				c := color.NRGBA{119, 166, 74, 255}
				if dirt(m, (float64(x)+.5)/s, (float64(y)+.5)/s) {
					c = color.NRGBA{220, 184, 122, 255}
				}
				im.SetNRGBA(m%4*s+x, m/4*s+y, c)
			}
		}
	}
	f, e := os.Create("../../scripts/willowbrook/assets/dual-grid-guide.png")
	if e != nil {
		panic(e)
	}
	if e = png.Encode(f, im); e != nil {
		panic(e)
	}
	f.Close()
}
