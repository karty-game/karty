package ssbump_test

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/karty-game/karty/internal/ssbump"
)

func TestMaterialNormalAndOcclusionStrengthsAreIndependent(t *testing.T) {
	t.Parallel()

	height := image.NewGray(image.Rect(0, 0, 32, 8))
	normals := image.NewNRGBA(height.Bounds())

	for y := range 8 {
		for x := range 32 {
			normals.SetNRGBA(x, y, color.NRGBA{R: 192, G: 128, A: 255})

			if x >= 16 && x < 20 {
				height.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}

	options := ssbump.Options{Depth: 12, Radius: 12, Samples: 128, Steps: 32}
	withoutHeight := options
	withoutHeight.Depth = 0

	normalOnly, err := ssbump.GenerateMaterial(t.Context(), height, normals, withoutHeight, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	if pixel := normalOnly.NRGBAAt(12, 4); pixel.R < 160 || pixel.R <= pixel.G || pixel.R <= pixel.B {
		t.Fatal("height zero removed the authored +X normal projection", pixel)
	}

	withoutAO, err := ssbump.GenerateMaterial(t.Context(), height, normals, options, 1, 0)
	if err != nil || !bytes.Equal(withoutAO.Pix, normalOnly.Pix) {
		t.Fatal("AO zero retained height self-shadowing or changed the normal response", err)
	}

	shadowOnly, err := ssbump.GenerateMaterial(t.Context(), height, normals, options, 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	if pixel := shadowOnly.NRGBAAt(12, 4); pixel.R >= 80 || pixel.G > 85 || pixel.B > 85 || pixel.R+5 >= pixel.G {
		t.Fatal("normal zero must keep flat projections with directional +X occlusion", pixel)
	}

	flat, err := ssbump.GenerateMaterial(t.Context(), height, normals, withoutHeight, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	for y := range 8 {
		for x := range 32 {
			if pixel := flat.NRGBAAt(x, y); pixel != (color.NRGBA{85, 85, 85, 255}) {
				t.Fatal("disabled effects did not produce unit flat RNM response", pixel)
			}
		}
	}
}
