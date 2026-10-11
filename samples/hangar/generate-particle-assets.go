//go:build ignore

// Generate the Hangar's original particle masks and opaque hot-coal texture.
// No external artwork or random input is used. Writes acid-sparkle.png,
// barrel-ember.png, barrel-flame-{1,2,3}.png and barrel-coals.png only.
// Run from the CLI repository root:
//
//	mise exec -- go run samples/hangar/generate-particle-assets.go
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const particleAssetDirectory = "samples/hangar/levels/hangar"

func main() {
	writeBarrelCoals()
	writeParticleMask("acid-sparkle", 96, func(x, y float64) float64 {
		// A soft core and short, asymmetric rays read as bright suspended droplets.
		core := math.Exp(-42 * (x*x + y*y))
		rays := math.Exp(-9*(x*x+y*y)) * math.Exp(-85*math.Min(x*x, y*y))
		return math.Min(1, core+0.28*rays)
	})
	writeParticleMask("barrel-ember", 64, func(x, y float64) float64 {
		return math.Exp(-48*x*x - 18*y*y)
	})
	for frame := 0; frame < 3; frame++ {
		phase := float64(frame) * 2 * math.Pi / 3
		writeParticleMask([]string{"barrel-flame-1", "barrel-flame-2", "barrel-flame-3"}[frame], 128, func(x, y float64) float64 {
			// Three tapering tongues merge at the base. Independent GPU particles
			// rotate, rise and select frames to keep the barrel's flame in motion.
			height := (0.87 - y) / 1.70
			if height <= 0 || height >= 1 {
				return 0
			}
			baseFade := math.Min(1, height*12)
			tipFade := math.Min(1, (1-height)*10)
			width := 0.34 * math.Pow(1-height, 0.72)
			centre := 0.10 * math.Sin(height*5+phase) * height
			main := math.Exp(-math.Pow((x-centre)/math.Max(width, 0.015), 2) * 2.5)
			left := flameTongue(x, height, -0.21, 0.70, phase+1)
			right := flameTongue(x, height, 0.21, 0.83, phase+3)
			return math.Min(1, (main+0.55*left+0.45*right)*baseFade*tipFade)
		})
	}
}

func flameTongue(x, height, origin, tip, phase float64) float64 {
	if height > tip {
		return 0
	}
	progress := height / tip
	width := 0.14 * math.Pow(1-progress, 0.7)
	centre := origin*(1-progress*0.7) + 0.09*math.Sin(progress*5+phase)*progress
	return math.Exp(-math.Pow((x-centre)/math.Max(width, 0.01), 2)*2.5) * math.Min(1, (1-progress)*9)
}

func writeParticleMask(name string, size int, coverage func(float64, float64) float64) {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			x := 2*(float64(px)+0.5)/float64(size) - 1
			y := 2*(float64(py)+0.5)/float64(size) - 1
			alpha := byte(math.Round(math.Min(1, math.Max(0, coverage(x, y))) * 255))
			// White RGB lets the GPU's color-over-life curve tint the mask.
			img.SetNRGBA(px, py, color.NRGBA{R: 255, G: 255, B: 255, A: alpha})
		}
	}
	writeParticleImage(name, img)
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
