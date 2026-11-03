//go:build ignore

// Author Hangar's quiet acid and bold wayfinding signs without external fonts.
// Run from the CLI root: mise exec -- go run scripts/hangar-wayfinding-assets.go
// Pass -check to verify byte-for-byte reproducibility without writing assets.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	check := flag.Bool("check", false, "verify generated acid and signs without writing")
	flag.Parse()
	assets := []struct {
		name string
		img  image.Image
	}{
		{"toxic-flow.png", acidFlow()},
		{"arrival-sign.png", wayfindingSign("arrival", 256)},
		{"reactor-sign.png", wayfindingSign("reactor", 256)},
		{"exit-sign.png", wayfindingSign("exit", 320)},
	}
	for _, asset := range assets {
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, asset.img); err != nil {
			panic(err)
		}
		path := filepath.Join("samples/hangar/levels/hangar", asset.name)
		if *check {
			current, err := os.ReadFile(path)
			if err != nil {
				panic(err)
			}
			if !bytes.Equal(current, encoded.Bytes()) {
				fmt.Fprintf(os.Stderr, "%s is stale; regenerate Hangar wayfinding assets\n", path)
				os.Exit(1)
			}
		} else if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
			panic(err)
		}
	}
}

// Periodic low-frequency fields keep the tile edges continuous. The base has
// no bubble circles: moving particles own those small, transient surface events.
// Original toxic.png is retained; this recipe deliberately lowers local contrast.
func acidFlow() *image.NRGBA {
	const size = 256
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			u, v := float64(x)/size, float64(y)/size
			warp := periodicNoise(u, v, 3, 3, 91) - 0.5
			a, b := u+0.19*warp, v+0.12*math.Sin(2*math.Pi*u)
			broad := 0.72*periodicNoise(a, b, 3, 2, 17) + 0.28*periodicNoise(a, b, 5, 4, 43)
			wisp := periodicNoise(a, b, 7, 11, 127)
			// Fine detail stays subordinate to the broad liquid variation.
			grain := float64((uint32(x)*73856093^uint32(y)*19349663)%3) - 1
			out.SetNRGBA(x, y, color.NRGBA{
				uint8(math.Round(24 + 40*broad + 8*wisp + grain)),
				uint8(math.Round(101 + 50*broad + 12*wisp + grain)),
				uint8(math.Round(7 + 12*broad + 3*wisp)), 255,
			})
		}
	}
	return out
}

func periodicNoise(u, v float64, columns, rows int, seed uint32) float64 {
	x, y := u*float64(columns), v*float64(rows)
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-math.Floor(x), y-math.Floor(y)
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	value := func(x, y int) float64 {
		x, y = (x%columns+columns)%columns, (y%rows+rows)%rows
		hash := uint32(x)*374761393 + uint32(y)*668265263 + seed*2246822519
		hash = (hash ^ hash>>13) * 1274126177
		return float64((hash^hash>>16)&65535) / 65535
	}
	a := value(x0, y0)*(1-fx) + value(x0+1, y0)*fx
	b := value(x0, y0+1)*(1-fx) + value(x0+1, y0+1)*fx
	return a*(1-fy) + b*fy
}

