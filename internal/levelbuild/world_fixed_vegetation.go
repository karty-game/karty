package levelbuild

import (
	"fmt"
	"slices"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
	worldgeometry "github.com/karty-game/karty/internal/worldbuild/geometry"
	worldsource "github.com/karty-game/karty/internal/worldbuild/source"
)

// Foliage is ordinary static bake transport. Use the processed cutout textures
// already selected for runtime sprites; normal textures do not alter diffuse
// visibility or color. The same immutable input is shared by every chunk bake.
//
//nolint:nilnil // An absent sidecar intentionally adds no vegetation bake input.
func prepareFixedVegetationBake(result worldMaterialBuild) (*worldlightmapbake.Vegetation, error) {
	var document *worldvegetation.Document

	for _, entry := range result.data {
		if entry.Name != worldvegetation.EntryName {
			continue
		}

		decoded, err := worldvegetation.Decode(entry.Data)
		if err != nil {
			return nil, err
		}

		document = &decoded

		break
	}

	if document == nil {
		return nil, nil
	}

	ids := make([]uint32, 0, len(document.Presets))
	for _, preset := range document.Presets {
		ids = append(ids, preset.Texture)
	}

	slices.Sort(ids)
	ids = slices.Compact(ids)

	sources := make(map[uint32][]byte, len(ids))
	for _, source := range result.sources {
		textureID, valid := level.TextureAssetID(source.Name)
		if !valid || !slices.Contains(ids, textureID) {
			continue
		}

		if source.Kind != level.EntryTexture || sources[textureID] != nil {
			return nil, fmt.Errorf("vegetation bake texture %d source: %w", textureID, ErrSource)
		}

		sources[textureID] = source.Data
	}

	var pixels uint64

	for _, textureID := range ids {
		metadata, err := qoi.Validate(sources[textureID])
		if err != nil {
			return nil, fmt.Errorf("vegetation bake texture %d: %w", textureID, ErrSource)
		}

		count := uint64(metadata.Width) * uint64(metadata.Height)
		if count > asset.MaxTexturePixels-pixels {
			return nil, fmt.Errorf("vegetation bake textures exceed pixel budget: %w", ErrSource)
		}

		pixels += count
	}

	input := &worldlightmapbake.Vegetation{Document: *document, Materials: make([]worldlightmapbake.Material, 0, len(ids))}
	for _, textureID := range ids {
		_, image, err := qoi.Decode(sources[textureID])
		if err != nil {
			return nil, err
		}

		input.Materials = append(input.Materials, worldlightmapbake.Material{ID: textureID, Albedo: image})
	}

	return input, nil
}

// Explicit render-bake and imported-prebake validation use the same original
// albedo inputs as their existing world-material bake contract.
//
//nolint:nilnil // Sources without placed vegetation intentionally add no bake input.
func prepareAuthoredVegetationBake(directory string, definition manifest, document *world.Document,
	metadata []metadataTexture, materials []worldlightmapbake.Material) (*worldlightmapbake.Vegetation, error) {
	expanded, err := worldsource.Load(directory, definition.World.Source)
	if err != nil {
		return nil, err
	}

	if expanded.Vegetation == nil || definition.omitVegetation {
		return nil, nil
	}

	ids := make(map[string]uint32, len(metadata))
	for _, texture := range metadata {
		ids[texture.Name] = texture.ID
	}

	vegetation, err := worldgeometry.CompileVegetation(expanded, document, ids)
	if err != nil {
		return nil, err
	}

	if vegetation == nil {
		return nil, nil
	}

	selected := make([]worldlightmapbake.Material, 0, len(vegetation.Presets))

	seen := make(map[uint32]bool)
	for _, preset := range vegetation.Presets {
		if seen[preset.Texture] {
			continue
		}

		seen[preset.Texture] = true
		for _, material := range materials {
			if material.ID == preset.Texture {
				selected = append(selected, material)

				break
			}
		}
	}

	return &worldlightmapbake.Vegetation{Document: *vegetation, Materials: selected}, nil
}

func validateVegetationPrebakeProvenance(directory string, definition manifest) error {
	expanded, err := worldsource.Load(directory, definition.World.Source)
	if err != nil {
		return err
	}

	if expanded.Vegetation != nil {
		return fmt.Errorf("vegetation requires an offline prebake with current cutout/placement provenance: %w", worldlightmap.ErrLayout)
	}

	return nil
}
