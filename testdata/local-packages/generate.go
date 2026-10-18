//go:build ignore

// Generate original shared-kit textures and game-owned override textures.
// Run from the CLI root: mise exec -- go run ./testdata/local-packages/generate.go
package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	writeTexture(
		"testdata/local-packages/library/assets/checker.png",
		color.NRGBA{R: 185, G: 185, B: 185, A: 255},
		color.NRGBA{R: 140, G: 140, B: 140, A: 255},
	)
	writeTexture(
		"testdata/local-packages/library/assets/wood.png",
		color.NRGBA{R: 180, G: 111, B: 55, A: 255},
		color.NRGBA{R: 132, G: 75, B: 35, A: 255},
	)
	writeTexture(
		"testdata/local-packages/library/assets/frame.png",
		color.NRGBA{R: 62, G: 68, B: 78, A: 255},
		color.NRGBA{R: 45, G: 48, B: 56, A: 255},
	)
	writeTexture(
		"testdata/local-packages/games/walk/authoring/room/accent.png",
		color.NRGBA{R: 44, G: 177, B: 193, A: 255},
		color.NRGBA{R: 30, G: 115, B: 131, A: 255},
	)
	writeTexture(
		"testdata/local-packages/games/precision/authoring/room/accent.png",
		color.NRGBA{R: 194, G: 91, B: 53, A: 255},
		color.NRGBA{R: 131, G: 53, B: 33, A: 255},
	)
}

func writeTexture(filename string, light, dark color.NRGBA) {
	texture := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			shade := dark
			if (x/8+y/8)%2 == 0 {
				shade = light
			}
			texture.SetNRGBA(x, y, shade)
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, texture); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filename, encoded.Bytes(), 0600); err != nil {
		panic(err)
	}
}
