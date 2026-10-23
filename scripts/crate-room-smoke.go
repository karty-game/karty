//go:build ignore

// Prepare deterministic opacity frames from the approved smoke artwork.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

const smokeSide, smokeFrames = 256, 16

func main() {
	check := flag.Bool("check", false, "verify smoke opacity frames without writing")
	flag.Parse()
	root := "samples/crate-room/levels/room/effects"
	input, err := os.Open(filepath.Join(root, "smoke-poof.png"))
	if err != nil {
		panic(err)
	}
	source, err := png.Decode(input)
	if err != nil {
		panic(err)
	}
	if err := input.Close(); err != nil {
		panic(err)
	}
	base := image.NewNRGBA(image.Rect(0, 0, smokeSide, smokeSide))
	draw.CatmullRom.Scale(base, base.Bounds(), source, source.Bounds(), draw.Src, nil)
	for frame := range smokeFrames {
		pixels := image.NewNRGBA(base.Bounds())
		copy(pixels.Pix, base.Pix)
		// A cosine fade preserves the initial puff and eases down to near zero.
		opacity := math.Pow(math.Cos(float64(frame)/smokeFrames*math.Pi/2), 2)
		for offset := 3; offset < len(pixels.Pix); offset += 4 {
			pixels.Pix[offset] = uint8(math.Round(float64(pixels.Pix[offset]) * opacity))
		}
		var output bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&output, pixels); err != nil {
			panic(err)
		}
		path := filepath.Join(root, "fade", fmt.Sprintf("smoke-%02d.png", frame))
		if *check {
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, output.Bytes()) {
				panic("stale smoke opacity frame: " + path)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				panic(err)
			}
			if err := os.WriteFile(path, output.Bytes(), 0644); err != nil {
				panic(err)
			}
		}
	}
	fmt.Printf("%d smoke opacity frames (%d×%d) verified/generated\n", smokeFrames, smokeSide, smokeSide)
}
