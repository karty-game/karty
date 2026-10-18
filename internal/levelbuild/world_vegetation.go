package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
	worldgeometry "github.com/karty-game/karty/internal/worldbuild/geometry"
	worldsource "github.com/karty-game/karty/internal/worldbuild/source"
)

func compileWorldVegetation(
	directory string,
	definition manifest,
	compiledWorld *world.Document,
	textures []metadataTexture,
	assets *assetBuild,
) ([]level.SourceEntry, []string, error) {
	expanded, err := worldsource.Load(directory, definition.World.Source)
	if err != nil {
		return nil, nil, err
	}

	if expanded.Vegetation == nil {
		return nil, nil, nil
	}

	feature, err := worldvegetation.FeatureForVersion(expanded.Vegetation.Version)
	if err != nil {
		return nil, nil, err
	}

	capability := asset.Capability(feature)
	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, capability) {
		return nil, nil, fmt.Errorf("vegetation requires SDK advertising %s: %w", capability, ErrManifest)
	}

	ids := map[string]uint32{}
	for _, texture := range textures {
		ids[texture.Name] = texture.ID
	}

	document, err := worldgeometry.CompileVegetation(expanded, compiledWorld, ids)
	if err != nil {
		return nil, nil, err
	}

	if document == nil {
		return nil, nil, nil
	}

	if err := validateVegetationNormalDimensions(document, textures); err != nil {
		return nil, nil, err
	}

	encoded, err := worldvegetation.Encode(*document)
	if err != nil {
		return nil, nil, err
	}

	return []level.SourceEntry{
		{Name: worldvegetation.EntryName, Kind: level.EntryData, Data: encoded},
	}, []string{
		feature,
	}, nil
}

func validateVegetationNormalDimensions(document *worldvegetation.Document, textures []metadataTexture) error {
	dimensions := make(map[uint32]metadataTexture, len(textures))
	for _, texture := range textures {
		dimensions[texture.ID] = texture
	}

	for _, preset := range document.Presets {
		if preset.NormalTexture == 0 {
			continue
		}

		albedo := dimensions[preset.Texture]

		normal := dimensions[preset.NormalTexture]
		if albedo.Width != normal.Width || albedo.Height != normal.Height {
			return fmt.Errorf("vegetation %q normal texture dimensions differ from albedo: %w", preset.Name, ErrSource)
		}
	}

	return nil
}