func wayfindingSign(kind string, width int) *image.NRGBA {
	const height = 160
	out := image.NewNRGBA(image.Rect(0, 0, width, height))
	background := color.NRGBA{18, 29, 25, 255}
	border := color.NRGBA{85, 95, 76, 255}
	letter := color.NRGBA{245, 235, 192, 255}
	muted := color.NRGBA{181, 176, 135, 255}
	fill(out, out.Bounds(), background)
	// Narrow frame, sparse labels, and broad solid strokes survive minification.
	for _, edge := range []image.Rectangle{
		image.Rect(4, 4, width-4, 7), image.Rect(4, height-7, width-4, height-4),
		image.Rect(4, 4, 7, height-4), image.Rect(width-7, 4, width-4, height-4),
	} {
		fill(out, edge, border)
	}
	switch kind {
	case "arrival":
		text(out, "HANGAR 01", 16, 17, 2, muted)
		text(out, "REACTOR", 16, 53, 4, letter)
		text(out, "SERVICE", 16, 109, 4, letter)
		arrow(out, width-42, 53, false, letter)
		arrow(out, width-42, 109, true, letter)
	case "reactor":
		centerText(out, "CAUTION", 16, 2, muted)
		centerText(out, "POWER", 49, 5, letter)
		centerText(out, "CORE", 96, 5, letter)
		centerText(out, "HIGH VOLTAGE", 139, 1, muted)
	case "exit":
		centerText(out, "SECTOR 06", 17, 2, muted)
		centerText(out, "EXTRACTION", 58, 5, letter)
		centerText(out, "STAND BY", 125, 2, muted)
	default:
		panic("unknown wayfinding sign: " + kind)
	}
	return out
}

func fill(out draw.Image, rect image.Rectangle, ink color.NRGBA) {
	draw.Draw(out, rect, image.NewUniform(ink), image.Point{}, draw.Src)
}

func centerText(out *image.NRGBA, label string, y, scale int, ink color.NRGBA) {
	text(out, label, (out.Bounds().Dx()-(len(label)*6-1)*scale)/2, y, scale, ink)
}

func text(out *image.NRGBA, label string, x, y, scale int, ink color.NRGBA) {
	for _, ch := range label {
		if ch != ' ' {
			glyph, exists := signGlyphs[ch]
			if !exists {
				panic(fmt.Sprintf("missing sign glyph %q", ch))
			}
			for row, bits := range glyph {
				for col := range 5 {
					if bits&(1<<(4-col)) != 0 {
						fill(out, image.Rect(x+col*scale, y+row*scale, x+(col+1)*scale, y+(row+1)*scale), ink)
					}
				}
			}
		}
		x += 6 * scale
	}
}

func arrow(out *image.NRGBA, x, y int, right bool, ink color.NRGBA) {
	// A filled 28px arrow replaces the original one-pixel chevrons.
	for row := range 28 {
		for col := range 28 {
			u, v := col, row
			if right {
				u, v = row, 27-col
			}
			if v < 14 && absInt(u-14) <= v || v >= 12 && u >= 9 && u <= 18 {
				out.SetNRGBA(x+col, y+row, ink)
			}
		}
	}
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Original compact block alphabet; no system-font or network dependency.
var signGlyphs = map[rune][7]uint8{
	'A': {14, 17, 17, 31, 17, 17, 17}, 'B': {30, 17, 17, 30, 17, 17, 30},
	'C': {15, 16, 16, 16, 16, 16, 15}, 'D': {30, 17, 17, 17, 17, 17, 30},
	'E': {31, 16, 16, 30, 16, 16, 31}, 'G': {15, 16, 16, 23, 17, 17, 15},
	'H': {17, 17, 17, 31, 17, 17, 17}, 'I': {31, 4, 4, 4, 4, 4, 31},
	'L': {16, 16, 16, 16, 16, 16, 31}, 'N': {17, 25, 25, 21, 19, 19, 17},
	'O': {14, 17, 17, 17, 17, 17, 14}, 'P': {30, 17, 17, 30, 16, 16, 16},
	'R': {30, 17, 17, 30, 20, 18, 17}, 'S': {15, 16, 16, 14, 1, 1, 30},
	'T': {31, 4, 4, 4, 4, 4, 4}, 'U': {17, 17, 17, 17, 17, 17, 14},
	'V': {17, 17, 17, 17, 17, 10, 4}, 'W': {17, 17, 17, 21, 21, 27, 17},
	'X': {17, 17, 10, 4, 10, 17, 17}, 'Y': {17, 17, 10, 4, 4, 4, 4},
	'0': {14, 17, 19, 21, 25, 17, 14}, '1': {4, 12, 4, 4, 4, 4, 14},
	'6': {14, 16, 16, 30, 17, 17, 14},
}
