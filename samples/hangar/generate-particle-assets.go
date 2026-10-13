//go:build ignore

// Generate the Hangar's opaque hot-coal texture. No external artwork or random
// input is used. Writes barrel-coals.png only; particle assets come from the
// reusable Particle Lab recipes through generate-effects.go.
// Run from the CLI repository root:
//
//	mise exec -- go run samples/hangar/generate-particle-assets.go
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

const particleAssetDirectory = "samples/hangar/levels/hangar"

func main() {
	writeBarrelCoals()
}

func writeBarrelCoals() {
	const size = 128
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			grain := (x*17 + y*31 + (x/9)*(y/7)*13) % 19
			c := color.NRGBA{R: byte(19 + grain), G: byte(14 + grain/2), B: byte(11 + grain/3), A: 255}
			// Irregular orange fissures between the dark coal fragments.
			if (x+y/3+(y/17)*5)%29 < 2 || (y-x/4+(x/19)*3+size)%37 < 2 {
				c = color.NRGBA{R: byte(125 + grain*3), G: byte(24 + grain), B: 3, A: 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	writeParticleImage("barrel-coals", img)
}

func writeParticleImage(name string, img image.Image) {
	file, err := os.Create(filepath.Join(particleAssetDirectory, name+".png"))
	if err != nil {
		panic(err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		panic(err)
	}
	if err := file.Close(); err != nil {
		panic(err)
	}
}
