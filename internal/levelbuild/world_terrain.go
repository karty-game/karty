package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldterrain"
)

type terrainBuildSettings struct {
	Source string `toml:"source"`
}

func compileWorldSkyAndTerrain(directory string, definition manifest, compiled *world.Document,
	assets *assetBuild) ([]level.SourceEntry, []string, error) {
	entries, features, err := compileWorldSky(directory, definition, assets)
	if err != nil {
		return nil, nil, fmt.Errorf("sky: %w", err)
	}

	terrainEntries, terrainFeatures, err := compileWorldTerrain(directory, definition, compiled, assets)
	if err != nil {
		return nil, nil, fmt.Errorf("terrain: %w", err)
	}

	return append(entries, terrainEntries...), append(features, terrainFeatures...), nil
}

func validateTerrainSettings(definition manifest, assets *assetBuild) error {
	if definition.Terrain == nil {
		return nil
	}

	if definition.World.Source == "" || definition.Terrain.Source == "" || assets == nil ||
		!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.Capability(worldterrain.Feature)) {
		return fmt.Errorf("terrain requires a world, source and SDK advertising %s: %w", worldterrain.Feature, ErrManifest)
	}

	if definition.Chunks != nil && definition.Chunks.Enabled {
		return fmt.Errorf("terrain cannot be combined with sector chunk streaming in this version: %w", ErrManifest)
	}

	return nil
}

// compileWorldTerrain imports an already compiled public terrain resource. The
// CLI never executes an external authoring script or evaluates terrain at runtime.
func compileWorldTerrain(directory string, definition manifest, compiled *world.Document,
	assets *assetBuild) ([]level.SourceEntry, []string, error) {
	if definition.Terrain == nil {
		return nil, nil, nil
	}

	if err := validateTerrainSettings(definition, assets); err != nil {
		return nil, nil, err
	}

	encoded, err := readConfinedFile(directory, definition.Terrain.Source, worldterrain.MaxEncodedSize)
	if err != nil {
		return nil, nil, fmt.Errorf("read terrain: %w", err)
	}

	terrain, err := worldterrain.Decode(encoded)
	if err != nil {
		return nil, nil, fmt.Errorf("decode terrain: %w", err)
	}

	if err := worldterrain.ValidateForWorld(&terrain, compiled); err != nil {
		return nil, nil, fmt.Errorf("terrain world association: %w", err)
	}

	return []level.SourceEntry{{Name: worldterrain.EntryName, Kind: level.EntryData, Data: encoded}}, []string{worldterrain.Feature}, nil
}
