package assetpipeline

import (
	"context"
	"crypto/sha256"
	"image"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty/internal/ssbump"
)

const worldMaterialSSBumpProcessor = "world-ssbump-material@2"

type worldSSBumpSurface struct {
	height, normals                   *image.NRGBA
	options                           ssbump.Options
	normalStrength, occlusionStrength float64
}

func cachedWorldMaterialSSBump(ctx context.Context, root string, surface worldSSBumpSurface) (*image.NRGBA, Artifact, error) {
	if surface.height == nil || surface.normals == nil || surface.height.Bounds() != surface.normals.Bounds() {
		return nil, Artifact{}, ErrCacheRecipe
	}

	hash := sha256.New()

	for _, pixels := range []*image.NRGBA{surface.height, surface.normals} {
		encoded, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
		if err != nil {
			return nil, Artifact{}, err
		}

		hash.Write(encoded)
	}

	source, err := SnapshotBytes(hash.Sum(nil))
	if err != nil {
		return nil, Artifact{}, err
	}

	recipe, err := NewRecipe("world-ssbump", worldMaterialSSBumpProcessor, []Revision{
		{Name: "algorithm", Version: ssbump.MaterialAlgorithm}, {Name: "coefficient-mips", Version: "linear-average@1"},
	}, struct {
		Options                           ssbump.Options
		NormalStrength, OcclusionStrength float64
	}{surface.options, surface.normalStrength, surface.occlusionStrength})
	if err != nil {
		return nil, Artifact{}, err
	}

	artifact, err := NewProjectCache(
		root,
	).Resolve(ctx, source, recipe, func(ctx context.Context, _ SourceSnapshot, _ Recipe) (Processed, error) {
		pixels, err := ssbump.GenerateMaterial(ctx, surface.height, surface.normals, surface.options,
			surface.normalStrength, surface.occlusionStrength)
		if err != nil {
			return Processed{}, err
		}

		output, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})

		return Processed{Encoding: worldMaterialSSBumpProcessor, Payload: output}, err
	})
	if err != nil {
		return nil, Artifact{}, err
	}

	data, err := readBoundedMaterialFile(artifact.PayloadPath)
	if err != nil {
		return nil, Artifact{}, err
	}

	meta, pixels, err := qoi.Decode(data)
	if err != nil || artifact.Encoding != worldMaterialSSBumpProcessor || pixels.Bounds() != surface.height.Bounds() ||
		meta.Channels != qoi.ChannelsRGBA || meta.Colorspace != qoi.ColorspaceLinear {
		return nil, Artifact{}, ErrCacheOutput
	}

	return pixels, artifact, nil
}
