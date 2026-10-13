//go:build ignore

// Generate restrained industrial surfaces without changing the original artwork.
// Run from the CLI root: mise exec -- go run scripts/hangar-material-assets.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	check := flag.Bool("check", false, "verify generated surfaces without writing")
	flag.Parse()
	for _, name := range []string{"floor", "ceiling", "grate", "ribbed", "hero-steel", "strip-light", "strip-light-broken"} {
		if err := generate(name, *check); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func generate(name string, check bool) error {
	const size = 256
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Integer hashing keeps the fine grain deterministic and low contrast.
			grain := int((uint32(x)*73856093^uint32(y)*19349663)%5) - 2
			r, g, b := 72, 82, 84
			switch name {
			case "floor":
				// Four broad plates per repeat; narrow seams and faint tread marks.
				if x%128 < 2 || y%128 < 2 {
					r, g, b = 52, 61, 63
				} else if x%128 == 2 || y%128 == 2 {
					r, g, b = 81, 90, 92
				} else if (x+y)%32 < 2 && x%32 > 7 && x%32 < 22 {
					r, g, b = 77, 87, 89
				}
			case "ceiling":
				r, g, b = 48, 58, 59
				if x%64 < 3 || y%64 < 3 {
					r, g, b = 63, 73, 74
				}
			case "grate":
				// Recesses remain legible without near-black checkerboard contrast.
				r, g, b = 51, 62, 63
				if x%32 < 5 || y%32 < 5 {
					r, g, b = 72, 83, 84
				} else if x%32 == 5 || y%32 == 5 {
					r, g, b = 61, 72, 73
				}
			case "ribbed":
				r, g, b = 65, 74, 75
				if x%16 < 3 {
					r, g, b = 49, 57, 58
				} else if x%16 < 5 {
					r, g, b = 74, 83, 84
				}
			case "hero-steel":
				// Bare brushed trim: quieter and lighter than ribbed painted panels.
				r, g, b = 141, 153, 157
				grain = (y*13+y*y%17)%5 - 2
			case "strip-light":
				// A nearly white opal diffuser, separate from the green reactor lens.
				r, g, b = 237, 235, 223
				grain = 0
			case "strip-light-broken":
				// Soot-darkened glass with a jagged crack and a branching fracture.
				r, g, b = 58, 62, 57
				crack := (y*3/4 + (y/13)%3*7 + 43) % size
				if abs(x-crack) < 3 || y > 88 && y < 184 && abs(x-(crack+(y-88)/2)) < 2 {
					r, g, b = 13, 17, 14
				}
			}
			out.SetNRGBA(x, y, color.NRGBA{uint8(r + grain), uint8(g + grain), uint8(b + grain), 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, out); err != nil {
		return err
	}
	filename := name + "-quiet.png"
	if name == "hero-steel" || strings.HasPrefix(name, "strip-light") {
		filename = name + ".png"
	}
	path := filepath.Join("samples/hangar/levels/hangar", filename)
	if check {
		current, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, encoded.Bytes()) {
			return fmt.Errorf("%s is stale; regenerate Hangar materials", path)
		}
		return nil
	}
	return os.WriteFile(path, encoded.Bytes(), 0644)
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
