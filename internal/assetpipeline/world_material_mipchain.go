package assetpipeline

import (
	"context"
	"image"
	"image/color"
	"math"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
)

const (
	materialSRGBThreshold   = 0.04045
	materialLinearThreshold = 0.0031308
	materialSRGBSlope       = 12.92
	materialSRGBOffset      = 0.055
	materialSRGBScale       = 1.055
	materialSRGBGamma       = 2.4
	materialMipSamples      = 4
	materialMipScale        = 2
)

// Build interiors independently before extruding their own level's gutter.
// The combined tail is raw storage: its albedo regions still hold sRGB bytes.
func buildWorldMaterialMips(ctx context.Context, layout worldmaterial.Layout, albedo, data *image.NRGBA) (*image.NRGBA, error) {
	if layout.Validate() != nil || len(layout.Mips) != worldmaterial.MipLevels ||
		albedo.Bounds() != image.Rect(0, 0, layout.Width, layout.Height) || data.Bounds() != albedo.Bounds() {
		return nil, worldmaterial.ErrAtlas
	}

	width, height := layout.MipDimensions()

	cost := uint64(
		layout.Width,
	)*uint64(
		layout.Height,
	)*materialPixelBytes*materialAtlasCount + uint64(
		width,
	)*uint64(
		height,
	)*materialPixelBytes
	if cost > asset.MaxDecodedTextures {
		return nil, ErrAssetResourceLimits
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tail := image.NewNRGBA(image.Rect(0, 0, width, height))

	for slot, rect := range layout.Materials {
		previousAlbedo, previousData := materialInterior(albedo, rect), materialInterior(data, rect)

		for level := 1; level <= worldmaterial.MipLevels; level++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			currentAlbedo, currentData := downsampleMaterial(previousAlbedo, previousData)
			albedoRect, _ := layout.MipRect(level, slot, false)
			dataRect, _ := layout.MipRect(level, slot, true)

			putMaterialMip(tail, currentAlbedo, albedoRect)
			putMaterialMip(tail, currentData, dataRect)
			previousAlbedo, previousData = currentAlbedo, currentData
		}
	}

	return tail, nil
}

func materialInterior(source *image.NRGBA, rect worldmaterial.Rect) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, rect.Width, rect.Height))
	for row := range rect.Height {
		start := source.PixOffset(rect.X, rect.Y+row)
		copy(
			result.Pix[row*result.Stride:row*result.Stride+rect.Width*materialPixelBytes],
			source.Pix[start:start+rect.Width*materialPixelBytes],
		)
	}

	return result
}

func putMaterialMip(destination, source *image.NRGBA, rect worldmaterial.Rect) {
	for row := -rect.Gutter; row < rect.Height+rect.Gutter; row++ {
		for column := -rect.Gutter; column < rect.Width+rect.Gutter; column++ {
			destination.SetNRGBA(rect.X+column, rect.Y+row,
				source.NRGBAAt(min(max(column, 0), rect.Width-1), min(max(row, 0), rect.Height-1)))
		}
	}
}

func downsampleMaterial(albedo, data *image.NRGBA) (*image.NRGBA, *image.NRGBA) {
	var srgb [256]float64
	for value := range srgb {
		srgb[value] = materialDecodeSRGB(uint8(value))
	}

	size := albedo.Bounds().Dx() / materialMipScale

	resultAlbedo, resultData := image.NewNRGBA(image.Rect(0, 0, size, size)), image.NewNRGBA(image.Rect(0, 0, size, size))
	for row := range size {
		for column := range size {
			var (
				linear, normal              [3]float64
				height, occlusion, coverage float64
			)

			for sample := range materialMipSamples {
				x, y := materialMipScale*column+sample%materialMipScale, materialMipScale*row+sample/materialMipScale
				albedoPixel, dataPixel := albedo.NRGBAAt(x, y), data.NRGBAAt(x, y)
				weight := float64(albedoPixel.A) / materialOpaque
				coverage += weight
				linear[0] += srgb[albedoPixel.R] * weight
				linear[1] += srgb[albedoPixel.G] * weight
				linear[2] += srgb[albedoPixel.B] * weight

				n := materialDecodeNormal(dataPixel.R, dataPixel.G)
				for axis := range normal {
					normal[axis] += n[axis] * weight
				}

				height += float64(dataPixel.B) * weight
				occlusion += float64(dataPixel.A) * weight
			}

			if coverage == 0 {
				resultAlbedo.SetNRGBA(column, row, color.NRGBA{})
				resultData.SetNRGBA(column, row, color.NRGBA{R: materialNeutral, G: materialNeutral, A: materialOpaque})

				continue
			}

			normal = materialNormalizeNormal(normal)

			resultAlbedo.SetNRGBA(column, row, color.NRGBA{
				R: materialEncodeSRGB(linear[0] / coverage),
				G: materialEncodeSRGB(linear[1] / coverage),
				B: materialEncodeSRGB(linear[2] / coverage), A: uint8(math.Round(coverage * materialOpaque / materialMipSamples)),
			})
			resultData.SetNRGBA(column, row, color.NRGBA{
				R: materialEncodeNormal(normal[0]), G: materialEncodeNormal(normal[1]),
				B: uint8(math.Round(height / coverage)), A: uint8(math.Round(occlusion / coverage)),
			})
		}
	}

	return resultAlbedo, resultData
}

func materialDecodeSRGB(value uint8) float64 {
	linear := float64(value) / materialOpaque
	if linear <= materialSRGBThreshold {
		return linear / materialSRGBSlope
	}

	return math.Pow((linear+materialSRGBOffset)/materialSRGBScale, materialSRGBGamma)
}

func materialEncodeSRGB(linear float64) uint8 {
	encoded := materialSRGBSlope * linear
	if linear > materialLinearThreshold {
		encoded = materialSRGBScale*math.Pow(linear, 1/materialSRGBGamma) - materialSRGBOffset
	}

	return uint8(math.Round(min(max(encoded, 0), 1) * materialOpaque))
}

func materialDecodeNormal(red, green uint8) [3]float64 {
	x := min(max((float64(red)-materialNeutral)/materialMidpoint, -1), 1)
	y := min(max((float64(green)-materialNeutral)/materialMidpoint, -1), 1)

	return materialNormalizeNormal([3]float64{x, y, math.Sqrt(max(0, 1-x*x-y*y))})
}

func materialNormalizeNormal(normal [3]float64) [3]float64 {
	length := math.Sqrt(normal[0]*normal[0] + normal[1]*normal[1] + normal[2]*normal[2])
	if length == 0 {
		return [3]float64{0, 0, 1}
	}

	for axis := range normal {
		normal[axis] /= length
	}

	return normal
}

func materialEncodeNormal(component float64) uint8 {
	return uint8(math.Round(min(max(component*materialMidpoint+materialNeutral, 0), materialOpaque)))
}
