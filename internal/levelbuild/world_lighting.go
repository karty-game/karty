package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
)

func worldLightingFeatures(assets *assetBuild, document world.Document) ([]string, error) {
	if document.Lighting == nil {
		return nil, nil
	}

	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldLightingV1) {
		return nil, fmt.Errorf("requires an SDK advertising %s: %w", asset.CapabilityWorldLightingV1, ErrManifest)
	}

	return []string{world.FeatureLighting}, nil
}
