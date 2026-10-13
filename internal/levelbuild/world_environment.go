package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
)

func worldEnvironmentFeatures(assets *assetBuild, document world.Document) ([]string, error) {
	if document.Environment == nil {
		return nil, nil
	}

	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldEnvironmentV1) {
		return nil, fmt.Errorf("requires an SDK advertising %s: %w", asset.CapabilityWorldEnvironmentV1, ErrManifest)
	}

	features := []string{world.FeatureEnvironment}

	if document.Environment.Backdrop != "" {
		if !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldEnvironmentBackdropV1) {
			return nil, fmt.Errorf("requires an SDK advertising %s: %w", asset.CapabilityWorldEnvironmentBackdropV1, ErrManifest)
		}

		features = append(features, world.FeatureEnvironmentBackdrop)
	}

	return features, nil
}
