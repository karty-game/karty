package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
)

func worldMappingFeatures(assets *assetBuild, document world.Document) ([]string, error) {
	if document.MaterialMapping == nil {
		return nil, nil
	}

	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialMappingV1) {
		return nil, fmt.Errorf("requires an SDK advertising %s: %w", asset.CapabilityWorldMaterialMappingV1, ErrManifest)
	}

	return []string{world.FeatureMaterialMapping}, nil
}
