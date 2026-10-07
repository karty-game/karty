package levelbuild_test

import (
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/sdk"
)

// geometrySDK isolates level packaging from the external atlas processor.
// Complete SDK manifests remain covered by the sample integration builds.
func geometrySDK(manifest sdk.Manifest) sdk.Manifest {
	manifest.Assets.Capabilities.Runtime = slices.DeleteFunc(slices.Clone(manifest.Assets.Capabilities.Runtime),
		func(capability asset.Capability) bool {
			return capability == asset.CapabilityWorldMaterialAtlasV1 || capability == asset.CapabilityWorldMaterialAtlasV2
		})

	return manifest
}
