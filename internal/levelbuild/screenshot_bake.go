package levelbuild

import (
	"context"
	"fmt"

	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func prepareLevelWorld(ctx context.Context, directory string, definition manifest, assets *assetBuild) (manifest, error) {
	if err := validateWorldSchema(directory, definition); err != nil {
		return definition, err
	}

	if assets == nil || !assets.screenshot {
		return definition, nil
	}

	return prepareScreenshotBake(ctx, assets.projectRoot, directory, definition, assets)
}

// Screenshot transport settings are temporary overrides. Authored manifests and
// strict explicit imports remain untouched; normal builds retain their settings.
func prepareScreenshotBake(ctx context.Context, root, directory string, definition manifest, assets *assetBuild) (manifest, error) {
	if definition.Lightmap == nil || !definition.Lightmap.Enabled || definition.Lightmap.Prebake != nil ||
		len(definition.Lightmap.Lights) == 0 {
		return definition, nil
	}

	settings := *definition.Lightmap
	settings.Offline = true
	settings.BakeSamples, settings.BakeBounces = nil, nil

	definition.Lightmap = &settings
	if err := validateLightmapSettings(definition, assets); err != nil {
		return definition, err
	}

	if err := ctx.Err(); err != nil {
		return definition, err
	}

	metadata, _, err := loadBakeMaterials(directory, definition)
	if err != nil {
		return definition, err
	}

	document, _, err := compileMaterialWorld(directory, definition, metadata, nil)
	if err != nil {
		return definition, err
	}

	options, err := settings.options()
	if err != nil {
		return definition, err
	}

	layout, err := worldlightmap.Compile(document, options)
	if err != nil {
		return definition, err
	}

	if len(readAutomaticPrebake(root, directory, definition, layout, document)) != 0 {
		return definition, nil
	}

	const quickSamples, quickBounces = 4, 1

	samples, bounces := quickSamples, quickBounces
	if _, err := bakeLevel(ctx, root, directory, definition, BakeOptions{Samples: &samples, Bounces: &bounces}); err != nil {
		return definition, fmt.Errorf("quick screenshot bake: %w", err)
	}

	if len(readAutomaticPrebake(root, directory, definition, layout, document)) == 0 {
		return definition, fmt.Errorf("quick screenshot bake did not validate: %w", ErrManifest)
	}

	return definition, nil
}
