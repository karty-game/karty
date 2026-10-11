package assetpipeline

import (
	"context"
	"image"
	"image/color"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
)

const reflectanceMipBufferCount = 2

func newNeutralReflectance(bounds image.Rectangle) *image.NRGBA {
	result := image.NewNRGBA(bounds)
	for offset := materialPixelBytes - 1; offset < len(result.Pix); offset += materialPixelBytes {
		result.Pix[offset] = materialOpaque
	}

	return result
}

// Reflectance stores scalar linear values, never encoded normal coordinates.
// Each material's interior is filtered independently before extruding gutters.
// Unused atlas regions (including the albedo half of each mip row) stay neutral.
func buildWorldReflectanceMips(ctx context.Context, layout worldmaterial.Layout, reflectance *image.NRGBA) (*image.NRGBA, error) {
	if layout.Validate() != nil || len(layout.Mips) != worldmaterial.MipLevels ||
		reflectance.Bounds() != image.Rect(0, 0, layout.Width, layout.Height) {
		return nil, worldmaterial.ErrAtlas
	}

	width, height := layout.MipDimensions()

	cost := uint64(layout.Width)*uint64(layout.Height)*materialPixelBytes*(materialAtlasCount+1) +
		uint64(width)*uint64(height)*materialPixelBytes*reflectanceMipBufferCount
	if cost > asset.MaxDecodedTextures {
		return nil, ErrAssetResourceLimits
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tail := newNeutralReflectance(image.Rect(0, 0, width, height))

	for slot, rect := range layout.Materials {
		previous := materialInterior(reflectance, rect)

		for level := 1; level <= worldmaterial.MipLevels; level++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			current := downsampleReflectance(previous)
			mipRect, _ := layout.MipRect(level, slot, true)
			putMaterialMip(tail, current, mipRect)
			previous = current
		}
	}

	return tail, nil
}

func downsampleReflectance(source *image.NRGBA) *image.NRGBA {
	size := source.Bounds().Dx() / materialMipScale

	result := newNeutralReflectance(image.Rect(0, 0, size, size))
	for row := range size {
		for column := range size {
			var metallic, smoothness uint32

			for sample := range materialMipSamples {
				x, y := materialMipScale*column+sample%materialMipScale, materialMipScale*row+sample/materialMipScale
				pixel := source.NRGBAAt(x, y)
				metallic += uint32(pixel.R)
				smoothness += uint32(pixel.G)
			}

			result.SetNRGBA(column, row, color.NRGBA{
				R: uint8((metallic + materialMipSamples/2) / materialMipSamples),
				G: uint8((smoothness + materialMipSamples/2) / materialMipSamples), A: materialOpaque,
			})
		}
	}

	return result
}
