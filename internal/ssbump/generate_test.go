package ssbump_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/karty-game/karty/internal/ssbump"
)

func TestFlatHeightHasUnitRNMResponse(t *testing.T) {
	t.Parallel()

	height := image.NewGray16(image.Rect(3, 4, 19, 20))
	for y := height.Bounds().Min.Y; y < height.Bounds().Max.Y; y++ {
		for x := height.Bounds().Min.X; x < height.Bounds().Max.X; x++ {
			height.SetGray16(x, y, color.Gray16{Y: 32768})
		}
	}

	result, err := ssbump.Generate(t.Context(), height, ssbump.Defaults())
	if err != nil {
		t.Fatal(err)
	}

	for y := range 16 {
		for x := range 16 {
			if actual := result.NRGBAAt(x, y); actual != (color.NRGBA{R: 85, G: 85, B: 85, A: 255}) {
				t.Fatalf("flat texel %d,%d = %+v, want equal unit-response coefficients", x, y, actual)
			}
		}
	}
}

func TestHeightOcclusionIsDirectionalAndPeriodic(t *testing.T) {
	t.Parallel()

	const (
		width, height = 64, 16
		shift         = 7
	)

	input := image.NewGray(image.Rect(0, 0, width, height))
	translated := image.NewGray(input.Bounds())

	for y := range height {
		for x := 36; x < 44; x++ {
			input.SetGray(x, y, color.Gray{Y: 255})
			translated.SetGray((x+shift)%width, y, color.Gray{Y: 255})
		}
	}

	options := ssbump.Options{Depth: 12, Radius: 12, Samples: 128, Steps: 32}

	result, err := ssbump.Generate(t.Context(), input, options)
	if err != nil {
		t.Fatal(err)
	}
	// The floor normal here is flat. Only the height wall at +X can cause
	// directional darkening; scalar AO copied to RGB cannot pass this check.
	pixel := result.NRGBAAt(32, 8)
	if int(pixel.R)+5 >= int(pixel.G) || int(pixel.R)+5 >= int(pixel.B) || int(pixel.R)+int(pixel.G)+int(pixel.B) >= 250 {
		t.Fatalf("wall must occlude +X more than the other lobes and retain darkening: %+v", pixel)
	}

	shifted, err := ssbump.Generate(t.Context(), translated, options)
	if err != nil {
		t.Fatal(err)
	}

	for y := range height {
		for x := range width {
			if actual, expected := shifted.NRGBAAt((x+shift)%width, y), result.NRGBAAt(x, y); actual != expected {
				t.Fatalf("periodic translation changed texel %d,%d: %+v != %+v", x, y, actual, expected)
			}
		}
	}

	again, err := ssbump.Generate(t.Context(), input, options)
	if err != nil || !bytes.Equal(result.Pix, again.Pix) {
		t.Fatalf("identical input/settings must be deterministic: %v", err)
	}
}

func TestGenerationRejectsInvalidOrCanceledWork(t *testing.T) {
	t.Parallel()

	height := image.NewGray(image.Rect(0, 0, 8, 8))

	for name, change := range map[string]func(*ssbump.Options){
		"NaN depth":       func(options *ssbump.Options) { options.Depth = math.NaN() },
		"infinite radius": func(options *ssbump.Options) { options.Radius = math.Inf(1) },
		"zero radius":     func(options *ssbump.Options) { options.Radius = 0 },
		"too many rays":   func(options *ssbump.Options) { options.Samples = 129 },
		"zero steps":      func(options *ssbump.Options) { options.Steps = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			options := ssbump.Defaults()
			change(&options)

			if _, err := ssbump.Generate(t.Context(), height, options); err == nil {
				t.Fatal("accepted invalid controls")
			}
		})
	}

	if err := ssbump.Defaults().Validate(image.Rect(0, 0, ssbump.MaxDimension+1, 1)); err == nil {
		t.Fatal("accepted excessive decoded dimensions")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := ssbump.Generate(ctx, height, ssbump.Defaults()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled generation returned %v", err)
	}

	if _, err := ssbump.Generate(t.Context(), image.NewNRGBA(height.Bounds()), ssbump.Defaults()); err == nil {
		t.Fatal("accepted transparent height values")
	}
}
