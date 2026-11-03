package assetpipeline

import (
	"context"
	"image"
	"image/color"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldmaterialssbump"
)

// PackWorldMaterialSSBump accepts independent completed 256px tiles, including
// any already reprojected/composited charts. It never traces the packed image.
func PackWorldMaterialSSBump(
	ctx context.Context,
	layout worldmaterial.Layout,
	tiles map[uint32]*image.NRGBA,
) (worldmaterialssbump.Pair, error) {
	if err := layout.Validate(); err != nil {
		return worldmaterialssbump.Pair{}, err
	}

	if err := ctx.Err(); err != nil {
		return worldmaterialssbump.Pair{}, err
	}

	ids, err := validateSSBumpTiles(layout, tiles)
	if err != nil {
		return worldmaterialssbump.Pair{}, err
	}

	descriptor, err := worldmaterialssbump.NewDescriptor(layout, ids)
	if err != nil {
		return worldmaterialssbump.Pair{}, err
	}

	width, height := layout.MipDimensions()
	if (uint64(layout.Width)*uint64(layout.Height)+uint64(width)*uint64(height))*4 > asset.MaxDecodedTextures {
		return worldmaterialssbump.Pair{}, ErrAssetResourceLimits
	}

	atlas := flatSSBump(layout.Width, layout.Height)

	var tail *image.NRGBA
	if len(layout.Mips) != 0 {
		tail = flatSSBump(width, height)
	}

	for slot, rect := range layout.Materials {
		previous := tiles[rect.MaterialID]
		if previous == nil {
			continue
		}

		if err := ctx.Err(); err != nil {
			return worldmaterialssbump.Pair{}, err
		}

		putMaterialMip(atlas, previous, rect)

		if tail == nil {
			continue
		}

		for level := 1; level <= worldmaterial.MipLevels; level++ {
			if err := ctx.Err(); err != nil {
				return worldmaterialssbump.Pair{}, err
			}

			previous = downsampleSSBump(previous)
			placement, _ := layout.MipRect(level, slot, true)
			putMaterialMip(tail, previous, placement)
		}
	}

	pair := worldmaterialssbump.Pair{Descriptor: descriptor}

	pair.Atlas, _, err = qoi.Encode(atlas, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		return worldmaterialssbump.Pair{}, err
	}

	if tail != nil {
		pair.MipTail, _, err = qoi.Encode(tail, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
		if err != nil {
			return worldmaterialssbump.Pair{}, err
		}
	}

	return pair, pair.Validate(layout)
}

func validateSSBumpTiles(layout worldmaterial.Layout, tiles map[uint32]*image.NRGBA) ([]uint32, error) {
	ids := make([]uint32, 0, len(tiles))
	for _, rect := range layout.Materials {
		pixels, exists := tiles[rect.MaterialID]
		if !exists {
			continue
		}

		if pixels == nil || pixels.Bounds() != image.Rect(0, 0, rect.Width, rect.Height) {
			return nil, ErrCacheOutput
		}

		for offset := 3; offset < len(pixels.Pix); offset += 4 {
			if pixels.Pix[offset] != worldSSBumpOpaque {
				return nil, ErrCacheOutput
			}
		}

		ids = append(ids, rect.MaterialID)
	}

	if len(ids) != len(tiles) {
		return nil, ErrCacheOutput
	}

	return ids, nil
}

func flatSSBump(width, height int) *image.NRGBA {
	p := image.NewNRGBA(image.Rect(0, 0, width, height))
	for o := 0; o < len(p.Pix); o += 4 {
		copy(p.Pix[o:o+4], []byte{85, 85, 85, 255})
	}

	return p
}

// Coefficient energy and directional occlusion are averaged, never normalized.
func downsampleSSBump(source *image.NRGBA) *image.NRGBA {
	size := source.Bounds().Dx() / materialMipScale

	result := image.NewNRGBA(image.Rect(0, 0, size, size))
	for row := range size {
		for column := range size {
			var sum [3]uint32

			for sample := range materialMipSamples {
				p := source.NRGBAAt(column*materialMipScale+sample%materialMipScale, row*materialMipScale+sample/materialMipScale)
				sum[0] += uint32(p.R)
				sum[1] += uint32(p.G)
				sum[2] += uint32(p.B)
			}

			result.SetNRGBA(column, row, color.NRGBA{
				uint8((sum[0] + worldSSBumpMipRound) / materialMipSamples),
				uint8((sum[1] + worldSSBumpMipRound) / materialMipSamples),
				uint8((sum[2] + worldSSBumpMipRound) / materialMipSamples), worldSSBumpOpaque,
			})
		}
	}

	return result
}
