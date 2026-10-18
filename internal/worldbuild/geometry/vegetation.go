package geometry

import (
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

// CompileVegetation resolves room layers after convex decomposition. Placement
// remains seeded runtime data: no plant coordinates are stored in the level.
func CompileVegetation(
	expanded source.Expanded,
	compiledWorld *world.Document,
	textures map[string]uint32,
) (*worldvegetation.Document, error) {
	if expanded.Vegetation == nil {
		return nil, nil //nolint:nilnil // Optional sidecar.
	}

	if compiledWorld == nil {
		return nil, worldvegetation.ErrConfinement
	}

	if len(expanded.Vegetation.Presets) > worldvegetation.MaxPresets {
		return nil, worldvegetation.ErrBounds
	}

	document := &worldvegetation.Document{Version: expanded.Vegetation.Version}
	for _, preset := range expanded.Vegetation.Presets {
		compiled, err := worldsource.CompileVegetationPresetForVersion(preset, textures, expanded.Vegetation.Version)
		if err != nil {
			return nil, err
		}

		document.Presets = append(document.Presets, compiled)
	}

	for _, room := range expanded.Rooms {
		found := false

		for index, sector := range compiledWorld.Sectors {
			if particleSectorOwner(sector) != room.ID {
				continue
			}

			found = true

			for _, layer := range room.VegetationLayers {
				if len(document.Layers) >= worldvegetation.MaxLayers {
					return nil, worldvegetation.ErrBounds
				}

				document.Layers = append(
					document.Layers,
					worldvegetation.Layer{
						Sector:  uint32(index),
						Preset:  layer.Preset,
						Density: layer.Density,
						Seed:    layer.Seed,
						Surface: layer.Surface,
					},
				)
			}
		}

		if !found && len(room.VegetationLayers) > 0 {
			return nil, worldvegetation.ErrConfinement
		}
	}

	if len(document.Layers) == 0 {
		return nil, nil //nolint:nilnil // Unused library has no runtime sidecar.
	}

	if err := worldvegetation.ValidateForWorld(document, compiledWorld); err != nil {
		return nil, err
	}

	return document, nil
}
