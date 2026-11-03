package levelbuild_test

import (
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

// Current 3D fixtures retain the production GPU2/material profile; standalone
// sprite assertions omit only the independent 2D atlas.
func currentWorldSDK(manifest sdk.Manifest) sdk.Manifest {
	manifest.Assets.Capabilities.Runtime = slices.DeleteFunc(
		slices.Clone(manifest.Assets.Capabilities.Runtime),
		func(capability asset.Capability) bool { return capability == asset.CapabilityTextureAtlasV1 },
	)

	return manifest
}
func worldFixtureSDK(t *testing.T) sdk.Manifest {
	t.Helper()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	return currentWorldSDK(manifest)
}
func TestExternalWorldSDKRetainsCurrentGPUProfile(t *testing.T) {
	t.Parallel()

	selected := sdk.Manifest{}
	selected.Assets.Capabilities.Runtime = []asset.Capability{
		asset.CapabilityWorldGPUChunksV2,
		asset.CapabilityWorldMaterialGPUV1,
		asset.CapabilityWorldLightingV1,
		asset.CapabilityTextureAtlasV1,
	}

	filtered := currentWorldSDK(selected)
	if !slices.Equal(filtered.Assets.Capabilities.Runtime, selected.Assets.Capabilities.Runtime[:3]) ||
		len(selected.Assets.Capabilities.Runtime) != 4 {
		t.Fatal("current fixture lost GPU profile or mutated selected SDK")
	}
}
