package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
)

func worldSolidsFeatures(assets *assetBuild, document world.Document) ([]string, error) {
	if document.StaticSolids == nil {
		return nil, nil
	}

	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldStaticSolidsV1) {
		return nil, fmt.Errorf("requires an SDK advertising %s: %w", asset.CapabilityWorldStaticSolidsV1, ErrManifest)
	}

	return []string{world.FeatureStaticSolids}, nil
}
