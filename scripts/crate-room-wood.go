//go:build ignore

// Original seamless crate oak. Run from the CLI root; no source package edits.
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

const woodSize = 256

func main() {
	check := flag.Bool("check", false, "verify generated wood without writing")
	flag.Parse()
	for _, kind := range []string{"panel", "lid", "frame"} {
		pixels := image.NewNRGBA(image.Rect(0, 0, woodSize, woodSize))
		for y := range woodSize {
			for x := range woodSize {
				pixels.SetNRGBA(x, y, oakPixel(x, y, kind))
			}
		}
		var output bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&output, pixels); err != nil {
			panic(err)
		}
		path := filepath.Join("samples/crate-room/levels/room/wood", "oak-"+kind+".png")
		if *check {
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, output.Bytes()) {
				panic("stale wood: " + path)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				panic(err)
			}
			if err := os.WriteFile(path, output.Bytes(), 0644); err != nil {
				panic(err)
			}
		}
		fmt.Println(path)
	}
}

// Luminance carries growth rings, open grain and joints, without painted light
// or prop frames. Materialize derives normal/height/AO at build time.
func oakPixel(x, y int, kind string) color.NRGBA {
	if kind == "lid" {
		x, y = y, x
	}
	u, v := float64(x)/woodSize, float64(y)/woodSize
	board := y / 64
	warp := 2.8*math.Sin(2*math.Pi*u) + 1.5*math.Sin(6*math.Pi*u+.9*float64(board))
	// A long oval knot bends neighboring grain; its center stays away from seams.
	cx, cy := .28+float64(board%2)*.39, (float64(board)+.5)/4
	dx, dy := (u-cx)/.17, (v-cy)/.055
	radius := math.Sqrt(dx*dx + dy*dy)
	knot := math.Exp(-radius * radius * .8)
	warp += knot * 5 * math.Sin(math.Atan2(dy, dx))
	grain := math.Sin(v*2*math.Pi*29 + warp)
	fiber := math.Sin(v*2*math.Pi*83 + warp*2 + math.Sin(u*2*math.Pi*3))
	variation := math.Sin(2*math.Pi*u*3+float64(board)) * math.Sin(2*math.Pi*v*7)
	tone := 11*grain + 4*fiber + 7*variation + float64([]int{9, -12, 2, -6}[board])
	tone -= 34 * knot * (.5 + .5*math.Cos(radius*15))
	joint := min(y%64, 63-y%64)
	if kind != "frame" && joint < 2 {
		tone -= float64(2-joint) * 23
	}
	// Rare longitudinal checks and weathering add readable medium-scale detail.
	check := math.Abs(math.Sin(v*2*math.Pi*13 + .35*math.Sin(u*2*math.Pi*2)))
	if check < .055 && math.Sin(u*2*math.Pi*3+float64(board)) > .35 {
		tone -= 24
	}
	base := [3]float64{162, 126, 77}
	if kind == "frame" {
		base = [3]float64{120, 88, 51}
		tone *= .8
	}
	channel := func(value float64) uint8 { return uint8(max(0, min(255, math.Round(value+tone)))) }
	return color.NRGBA{R: channel(base[0]), G: channel(base[1]), B: channel(base[2]), A: 255}
}
