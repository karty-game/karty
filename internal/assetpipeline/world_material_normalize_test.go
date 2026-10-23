package assetpipeline

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"testing"
)

func TestMaterialGeometryNormalizationBrightnessAndTint(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(-2, 3, 30, 35)

	variants := []*image.NRGBA{image.NewNRGBA(bounds), image.NewNRGBA(bounds), image.NewNRGBA(bounds)}

	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for column := bounds.Min.X; column < bounds.Max.X; column++ {
			value := uint8(48 + ((column-bounds.Min.X)*5+(row-bounds.Min.Y)*7)%80)
			variants[0].SetNRGBA(column, row, color.NRGBA{R: value, G: value, B: value, A: 255})
			variants[1].SetNRGBA(column, row, color.NRGBA{R: value/2 + 30, G: value/2 + 30, B: value/2 + 30, A: 255})
			variants[2].SetNRGBA(column, row, color.NRGBA{R: value/2 + 50, G: uint8(int(value)*3/4 + 40), B: value + 10, A: 255})
		}
	}

	baseline := normalizedMaterialGeometry(t, variants[0])
	for _, source := range variants {
		before := bytes.Clone(source.Pix)
		normalized := normalizedMaterialGeometry(t, source)

		if !bytes.Equal(before, source.Pix) {
			t.Fatal("normalization mutated authored source pixels")
		}

		if !bytes.Equal(normalized.Pix, normalizedMaterialGeometry(t, source).Pix) {
			t.Fatal("normalization must be deterministic")
		}

		for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
			for column := bounds.Min.X; column < bounds.Max.X; column++ {
				pixel, expected := normalized.NRGBAAt(column, row), baseline.NRGBAAt(column, row)
				difference := int(pixel.R) - int(expected.R)

				if difference < -3 || difference > 3 || pixel.G != pixel.R || pixel.B != pixel.R || pixel.A != 255 {
					t.Fatalf("affine brightness/tint changed geometry at (%d,%d): %v versus %v", column, row, pixel, expected)
				}
			}
		}
	}
}

func TestMaterialGeometryNormalizationRetainsLocalDetail(t *testing.T) {
	t.Parallel()

	source := image.NewNRGBA(image.Rect(0, 0, 16, 16))

	for row := range 16 {
		for column := range 16 {
			value := uint8(64)
			if column >= 8 {
				value = 192
			}

			source.SetNRGBA(column, row, color.NRGBA{R: value, G: value, B: value, A: 255})
		}
	}

	source.SetNRGBA(4, 4, color.NRGBA{R: 70, G: 70, B: 70, A: 255})
	source.SetNRGBA(4, 5, color.NRGBA{R: 58, G: 58, B: 58, A: 255})
	result := normalizedMaterialGeometry(t, source)

	for point, want := range map[image.Point]uint8{{X: 4, Y: 4}: 99, {X: 4, Y: 5}: 93, {X: 5, Y: 4}: 96, {X: 12, Y: 4}: 160} {
		if got := result.NRGBAAt(point.X, point.Y).R; got != want {
			t.Fatalf("local detail or adjacent flat texels changed: %v = %d, want %d", point, got, want)
		}
	}
}

func TestMaterialGeometryNormalizationBoundsNoiseAndGain(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		low, high uint8
		wantLow   uint8
		wantHigh  uint8
		wantGain  float64
	}{
		{"flat", 130, 130, 128, 128, 1},
		{"near-flat", 128, 132, 126, 130, 1},
		{"noise-threshold", 100, 108, 124, 132, 1},
		{"ramp-start", 100, 109, 122, 134, 1.25},
		{"ramp-midpoint", 100, 112, 116, 140, 2},
		{"ramp-end", 100, 116, 104, 152, 3},
		{"gain-capped", 100, 120, 98, 158, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := image.NewNRGBA(image.Rect(0, 0, 16, 16))

			for row := range 16 {
				for column := range 16 {
					value := test.low
					if column >= 8 {
						value = test.high
					}

					source.SetNRGBA(column, row, color.NRGBA{R: value, G: value, B: value, A: 255})
				}
			}

			if gain := materialGeometryGain(float64(test.high - test.low)); gain != test.wantGain {
				t.Fatalf("gain transition = %g, want %g", gain, test.wantGain)
			}

			result := normalizedMaterialGeometry(t, source)
			if result.NRGBAAt(0, 0).R != test.wantLow || result.NRGBAAt(15, 0).R != test.wantHigh {
				t.Fatal("flat/noisy inputs or maximum gain were not bounded")
			}
		})
	}
}

func TestMaterialGeometryNormalizationIgnoresTransparentPixels(t *testing.T) {
	t.Parallel()

	source := image.NewNRGBA(image.Rect(0, 0, 32, 32))

	for row := range 32 {
		for column := range 32 {
			source.SetNRGBA(column, row, color.NRGBA{R: 255, B: 255})
		}
	}

	for column := range 16 {
		value := uint8(64)
		if column >= 8 {
			value = 192
		}

		source.SetNRGBA(column, 0, color.NRGBA{R: value, G: value, B: value, A: 128})
	}

	result := normalizedMaterialGeometry(t, source)
	if result.NRGBAAt(0, 0) != (color.NRGBA{R: 96, G: 96, B: 96, A: 128}) ||
		result.NRGBAAt(15, 0) != (color.NRGBA{R: 160, G: 160, B: 160, A: 128}) ||
		result.NRGBAAt(31, 31) != (color.NRGBA{R: 128, G: 128, B: 128}) {
		t.Fatal("hidden colors influenced generation statistics or coverage changed")
	}

	empty := normalizedMaterialGeometry(t, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	if empty.NRGBAAt(0, 0) != (color.NRGBA{R: 128, G: 128, B: 128}) {
		t.Fatal("fully uncovered input must remain neutral and uncovered")
	}
}

func TestMaterialGeometryNormalizationCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := normalizeMaterialGeometryInput(ctx, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	if result != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled normalization returned partial data: %v", err)
	}
}

func normalizedMaterialGeometry(t *testing.T, source *image.NRGBA) *image.NRGBA {
	t.Helper()

	result, err := normalizeMaterialGeometryInput(t.Context(), source)
	if err != nil {
		t.Fatal(err)
	}

	return result
}
