//go:build ignore

// Generate localized soot and service grime for Hangar's stationary equipment.
// These are floor residue, not cast shadows; moving freight uses the fixed
// lighting renderer's supported receiver-projected contact footprints.
// Run from the CLI root: mise exec -- go run scripts/hangar-contact-assets.go
// Add -check to verify checked-in pixels without writing.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	check := flag.Bool("check", false, "verify authored contact-wear assets without writing")
	flag.Parse()
	for _, recipe := range []struct {
		name string
		soot bool
	}{
		{"barrel-floor-soot", true},
		{"machine-base-grime", false},
	} {
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, contactWear(recipe.soot)); err != nil {
			panic(err)
		}
		path := filepath.Join("samples/hangar/levels/hangar", recipe.name+".png")
		if *check {
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, encoded.Bytes()) {
				fmt.Fprintf(os.Stderr, "%s is stale; run the Hangar contact asset generator\n", path)
				os.Exit(1)
			}
			continue
		}
		if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
			panic(err)
		}
	}
}

func contactWear(soot bool) image.Image {
	const size = 128
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			u, v := (float64(x)+.5-size/2)/(size/2), (float64(y)+.5-size/2)/(size/2)
			radius, angle := math.Hypot(u, v), math.Atan2(v, u)
			// A feathered irregular outline leaves the original floor texture and
			// seams visible. Fine variation represents settled soot/oily dirt;
			// there is no painted directional highlight or detached shadow edge.
			edge := .92 + .025*math.Sin(7*angle) + .018*math.Cos(11*angle)
			coverage := max(0, min(1, (edge-radius)/.35))
			coverage = coverage * coverage * (3 - 2*coverage)
			grain := float64(contactNoise(uint32(x), uint32(y))&255) / 255
			alpha := coverage * (.68 + .32*grain)
			c := color.NRGBA{R: 28, G: 30, B: 24, A: uint8(math.Round(60 * alpha))}
			if soot {
				c = color.NRGBA{R: 26, G: 24, B: 21, A: uint8(math.Round(78 * alpha))}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func contactNoise(x, y uint32) uint32 {
	n := x*374761393 + y*668265263 + 0x61c88647
	n = (n ^ (n >> 13)) * 1274126177
	return n ^ (n >> 16)
}
