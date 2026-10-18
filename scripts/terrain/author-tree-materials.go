//go:build ignore

// Offline normal-map preparation for the retained original tree atlas artwork.
// Run from the CLI root with mise run author-terrain-tree-materials.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	root := "samples/terrain/levels/ground"
	for _, name := range []string{"palm", "broadleaf"} {
		if err := prepare(root, name); err != nil {
			panic(err)
		}
	}
}

func prepare(root, name string) error {
	input, err := os.Open(filepath.Join(root, name+"-atlas-v1.png"))
	if err != nil {
		return err
	}
	albedo, err := png.Decode(input)
	input.Close()
	if err != nil {
		return err
	}
	bounds := albedo.Bounds()
	normals := image.NewNRGBA(bounds)
	// Low-frequency relief from the diffuse source is an artistic estimate,
	// rather than measured geometry. Alpha edges never become relief ridges.
	luma := func(x, y int) float64 {
		x, y = max(bounds.Min.X, min(bounds.Max.X-1, x)), max(bounds.Min.Y, min(bounds.Max.Y-1, y))
		c := color.NRGBAModel.Convert(albedo.At(x, y)).(color.NRGBA)
		if c.A < 128 {
			return .4
		}
		return (float64(c.R)*.2126 + float64(c.G)*.7152 + float64(c.B)*.0722) / 255
	}
	encode := func(v float64) uint8 { return uint8(math.Round((v*.5 + .5) * 255)) }
	step := max(1, bounds.Dx()/256)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			n := color.NRGBA{128, 128, 255, 255}
			_, _, _, alpha := albedo.At(x, y).RGBA()
			if alpha >= 0x8000 {
				nx := (luma(x-step, y) - luma(x+step, y)) * .7
				ny := (luma(x, y-step) - luma(x, y+step)) * .7
				inverse := 1 / math.Sqrt(1+nx*nx+ny*ny)
				n = color.NRGBA{encode(nx * inverse), encode(ny * inverse), encode(inverse), 255}
			}
			normals.SetNRGBA(x, y, n)
		}
	}
	output, err := os.Create(filepath.Join(root, name+"-normal-v1.png"))
	if err != nil {
		return err
	}
	if err := png.Encode(output, normals); err != nil {
		output.Close()
		return err
	}
	fmt.Printf("Prepared %s estimated normals, %d×%d; opaque tangent-space data\n", name, bounds.Dx(), bounds.Dy())
	return output.Close()
}
