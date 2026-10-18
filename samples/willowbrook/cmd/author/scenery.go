package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// Explicit source rectangles isolate irregularly packed assets. This source is
// the approved frontal cottage artwork; it is not an equal-cell sprite sheet.
func prepareScenery() {
	prepareIntactLandmarks()
	atlas := load("../../scripts/willowbrook/assets/village-extracted.png")
	for _, a := range []struct {
		name  string
		box   image.Rectangle
		width int
	}{
		{"cottage", image.Rect(30, 24, 475, 465), 256},
		{"tree", image.Rect(483, 70, 825, 463), 192},
		{"bush", image.Rect(828, 220, 1105, 465), 128},
		{"rocks", image.Rect(1114, 220, 1430, 473), 96},
		{"cliff", image.Rect(14, 530, 606, 748), 256},
		{"fence", image.Rect(624, 550, 940, 750), 192},
		{"lantern", image.Rect(960, 485, 1115, 755), 128},
		{"clutter", image.Rect(1129, 580, 1448, 747), 128},
	} {
		cell := crop(atlas, a.box)
		b := alphaBounds(cell)
		out := resize(crop(cell, b), a.width, max(1, b.Dy()*a.width/b.Dx()))
		// Alpha-cutout sources retain sharp pixels without carrying low-alpha guide
		// fringes into ordinary 2D sprite blending.
		for y := range out.Bounds().Dy() {
			for x := range out.Bounds().Dx() {
				c := out.NRGBAAt(x, y)
				if c.A < 128 {
					c = color.NRGBA{}
				} else {
					c.A = 255
				}
				out.SetNRGBA(x, y, c)
			}
		}
		if a.name == "tree" {
			keepLargestComponent(out)
		}
		save(a.name, out)
	}
	// Source wave pixels feed translucent animation; no opaque surface is bundled.
	textures["water"] = resize(crop(atlas, image.Rect(90, 780, 330, 1016)), 128, 128)
	closed := crop(atlas, image.Rect(825, 838, 1024, 1024))
	closed = crop(closed, alphaBounds(closed))
	open := crop(atlas, image.Rect(1158, 828, 1367, 1024))
	open = crop(open, alphaBounds(open))
	scale := math.Min(90/float64(max(closed.Bounds().Dx(), open.Bounds().Dx())), 90/float64(max(closed.Bounds().Dy(), open.Bounds().Dy())))
	for _, a := range []struct {
		name string
		im   *image.NRGBA
	}{{"treasure-closed", closed}, {"treasure-open", open}} {
		im := resize(a.im, max(1, int(float64(a.im.Bounds().Dx())*scale)), max(1, int(float64(a.im.Bounds().Dy())*scale)))
		out := image.NewNRGBA(image.Rect(0, 0, 96, 96))
		at := image.Pt((96-im.Bounds().Dx())/2, 94-im.Bounds().Dy())
		draw.Draw(out, im.Bounds().Add(at), im, image.Point{}, draw.Src)
		for y := range 96 {
			for x := range 96 {
				c := out.NRGBAAt(x, y)
				if c.A < 128 {
					c = color.NRGBA{}
				} else {
					c.A = 255
				}
				out.SetNRGBA(x, y, c)
			}
		}
		save(a.name, out)
	}
	source := load("../../scripts/willowbrook/assets/notice-board-v1.png")
	bounds := alphaBounds(source)
	notice := resize(crop(source, bounds), int(math.Round(float64(bounds.Dx())*64/float64(bounds.Dy()))), 64)
	canvas := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	at := image.Pt((64-notice.Bounds().Dx())/2, 0)
	draw.Draw(canvas, notice.Bounds().Add(at), notice, image.Point{}, draw.Src)
	save("clue-note", canvas)
}

// Retain the connected subject, excluding neighbouring fragments or detached
// shadows before measuring its bounds or normalizing its footing.
func keepLargestComponent(im *image.NRGBA) {
	w, h := im.Bounds().Dx(), im.Bounds().Dy()
	seen := make([]bool, w*h)
	var largest []int
	for start := range w * h {
		if seen[start] || im.NRGBAAt(start%w, start/w).A < 128 {
			continue
		}
		queue := []int{start}
		seen[start] = true
		for i := 0; i < len(queue); i++ {
			p := queue[i]
			x, y := p%w, p/w
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					xx, yy := x+dx, y+dy
					if xx < 0 || yy < 0 || xx >= w || yy >= h {
						continue
					}
					n := yy*w + xx
					if !seen[n] && im.NRGBAAt(xx, yy).A >= 128 {
						seen[n] = true
						queue = append(queue, n)
					}
				}
			}
		}
		if len(queue) > len(largest) {
			largest = queue
		}
	}
	keep := make([]bool, w*h)
	for _, p := range largest {
		keep[p] = true
	}
	for p := range w * h {
		if !keep[p] {
			im.SetNRGBA(p%w, p/w, color.NRGBA{})
		}
	}
}

// Each landmark is one intact source image, uniformly scaled. Its front masks
// are copied later from the same pixels using editable Tiled shape metadata.
func prepareIntactLandmarks() {
	for _, a := range []struct {
		name, source string
		width        int
	}{{"staircase-intact", "../../scripts/willowbrook/assets/staircase-intact-v2.png", 320}, {"bridge-intact", "../../scripts/willowbrook/assets/bridge-intact-v2.png", 384}} {
		source := load(a.source)
		bounds := alphaBounds(source)
		out := resize(crop(source, bounds), a.width, int(math.Round(float64(bounds.Dy())*float64(a.width)/float64(bounds.Dx()))))
		for y := range out.Bounds().Dy() {
			for x := range out.Bounds().Dx() {
				c := out.NRGBAAt(x, y)
				if c.A < 128 {
					c = color.NRGBA{}
				} else {
					c.A = 255
				}
				out.SetNRGBA(x, y, c)
			}
		}
		save(a.name, out)
	}
}
