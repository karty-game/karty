package levelbuild

import (
	"context"
	"image"
	"image/color"
	"math"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
)

// Rebuild existing public atlas mip streams after changing tile order. Albedo
// averages premultiplied linear light; data preserves tangent normal length;
// height/AO/reflectance are linear scalars. No authored mip payload is relabelled.
func webGPUChartMips(ctx context.Context, layout worldmaterial.Layout, source [3]*image.NRGBA) ([]byte, []byte, error) {
	width, height := layout.MipDimensions()
	tail := image.NewNRGBA(image.Rect(0, 0, width, height))

	var reflection *image.NRGBA
	if source[chartReflectanceChannel] != nil {
		reflection = image.NewNRGBA(tail.Rect)
	}

	for _, target := range []*image.NRGBA{tail, reflection} {
		if target != nil {
			for at := 3; at < len(target.Pix); at += 4 {
				target.Pix[at] = webGPUChartByteMax
			}
		}
	}

	for slot, original := range layout.Materials {
		if err := writeWebGPUChartMips(ctx, layout, source, original, slot, tail, reflection); err != nil {
			return nil, nil, err
		}
	}

	encoded, _, err := qoi.Encode(tail, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil || reflection == nil {
		return encoded, nil, err
	}

	reflected, _, err := qoi.Encode(reflection, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})

	return encoded, reflected, err
}

func downsampleWebGPUChart(source *image.NRGBA, channel int) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, source.Rect.Dx()/2, source.Rect.Dy()/2))
	for row := range result.Rect.Dy() {
		for column := range result.Rect.Dx() {
			var (
				sum    [4]float64
				normal chartVector
			)

			for dy := range 2 {
				for dx := range 2 {
					pixel := source.NRGBAAt(source.Rect.Min.X+column*2+dx, source.Rect.Min.Y+row*2+dy)

					accumulateWebGPUChartTexel(&sum, &normal, pixel, channel)
				}
			}

			pixel := color.NRGBA{chartByte(sum[0] / 4), chartByte(sum[1] / 4), chartByte(sum[2] / 4), chartByte(sum[3] / 4)}

			switch channel {
			case 0:
				pixel.R, pixel.G, pixel.B = chartSRGB(
					sum[0]/max(sum[3], webGPUChartEpsilon),
				), chartSRGB(
					sum[1]/max(sum[3], webGPUChartEpsilon),
				), chartSRGB(
					sum[2]/max(sum[3], webGPUChartEpsilon),
				)
			case 1:
				normal = chartUnit(normal)
				pixel.R, pixel.G = chartNormalByte(normal[0]), chartNormalByte(normal[1])
			}

			result.SetNRGBA(column, row, pixel)
		}
	}

	return result
}

func writeWebGPUChartMips(ctx context.Context, layout worldmaterial.Layout, source [3]*image.NRGBA,
	original worldmaterial.Rect, slot int, tail, reflection *image.NRGBA,
) error {
	var previous [3]*image.NRGBA

	for channel, pixels := range source {
		if pixels != nil {
			region, ok := pixels.SubImage(image.Rect(original.X, original.Y, original.X+original.Width, original.Y+original.Height)).(*image.NRGBA)
			if !ok {
				return ErrSource
			}

			previous[channel] = region
		}
	}

	for level := 1; level <= worldmaterial.MipLevels; level++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		for channel, pixels := range previous {
			if pixels == nil {
				continue
			}

			mip := downsampleWebGPUChart(pixels, channel)

			rect, err := layout.MipRect(level, slot, channel != 0)
			if err != nil {
				return err
			}

			target := tail
			if channel == chartReflectanceChannel {
				target = reflection
			}

			copyWebGPUTile(target, mip, rect, 0, 0)
			previous[channel] = mip
		}
	}

	return nil
}

func accumulateWebGPUChartTexel(sum *[4]float64, normal *chartVector, pixel color.NRGBA, channel int) {
	values := [4]byte{pixel.R, pixel.G, pixel.B, pixel.A}
	if channel == 0 {
		alpha := float64(pixel.A) / webGPUChartByteMax
		for axis := range 3 {
			sum[axis] += chartLinear(values[axis]) * alpha
		}

		sum[3] += alpha
	} else {
		for axis := range 4 {
			sum[axis] += float64(values[axis]) / webGPUChartByteMax
		}

		if channel == 1 {
			x, y := float64(
				int(pixel.R)-webGPUChartNormalMidpoint,
			)/webGPUChartNormalScale, float64(
				int(pixel.G)-webGPUChartNormalMidpoint,
			)/webGPUChartNormalScale
			*normal = chartAdd(*normal, chartUnit(chartVector{x, y, math.Sqrt(max(0, 1-x*x-y*y))}))
		}
	}
}
