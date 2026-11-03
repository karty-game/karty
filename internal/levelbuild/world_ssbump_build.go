package levelbuild

import (
	"context"
	"fmt"
	"image"
	"slices"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
	"github.com/karty-game/karty-sdk/format/worldmaterialssbump"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

const worldSSBumpRevision = "karty-cli-webgpu-ssbump@1"

type worldSSBumpBuild struct {
	tiles map[uint32]*image.NRGBA
	cache []assetpipeline.Artifact
	pair  worldmaterialssbump.Pair
}

func worldSSBumpEnabled(assets *assetBuild) bool {
	return assets != nil &&
		(slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialSSBumpV1) ||
			slices.Contains(assets.manifest.Assets.Capabilities.Runtime, worldmaterialgpu.Feature))
}

func generateWorldSSBump(ctx context.Context, assets *assetBuild, pair worldmaterial.Pair) (*worldSSBumpBuild, error) {
	ids := make([]uint32, len(pair.Layout.Materials))
	for i, r := range pair.Layout.Materials {
		ids[i] = r.MaterialID
	}

	tiles, cache, err := assetpipeline.GenerateWorldMaterialSSBump(ctx, assets.projectRoot, assets.manifest, pair, ids)
	if err != nil {
		return nil, err
	}

	return &worldSSBumpBuild{tiles: tiles, cache: cache}, nil
}

func (b *webGPUChartBaker) packSSBump(ctx context.Context, base worldmaterial.Pair) (*worldSSBumpBuild, error) {
	tiles := make(map[uint32]*image.NRGBA, len(base.Layout.Materials))

	generated := make(map[uint32]*image.NRGBA, len(b.tiles))
	for _, t := range b.tiles {
		generated[t.id] = t.ssbump
	}

	for _, rect := range base.Layout.Materials {
		pixels := generated[rect.MaterialID]
		if pixels == nil {
			pixels = b.ssbump.tiles[rect.MaterialID]
		}

		if pixels == nil {
			return nil, fmt.Errorf("SSBump material %d is missing: %w", rect.MaterialID, ErrSource)
		}

		tiles[rect.MaterialID] = pixels
	}

	pair, err := assetpipeline.PackWorldMaterialSSBump(ctx, base.Layout, tiles)
	if err != nil {
		return nil, err
	}

	if err := checkSSBumpCombinedBudget(base, pair); err != nil {
		return nil, err
	}

	if err := pair.ValidateTogether(base); err != nil {
		return nil, fmt.Errorf("SSBump shared atlas budget/validation: %w", err)
	}

	if err := checkSSBumpEntryBounds(pair); err != nil {
		return nil, err
	}

	return &worldSSBumpBuild{tiles: tiles, cache: b.ssbump.cache, pair: pair}, nil
}

func checkSSBumpCombinedBudget(base worldmaterial.Pair, ss worldmaterialssbump.Pair) error {
	var total uint64

	for _, encoded := range [][]byte{base.Albedo, base.Data, base.MipTail, base.Reflectance, base.ReflectanceMipTail, ss.Atlas, ss.MipTail} {
		if len(encoded) == 0 {
			continue
		}

		metadata, err := qoi.Inspect(encoded)
		if err != nil {
			return err
		}

		if metadata.DecodedBytes > asset.MaxDecodedTextures-total {
			return fmt.Errorf("%w: SSBump shared decoded atlas budget exceeds %d bytes", errWebGPUChartEnvelope, asset.MaxDecodedTextures)
		}

		total += metadata.DecodedBytes
	}

	return nil
}

func checkSSBumpEntryBounds(pair worldmaterialssbump.Pair) error {
	for _, entry := range []struct {
		name  string
		bytes []byte
	}{
		{worldmaterialssbump.AtlasEntry, pair.Atlas},
		{worldmaterialssbump.MipTailEntry, pair.MipTail},
	} {
		if len(entry.bytes) > level.MaxEntrySize {
			return fmt.Errorf("%w: %s is %d bytes", errWebGPUChartEnvelope, entry.name, len(entry.bytes))
		}
	}

	return nil
}

func preparedWebGPUMaterials(assets *assetBuild, pair worldmaterial.Pair, cached assetpipeline.Artifact, ss *worldSSBumpBuild) *assetBuild {
	owned := *assets
	owned.ssbump = ss
	owned.materials = func(context.Context, string, sdk.Manifest, world.Document, map[uint32][]byte, ...map[uint32]assetpipeline.MaterializeOptions) (worldmaterial.Pair, assetpipeline.Artifact, error) {
		return pair, cached, nil
	}

	return &owned
}
