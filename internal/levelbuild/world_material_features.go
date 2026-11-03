package levelbuild

import (
	"context"
	"fmt"

	"github.com/karty-game/karty-sdk/format/world"
)

func compiledWorldFeatures(assets *assetBuild, compiled world.Document, name string) ([]string, error) {
	animations, err := worldAnimationFeatures(assets, compiled)
	if err != nil {
		return nil, err
	}

	environment, err := worldEnvironmentFeatures(assets, compiled)
	if err != nil {
		return nil, fmt.Errorf("level %q environment: %w", name, err)
	}

	lighting, err := worldLightingFeatures(assets, compiled)
	if err != nil {
		return nil, fmt.Errorf("level %q world lighting: %w", name, err)
	}

	mapping, err := worldMappingFeatures(assets, compiled)
	if err != nil {
		return nil, fmt.Errorf("level %q material mapping: %w", name, err)
	}

	solids, err := worldSolidsFeatures(assets, compiled)
	if err != nil {
		return nil, fmt.Errorf("level %q static solids: %w", name, err)
	}

	features := animations
	features = append(features, environment...)
	features = append(features, lighting...)
	features = append(features, mapping...)
	features = append(features, solids...)

	return features, nil
}

func finishCompiledWorld(ctx context.Context, directory string, definition manifest, compiled world.Document,
	atlas worldMaterialBuild, assets *assetBuild,
) (worldMaterialBuild, error) {
	var err error

	atlas, err = prepareFixedChunkResources(atlas, definition)
	if err != nil {
		return worldMaterialBuild{}, err
	}

	atlas, err = buildWorldGPUChunks(ctx, directory, definition, compiled, atlas, assets)
	if err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q GPU cells: %w", definition.Level.Name, err)
	}

	if err := validateWorldRuntimeLights(&compiled, atlas); err != nil {
		return worldMaterialBuild{}, fmt.Errorf("level %q world lighting: %w", definition.Level.Name, err)
	}

	return atlas, nil
}
