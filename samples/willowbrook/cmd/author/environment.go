package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// These derived frames retain the painted source pixels. Animation changes
// the surface glints and foliage; roots, trunks and all structural art stay put.
func prepareEnvironment() {
	// Preserve the authored paired-bank channel and seamless tile boundaries.
	// This square source includes both shallow bank faces around transparent water.
	save("riverbanks", resize(load("../../scripts/willowbrook/assets/riverbanks-v2.png"), 192, 192))
	for _, a := range []struct {
		name, source string
		width        int
	}{
		{"waterfall", "../../scripts/willowbrook/assets/waterfall-v3-outlet.png", 320},
		{"garden-plot", "../../scripts/willowbrook/assets/garden-plot-v1.png", 160},
	} {
		source := load(a.source)
		bounds := alphaBounds(source)
		if a.name == "waterfall" {
			// Keep the generated outlet's translucent fade, including pixels
			// below half opacity, so it can join the shared river surface.
			bounds = alphaBoundsAt(source, 1)
		}
		save(a.name, resize(crop(source, bounds), a.width, int(math.Round(float64(bounds.Dy())*float64(a.width)/float64(bounds.Dx())))))
	}
	bed := load("../../scripts/willowbrook/assets/riverbed-v2.png")
	save("riverbed", resize(bed, 128, 128))
	grass := load("../../scripts/willowbrook/assets/grass-tuft-v1.png")
	b := alphaBounds(grass)
	tuft := resize(crop(grass, b), 64, int(math.Round(float64(b.Dy())*64/float64(b.Dx()))))
	for y := range tuft.Bounds().Dy() {
		for x := range tuft.Bounds().Dx() {
			c := tuft.NRGBAAt(x, y)
			if c.A < 128 {
				c = color.NRGBA{}
			} else {
				c.A = 255
			}
			tuft.SetNRGBA(x, y, c)
		}
	}
	save("grass-tuft", tuft)
	water := textures["water"]
	for frame, phase := range []int{0, 2, 4, 2} {
		out := image.NewNRGBA(water.Bounds())
		for y := range out.Bounds().Dy() {
			for x := range out.Bounds().Dx() {
				// Quantize the painted glints into deliberate four-pixel
				// clusters, over a stable turquoise tint. Fine source gravel
				// and white speckle must not dominate the composed stream.
				sx := (mirroredWaterCoordinate(x) / 4) * 8
				sy := ((mirroredWaterCoordinate(y)/4)*8 + phase) % 128
				painted := water.NRGBAAt(sx, sy)
				c := color.NRGBA{16, 165, 221, 124}
				if painted.R > 140 {
					c = color.NRGBA{130, 225, 234, 132}
				}
				out.SetNRGBA(x, y, c)
			}
		}
		for x := range out.Bounds().Dx() {
			out.SetNRGBA(x, out.Bounds().Dy()-1, out.NRGBAAt(x, 0))
		}
		save(fmt.Sprintf("water-surface-%02d", frame), out)
	}
	// The opaque source sample is a source for the translucent frames only.
	delete(textures, "water")
	tree := textures["tree"]
	for frame, shift := range []float64{0, 1, 0, -1} {
		save(fmt.Sprintf("tree-breeze-%02d", frame), foliageSway(tree, shift, 142))
	}
	flowers := textures["flowers"]
	for frame, shift := range []float64{0, 1, -1} {
		save(fmt.Sprintf("flower-breeze-%02d", frame), foliageSway(flowers, shift, 24))
	}
	shade := image.NewNRGBA(image.Rect(0, 0, 128, 64))
	for y := range 64 {
		for x := range 128 {
			dx, dy := (float64(x)+.5-64)/64, (float64(y)+.5-32)/32
			r := dx*dx + dy*dy
			a := 0.0
			if r < 1 {
				a = 62 * math.Pow(1-r, 1.4)
			}
			if a >= 1 {
				shade.SetNRGBA(x, y, color.NRGBA{27, 46, 39, uint8(a)})
			}
		}
	}
	save("shadow-soft", shade)
}

// Row shifts taper to exactly zero above the lower trunk/root rows. Every
// frame keeps the original canvas and every lower-row source pixel unchanged.
func foliageSway(source *image.NRGBA, shift float64, fixedFrom int) *image.NRGBA {
	out := image.NewNRGBA(source.Bounds())
	for y := range source.Bounds().Dy() {
		amount := 0
		if y < fixedFrom {
			amount = int(math.Round(shift * math.Min(1, float64(fixedFrom-y)/32)))
		}
		for x := range source.Bounds().Dx() {
			sx := x - amount
			if sx >= 0 && sx < source.Bounds().Dx() {
				out.SetNRGBA(x, y, source.NRGBAAt(sx, y))
			}
		}
	}
	return out
}

// Mirroring the painted half-tile makes a cyclic surface with continuous
// boundaries. Every animation phase wraps this same periodic source.
func mirroredWaterCoordinate(v int) int {
	v %= 128
	if v < 0 {
		v += 128
	}
	if v >= 64 {
		return 127 - v
	}
	return v
}
