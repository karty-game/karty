package levelbuild

import (
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/sdk"
)

// geometrySDK isolates level packaging from the external atlas processor.
// Complete SDK manifests remain covered by the sample integration builds.
func geometrySDK(manifest sdk.Manifest) sdk.Manifest {
	return withoutRuntimeCapabilities(manifest, asset.CapabilityWorldMaterialAtlasV1, asset.CapabilityWorldMaterialAtlasV2)
}

func withoutRuntimeCapabilities(manifest sdk.Manifest, excluded ...asset.Capability) sdk.Manifest {
	manifest.Assets.Capabilities.Runtime = slices.DeleteFunc(slices.Clone(manifest.Assets.Capabilities.Runtime),
		func(capability asset.Capability) bool { return slices.Contains(excluded, capability) })

	return manifest
}
