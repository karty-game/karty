//go:build ignore

// Command author-paints writes the small, deterministic lighthouse paint palette.
// Run from the CLI root; source images remain separate from derived materials.
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	paints := []struct {
		name  string
		base  [3]int
		grain int
	}{
		{"lighthouse-white", [3]int{222, 218, 201}, 7},
		{"lighthouse-red", [3]int{155, 57, 40}, 5},
		{"lighthouse-metal", [3]int{35, 44, 46}, 3},
		{"lighthouse-lamp", [3]int{255, 221, 146}, 0},
	}
	for _, paint := range paints {
		img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				grain := 0
				if paint.grain > 0 {
					grain = ((x*73856093^y*19349663)&255)%(paint.grain*2+1) - paint.grain
				}
				rgb := func(c int) uint8 { return uint8(max(0, min(255, c+grain))) }
				img.SetNRGBA(x, y, color.NRGBA{R: rgb(paint.base[0]), G: rgb(paint.base[1]), B: rgb(paint.base[2]), A: 255})
			}
		}
		out, err := os.Create(filepath.Join("samples", "terrain", "levels", "ground", paint.name+".png"))
		if err != nil {
			panic(err)
		}
		err = png.Encode(out, img)
		closeErr := out.Close()
		if err != nil {
			panic(err)
		}
		if closeErr != nil {
			panic(closeErr)
		}
	}
}
