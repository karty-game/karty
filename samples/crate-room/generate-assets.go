//go:build ignore

// Generates the sample's procedural material PNGs; no engine source is needed.
// Concrete panels, wooden planks/battens and hazard-painted steel banks/bumpers
// are classified in levels/room/level.toml; see docs/sample-materials.md.
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	for i, name := range []string{"floor", "ceiling", "wall", "wood", "frame", "ramp", "bumper"} {
		pixels := image.NewNRGBA(image.Rect(0, 0, 64, 64))
		for y := range 64 {
			for x := range 64 {
				pixels.SetNRGBA(x, y, materialPixel(uint32(i+1), x, y))
			}
		}
		file, err := os.Create(filepath.Join("samples/crate-room/levels/room", name+".png"))
		if err != nil {
			panic(err)
		}
		if err := png.Encode(file, pixels); err != nil {
			panic(err)
		}
		if err := file.Close(); err != nil {
			panic(err)
		}
	}
}
func materialPixel(material uint32, x, y int) color.NRGBA {
	grain := int(5*math.Sin(float64(y)*1.3+2*math.Sin(float64(x)*.2))) + ((x*13 + y*7) % 7)
	base := [3]int{65, 73, 81}
	switch material {
	case 1:
		base = [3]int{75, 83, 91}
		if x < 2 || y < 2 {
			base = [3]int{28, 34, 40}
		}
	case 2:
		base = [3]int{38, 47, 59}
	case 3:
		base = [3]int{85, 94, 104}
		if y%16 < 2 {
			base = [3]int{45, 54, 65}
		}
	case 4:
		base = [3]int{153, 103, 53}
		if y%16 < 2 {
			base = [3]int{78, 49, 24}
		}
	case 5:
		base = [3]int{104, 66, 33}
	case 6:
		base = [3]int{170, 133, 64}
		if (x+y)%32 < 12 {
			base = [3]int{51, 58, 66}
		}
	case 7:
		base = [3]int{178, 63, 47}
		if x%32 < 3 || y%32 < 3 {
			base = [3]int{221, 175, 75}
		}
	}
	return color.NRGBA{R: uint8(base[0] + grain), G: uint8(base[1] + grain), B: uint8(base[2] + grain), A: 255}
}
