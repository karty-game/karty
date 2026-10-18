package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
)

func worldAnimationFeatures(assets *assetBuild, document world.Document) ([]string, error) {
	features := []string{}

	if document.Fluids != nil {
		if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldFluidsV1) {
			return nil, fmt.Errorf("requires an SDK advertising %s: %w", asset.CapabilityWorldFluidsV1, ErrManifest)
		}

		features = append(features, world.FeatureFluids)
	}

	if document.Animations == nil {
		return features, nil
	}

	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldAnimationsV1) {
		return nil, fmt.Errorf("requires an SDK advertising %s: %w", asset.CapabilityWorldAnimationsV1, ErrManifest)
	}

	return append(features, world.FeatureAnimations), nil
}

func validateAnimationFrames(document world.Document, textures []metadataTexture) error {
	if document.Animations == nil {
		return nil
	}

	lookup := map[uint32]metadataTexture{}
	for _, texture := range textures {
		lookup[texture.ID] = texture
	}

	for _, preset := range document.Animations.Presets {
		if preset.Kind != "flipbook" {
			continue
		}

		first, ok := lookup[preset.Frames[0]]
		if !ok {
			return ErrSource
		}

		for _, frameID := range preset.Frames {
			texture, ok := lookup[frameID]
			if !ok || texture.Width != first.Width || texture.Height != first.Height {
				return fmt.Errorf("animation %q frames must have matching dimensions: %w", preset.Name, ErrSource)
			}
		}
	}

	return nil
}
