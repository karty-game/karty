package assetpipeline

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"slices"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
	"github.com/karty-game/karty-sdk/format/worldmaterialssbump"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/ssbump"
)

const (
	worldSSBumpProcessor = "world-ssbump-height@1"
	worldSSBumpOpaque    = 255
	worldSSBumpMipRound  = materialMipSamples / 2
)

// GenerateWorldMaterialSSBump generates original material interiors separately.
// Call before surface-chart packing: neighboring chart cells are unrelated
// height fields. The returned tiles own their pixels. Source data is untouched.
func GenerateWorldMaterialSSBump(ctx context.Context, projectRoot string, manifest sdk.Manifest,
	pair worldmaterial.Pair, ids []uint32,
) (map[uint32]*image.NRGBA, []Artifact, error) {
	if !slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialSSBumpV1) &&
		!slices.Contains(manifest.Assets.Capabilities.Runtime, worldmaterialgpu.Feature) {
		return nil, nil, fmt.Errorf("SDK %s does not enable %s: %w", manifest.Version, worldmaterialssbump.Feature, ErrCacheRecipe)
	}

	if _, err := worldmaterialssbump.NewDescriptor(pair.Layout, ids); err != nil {
		return nil, nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	decoded, err := pair.Decode()
	if err != nil {
		return nil, nil, err
	}

	tiles := make(map[uint32]*image.NRGBA, len(ids))

	artifacts := make([]Artifact, 0, len(ids))
	for _, rect := range pair.Layout.Materials {
		if !slices.Contains(ids, rect.MaterialID) {
			continue
		}

		height := image.NewNRGBA(image.Rect(0, 0, rect.Width, rect.Height))
		normals := image.NewNRGBA(height.Bounds())

		for row := range rect.Height {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}

			for x := range rect.Width {
				data := decoded.Data().NRGBAAt(rect.X+x, rect.Y+row)
				value := data.B
				height.SetNRGBA(x, row, color.NRGBA{value, value, value, worldSSBumpOpaque})
				normals.SetNRGBA(x, row, color.NRGBA{R: data.R, G: data.G, A: worldSSBumpOpaque})
			}
		}

		options := ssbump.Defaults()
		normalStrength, occlusionStrength := 1.0, 1.0

		if rect.Strengths != nil {
			options.Depth *= rect.Strengths.Height
			normalStrength, occlusionStrength = rect.Strengths.Normal, rect.Strengths.AO
		}

		tile, cached, err := cachedWorldMaterialSSBump(ctx, projectRoot, worldSSBumpSurface{
			height: height, normals: normals, options: options,
			normalStrength: normalStrength, occlusionStrength: occlusionStrength,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("material %d SSBump: %w", rect.MaterialID, err)
		}

		tiles[rect.MaterialID] = tile

		artifacts = append(artifacts, cached)
	}

	return tiles, artifacts, nil
}

func cachedWorldSSBump(
	ctx context.Context,
	projectRoot string,
	height *image.NRGBA,
	options ssbump.Options,
) (*image.NRGBA, Artifact, error) {
	if err := options.Validate(height.Bounds()); err != nil {
		return nil, Artifact{}, err
	}

	encoded, _, err := qoi.Encode(height, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		return nil, Artifact{}, err
	}

	source, err := SnapshotBytes(encoded)
	if err != nil {
		return nil, Artifact{}, err
	}

	recipe, err := NewRecipe("world-ssbump", worldSSBumpProcessor, []Revision{
		{Name: "algorithm", Version: ssbump.Algorithm}, {Name: "coefficient-mips", Version: "linear-average@1"},
	}, options)
	if err != nil {
		return nil, Artifact{}, err
	}

	cached, err := NewProjectCache(
		projectRoot,
	).Resolve(ctx, source, recipe, func(ctx context.Context, s SourceSnapshot, _ Recipe) (Processed, error) {
		_, input, err := qoi.Decode(s.Bytes())
		if err != nil {
			return Processed{}, err
		}

		pixels, err := generateWorldSSBumpTile(ctx, input, options)
		if err != nil {
			return Processed{}, err
		}

		output, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})

		return Processed{Encoding: worldSSBumpProcessor, Payload: output}, err
	})
	if err != nil {
		return nil, Artifact{}, err
	}

	if err := ctx.Err(); err != nil {
		return nil, Artifact{}, err
	}

	payload, err := readBoundedMaterialFile(cached.PayloadPath)
	if err != nil {
		return nil, Artifact{}, err
	}

	meta, pixels, err := qoi.Decode(payload)
	if err != nil || cached.Encoding != worldSSBumpProcessor || pixels.Bounds() != height.Bounds() || meta.Channels != qoi.ChannelsRGBA ||
		meta.Colorspace != qoi.ColorspaceLinear {
		return nil, Artifact{}, ErrCacheOutput
	}

	for o := 3; o < len(pixels.Pix); o += 4 {
		if pixels.Pix[o] != worldSSBumpOpaque {
			return nil, Artifact{}, ErrCacheOutput
		}
	}

	return pixels, cached, nil
}

// A constant periodic height field (or zero depth) has exactly equal unit
// response. Avoid tracing millions of identical open rays for default slots.
func generateWorldSSBumpTile(ctx context.Context, input *image.NRGBA, options ssbump.Options) (*image.NRGBA, error) {
	constant := true
	opaqueGray := true

	for o := 0; o < len(input.Pix); o += 4 {
		if input.Pix[o] != input.Pix[o+1] || input.Pix[o] != input.Pix[o+2] || input.Pix[o+3] != 255 {
			opaqueGray = false
		}

		if input.Pix[o] != input.Pix[0] || input.Pix[o+1] != input.Pix[0] || input.Pix[o+2] != input.Pix[0] || input.Pix[o+3] != 255 {
			constant = false
		}
	}

	if opaqueGray && (constant || options.Depth == 0) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		return flatSSBump(input.Bounds().Dx(), input.Bounds().Dy()), nil
	}

	return ssbump.Generate(ctx, input, options)
}
