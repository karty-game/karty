package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func generatedImage(t *testing.T, name string) image.Image {
	t.Helper()
	f, e := os.Open(filepath.Join("../..", target, name+".png"))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	im, e := png.Decode(f)
	if e != nil {
		t.Fatal(e)
	}
	return im
}
func TestGeneratedFoliageFramesKeepTrunksAndRootsFixed(t *testing.T) {
	for _, a := range []struct {
		source, prefix    string
		frames, fixedFrom int
	}{{"tree", "tree-breeze", 4, 142}, {"flowers", "flower-breeze", 3, 24}} {
		source := generatedImage(t, a.source)
		changed := 0
		for i := range a.frames {
			frame := generatedImage(t, fmt.Sprintf("%s-%02d", a.prefix, i))
			if frame.Bounds() != source.Bounds() {
				t.Fatal("foliage animation changed its canvas")
			}
			for y := range source.Bounds().Dy() {
				for x := range source.Bounds().Dx() {
					if frame.At(x, y) != source.At(x, y) {
						if y >= a.fixedFrom {
							t.Fatalf("%s moved a root or trunk pixel at%d,%d", a.prefix, x, y)
						}
						changed++
					}
				}
			}
		}
		if changed == 0 {
			t.Fatalf("%s frames never animate", a.prefix)
		}
	}
}
func TestRiverSurfaceRevealsBedAndMatchesTileEdges(t *testing.T) {
	bed := generatedImage(t, "riverbed")
	var first image.Image
	changed := 0
	for i := range 4 {
		frame := generatedImage(t, fmt.Sprintf("water-surface-%02d", i))
		if frame.Bounds() != bed.Bounds() {
			t.Fatal("river surface and bed dimensions differ")
		}
		for y := range 128 {
			for x := range 128 {
				_, _, _, a := frame.At(x, y).RGBA()
				_, _, _, ba := bed.At(x, y).RGBA()
				if a == 0 || a > 153*257 || ba != 65535 {
					t.Fatal("river bed is not visible beneath a translucent surface")
				}
				if i > 0 && frame.At(x, y) != first.At(x, y) {
					changed++
				}
			}
			if frame.At(0, y) != frame.At(127, y) {
				t.Fatal("horizontal surface tile seam")
			}
		}
		for x := range 128 {
			if frame.At(x, 0) != frame.At(x, 127) {
				t.Fatal("vertical surface tile seam")
			}
		}
		if i == 0 {
			first = frame
		}
	}
	if changed < 100 {
		t.Fatal("river surface frames are static")
	}
}
func TestSoftShadowHasTransparentCornersAndSoftCenter(t *testing.T) {
	im := generatedImage(t, "shadow-soft")
	for _, p := range []image.Point{{0, 0}, {127, 0}, {0, 63}, {127, 63}} {
		_, _, _, a := im.At(p.X, p.Y).RGBA()
		if a != 0 {
			t.Fatal("shadow has a rectangular edge")
		}
	}
	_, _, _, a := im.At(64, 32).RGBA()
	if a < 30*257 || a > 80*257 {
		t.Fatal("shadow center is missing or opaque")
	}
}

func TestWaterAnimationReturnsThroughTheSameMiddlePhase(t *testing.T) {
	a, b := generatedImage(t, "water-surface-01"), generatedImage(t, "water-surface-03")
	for y := range a.Bounds().Dy() {
		for x := range a.Bounds().Dx() {
			if a.At(x, y) != b.At(x, y) {
				t.Fatal("water cycle lost its return phase; endpoint would jump backward")
			}
		}
	}
}
