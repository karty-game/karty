package main

import (
	"math"
	"path/filepath"
	"testing"
)

func TestStandingPropsAndFlamesKeepVerticalGroundContact(t *testing.T) {
	t.Chdir("../..")
	previousSolids, previousContents, previousLights := solids, contents, lights
	t.Cleanup(func() { solids, contents, lights = previousSolids, previousContents, previousLights })
	for _, texture := range []string{"z-votives", "z-saint", "z-books"} {
		contents = nil
		prop("fixture", texture, 2, 3, 1.08, .8)
		content := contents[0]
		s := content["actor"].(record)["sprite"].(record)
		im := load(filepath.Join(target, texture+".png"))
		wantHeight := .8 * float64(im.Bounds().Dy()) / float64(im.Bounds().Dx())
		bounds := opaqueCharacterBounds(im, im.Bounds())
		assertStandingContact(t, s, float64(bounds.Max.Y)/float64(im.Bounds().Dy()), wantHeight)
		if content["position"].(record)["z"] != 1.08 {
			t.Fatal("standing prop moved off its authored tabletop")
		}
	}
	contents = nil
	candle("fixture", 2, 3, 1, .8, true)
	s := contents[0]["actor"].(record)["sprite"].(record)
	for frame := range 3 {
		texture := []string{"flame-0", "flame-1", "flame-2"}[frame]
		im := load(filepath.Join(target, texture+".png"))
		bottomVisible := false
		for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
			_, _, _, alpha := im.At(x, im.Bounds().Max.Y-1).RGBA()
			bottomVisible = bottomVisible || alpha != 0
		}
		if !bottomVisible {
			t.Fatal("flame flipbook needs a common visible wick base", frame)
		}
		assertStandingContact(t, s, 1, .36)
	}
	if z := contents[0]["position"].(record)["z"].(float64); math.Abs(z-1.8) > 1e-9 {
		t.Fatal("flame does not meet its wick", z)
	}
	contents = nil
	groundDecal("fixture-decal", "shadow", 2, 3, 1, 1, 0)
	actor := contents[0]["actor"].(record)
	if actor["sprite"].(record)["facing"] != "fixed" || actor["pitch_degrees"] != 90 {
		t.Fatal("ground decal stopped lying on the floor")
	}
	if _, standing := projectedSightHeight(actor["sprite"].(record)); standing {
		t.Fatal("floor decal must not receive a standing silhouette")
	}
}

func assertStandingContact(t *testing.T, sprite record, alphaBase, wantProjectedHeight float64) {
	t.Helper()
	if sprite["facing"] != "upright" {
		t.Fatal("standing sprite tilts away from the vertical", sprite["facing"])
	}
	height, origin := sprite["height"].(float64), sprite["origin_y"].(float64)
	if math.Abs(height*math.Cos(math.Pi/6)-wantProjectedHeight) > 1e-9 {
		t.Fatal("upright art lost its baked screen proportions", height, wantProjectedHeight)
	}
	if gap := (origin - alphaBase) * height; math.Abs(gap) > 1e-9 {
		t.Fatal("visible alpha base floats off its contact plane", gap)
	}
	if projected, ok := projectedSightHeight(sprite); !ok || math.Abs(projected-wantProjectedHeight) > 1e-9 {
		t.Fatal("sight silhouette no longer matches the upright projection", projected)
	}
}

func TestSightRejectsCameraFacingStandingSprites(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("author accepted a camera-facing standing sprite")
		}
	}()
	projectedSightHeight(record{"facing": "camera-facing", "height": 1.0})
}
