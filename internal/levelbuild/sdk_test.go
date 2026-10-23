package levelbuild

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
	return withoutRuntimeCapabilities(
		manifest,
		asset.CapabilityWorldMaterialAtlasV1,
		asset.CapabilityWorldMaterialAtlasV2,
		asset.CapabilityTextureAtlasV1,
		asset.Capability(worldchunk.ReadyFeature),
	)
}

func withoutRuntimeCapabilities(manifest sdk.Manifest, excluded ...asset.Capability) sdk.Manifest {
	manifest.Assets.Capabilities.Runtime = slices.DeleteFunc(slices.Clone(manifest.Assets.Capabilities.Runtime),
		func(capability asset.Capability) bool { return slices.Contains(excluded, capability) })

	return manifest
}
