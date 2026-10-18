package main

import (
	"image"
	"image/color"
	"math"
	"path/filepath"
	"testing"
)

func TestCharacterAtlasPreservesAllPosesWithGroundedSoles(t *testing.T) {
	sheet := load(filepath.Join("..", "..", sourceArt, "pilgrim-sheet.png"))
	frames, err := characterFrames(sheet)
	if err != nil {
		t.Fatal(err)
	}
	opaque := func(im image.Image) int {
		n := 0
		for y := im.Bounds().Min.Y; y < im.Bounds().Max.Y; y++ {
			for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
				_, _, _, alpha := im.At(x, y).RGBA()
				if alpha >= 0x8000 {
					n++
				}
			}
		}
		return n
	}
	total := 0
	for i, frame := range frames {
		bounds := opaqueCharacterBounds(frame, frame.Bounds())
		if bounds.Empty() || frame.Bounds() != frames[0].Bounds() {
			t.Fatal("missing or inconsistently sized pose", i, bounds)
		}
		// The old regular crops lost row-seven hood pixels and left some soles
		// over twenty screen pixels above their contact at maximum zoom.
		if gap := math.Abs(float64(bounds.Max.Y) - .98*float64(frame.Bounds().Dy())); gap > .5 {
			t.Fatal("sole does not match the fixed sprite anchor", i, gap)
		}
		if center := characterBodyAnchor(frame, bounds); math.Abs(center-float64(frame.Bounds().Dx())/2) > .5 {
			t.Fatal("pose retained body-axis placement drift", i, center)
		}
		if bounds.Min.Y < 2 || bounds.Min.X < 2 || bounds.Max.X > frame.Bounds().Max.X-2 {
			t.Fatal("pose touches a canvas edge", i, bounds)
		}
		total += opaque(frame)
	}
	if want := opaque(sheet); total != want {
		t.Fatalf("atlas extraction lost or duplicated opaque pixels: got %d want %d", total, want)
	}
}

func TestCharacterBodyRegistrationIgnoresWeaponAndStrideWidth(t *testing.T) {
	sheet := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	for row := range 8 {
		for pose := range 8 {
			x, y := pose*16, row*16
			for py := 2; py < 14; py++ {
				for px := 6; px < 11; px++ {
					sheet.SetNRGBA(x+px, y+py, color.NRGBA{R: 180, A: 255})
				}
			}
			// A moving sword/boot extends a different outline side, below the
			// stable head and torso. Whole-bounds centering would move the body.
			extension := 1
			if pose%2 == 1 {
				extension = 14
			}
			sheet.SetNRGBA(x+extension, y+12, color.NRGBA{B: 255, A: 255})
		}
	}
	frames, err := characterFrames(sheet)
	if err != nil {
		t.Fatal(err)
	}
	for i, frame := range frames {
		chest := opaqueCharacterBounds(frame, frame.Bounds())
		center := characterBandCenter(frame, chest, .3, .6)
		if math.Abs(center-float64(frame.Bounds().Dx())/2) > .5 {
			t.Fatal("weapon or stride moved the stable torso", i, center)
		}
	}
}

func TestCharacterAtlasRejectsMissingGutters(t *testing.T) {
	sheet := image.NewNRGBA(image.Rect(0, 0, 80, 80))
	for y := range 80 {
		for x := range 80 {
			sheet.SetNRGBA(x, y, color.NRGBA{A: 255})
		}
	}
	if _, err := characterFrames(sheet); err == nil {
		t.Fatal("malformed atlas combined poses without a transparent gutter")
	}
}
