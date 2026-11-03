package levelbuild

import (
	"context"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

// currentWorldSDK keeps the selected GPU2 material/cell contracts. Individual
// image tests can inspect standalone source textures without enabling 2D atlases.
func currentWorldSDK(manifest sdk.Manifest) sdk.Manifest {
	return withoutRuntimeCapabilities(manifest, asset.CapabilityTextureAtlasV1)
}

func TestCurrentWorldSDKPreservesPreparedGeometry(t *testing.T) {
	t.Parallel()

	selected := sdk.Manifest{}
	selected.Assets.Capabilities.Runtime = []asset.Capability{
		asset.CapabilityWorldGPUChunksV2,
		asset.CapabilityWorldMaterialGPUV1,
		asset.CapabilityTextureAtlasV1,
	}

	filtered := currentWorldSDK(selected)
	if !slices.Contains(filtered.Assets.Capabilities.Runtime, asset.CapabilityWorldGPUChunksV2) ||
		!slices.Contains(filtered.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialGPUV1) ||
		len(selected.Assets.Capabilities.Runtime) != 3 {
		t.Fatal("fixture discarded current prepared contracts or changed the SDK")
	}
}

func withoutRuntimeCapabilities(manifest sdk.Manifest, excluded ...asset.Capability) sdk.Manifest {
	manifest.Assets.Capabilities.Runtime = slices.DeleteFunc(slices.Clone(manifest.Assets.Capabilities.Runtime),
		func(capability asset.Capability) bool { return slices.Contains(excluded, capability) })

	return manifest
}

// buildAllWithFixtureMaterials exercises current GPU2 packaging with deterministic
// source atlas bytes. Materialize device generation has its own integration gate;
// the managed CPU KTX encoder still produces the real final GPU payloads here.
func buildAllWithFixtureMaterials(t *testing.T, root string, uiSchema uint32, theme string, manifest sdk.Manifest) ([]Artifact, error) {
	t.Helper()

	return buildAll(t.Context(), root, uiSchema, theme, fixtureAssetBuild(t, root, manifest))
}

func fixtureAssetBuild(t *testing.T, root string, manifest sdk.Manifest) *assetBuild {
	t.Helper()

	return &assetBuild{
		projectRoot: root,
		manifest:    manifest,
		materials: func(_ context.Context, _ string, _ sdk.Manifest, document world.Document, _ map[uint32][]byte, _ ...map[uint32]assetpipeline.MaterializeOptions) (worldmaterial.Pair, assetpipeline.Artifact, error) {
			return fixtureWorldMaterialPair(t, assetpipeline.WorldMaterialIDs(document)), assetpipeline.Artifact{}, nil
		},
	}
}
