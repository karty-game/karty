package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
)

func worldSolidsFeatures(assets *assetBuild, document world.Document) ([]string, error) {
	var features []string

	if document.SolidRenderers != nil {
		if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldSolidRenderersV1) {
			return nil, fmt.Errorf("requires an SDK advertising %s: %w", asset.CapabilityWorldSolidRenderersV1, ErrManifest)
		}

		features = append(features, world.FeatureSolidRenderers)
	}

	if document.StaticSolids == nil {
		return features, nil
	}

	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldStaticSolidsV1) {
		return nil, fmt.Errorf("requires an SDK advertising %s: %w", asset.CapabilityWorldStaticSolidsV1, ErrManifest)
	}

	return append(features, world.FeatureStaticSolids), nil
}
