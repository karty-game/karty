package levelbuild_test

import (
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty/internal/sdk"
)

// geometrySDK keeps legacy geometry fixtures independent of prepared atlases and chunks.
// Prepared chunk tests opt in explicitly, even when the current SDK enables them.
// Complete SDK manifests remain covered by the sample integration builds.
func geometrySDK(manifest sdk.Manifest) sdk.Manifest {
	manifest.Assets.Capabilities.Runtime = slices.DeleteFunc(slices.Clone(manifest.Assets.Capabilities.Runtime),
		func(capability asset.Capability) bool {
			return capability == asset.CapabilityWorldMaterialAtlasV1 || capability == asset.CapabilityWorldMaterialAtlasV2 ||
				capability == asset.CapabilityTextureAtlasV1 || capability == asset.Capability(worldchunk.ReadyFeature)
		})

	return manifest
}
