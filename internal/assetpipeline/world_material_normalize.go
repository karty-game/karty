package assetpipeline

import (
	"context"
	"image"
	"image/color"
	"math"
)

const (
	materialGeometryCenter          = 128.0
	materialGeometryRange           = 64.0
	materialGeometryMaxGain         = 3.0
	materialGeometryNoiseSpan       = 8.0
	materialGeometryGainRampSpan    = 8.0
	materialGeometryPercentileParts = 100
	materialGeometryMidpointScale   = 0.5
	materialLuminanceScale          = 10_000
)

// normalizeMaterialGeometryInput derives geometry evidence from one texture's
// own covered pixels. It preserves spatial detail and coverage without changing
// the authored albedo or requiring material names or companion images.
func normalizeMaterialGeometryInput(ctx context.Context, source *image.NRGBA) (*image.NRGBA, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var histogram [256]uint64

	var covered uint64

	bounds := source.Bounds()
	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		for column := bounds.Min.X; column < bounds.Max.X; column++ {
			pixel := source.NRGBAAt(column, row)
			if pixel.A != 0 {
				histogram[materialGeometryLuminance(pixel)]++
				covered++
			}
		}
	}

	midpoint, gain := materialGeometryCenter, 1.0

	if covered != 0 {
		trim := (covered - 1) / materialGeometryPercentileParts
		low := materialGeometryQuantile(&histogram, trim)
		high := materialGeometryQuantile(&histogram, covered-1-trim)
		midpoint = float64(low+high) * materialGeometryMidpointScale
		span := float64(high - low)

		gain = materialGeometryGain(span)
	}

	result := image.NewNRGBA(bounds)
	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		for column := bounds.Min.X; column < bounds.Max.X; column++ {
			pixel := source.NRGBAAt(column, row)

			value := materialGeometryCenter
			if pixel.A != 0 {
				value += (float64(materialGeometryLuminance(pixel)) - midpoint) * gain
			}

			gray := uint8(max(0, min(255, math.Round(value))))
			result.SetNRGBA(column, row, color.NRGBA{R: gray, G: gray, B: gray, A: pixel.A})
		}
	}

	return result, nil
}

func materialGeometryLuminance(pixel color.NRGBA) int {
	// Rec.709 coefficients apply directly to encoded byte values, matching the
	// native generator's input convention; there is no gamma conversion here.
	return (2126*int(pixel.R) + 7152*int(pixel.G) + 722*int(pixel.B) + materialLuminanceScale/2) / materialLuminanceScale
}

func materialGeometryQuantile(histogram *[256]uint64, rank uint64) int {
	var total uint64

	for value, count := range histogram {
		total += count
		if total > rank {
			return value
		}
	}

	return len(histogram) - 1
}

func materialGeometryGain(span float64) float64 {
	if span <= materialGeometryNoiseSpan {
		return 1
	}

	maximum := min(materialGeometryRange/span, materialGeometryMaxGain)
	weight := min((span-materialGeometryNoiseSpan)/materialGeometryGainRampSpan, 1)

	return 1 + (maximum-1)*weight
}
