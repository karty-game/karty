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
)

// Fixed transport uses the processed albedo payloads that feed world materials.
// Profile filters and authored size limits have already run. Normal maps,
// sprites and other unrelated textures must not consume the diffuse budget.
// Include surfaces throughout the world: another chunk can reflect light into
// the selected receiver. Terrain retains its separately compiled far color.
func loadFixedBakeMaterials(document *world.Document, sources []level.SourceEntry) ([]worldlightmapbake.Material, error) {
	ids, err := fixedBakeMaterialIDs(document)
	if err != nil {
		return nil, err
	}

	encoded := make(map[uint32][]byte, len(ids))

	needed := make(map[uint32]bool, len(ids))
	for _, materialID := range ids {
		needed[materialID] = true
	}

	for _, source := range sources {
		materialID, valid := level.TextureAssetID(source.Name)
		if !valid || !needed[materialID] {
			continue
		}

		_, duplicate := encoded[materialID]

		if source.Kind != level.EntryTexture || duplicate {
			return nil, fmt.Errorf("fixed bake material %d source: %w", materialID, ErrSource)
		}

		encoded[materialID] = source.Data
	}
	// Validate all required streams and their aggregate cost before any decode.
	// This is the baker's existing 16 Mi-pixel limit, not the larger runtime
	// texture budget. Unrelated texture bytes are never inspected or decoded.
	pixels := uint64(0)

	for _, materialID := range ids {
		metadata, err := qoi.Validate(encoded[materialID])
		if err != nil {
			return nil, fmt.Errorf("fixed bake material %d processed albedo: %w", materialID, ErrSource)
		}

		count := uint64(metadata.Width) * uint64(metadata.Height)
		if count > asset.MaxTexturePixels-pixels {
			return nil, fmt.Errorf("fixed bake albedo textures exceed %d pixels: %w", asset.MaxTexturePixels, ErrSource)
		}

		pixels += count
	}

	materials := make([]worldlightmapbake.Material, 0, len(ids))
	for _, materialID := range ids {
		_, albedo, err := qoi.Decode(encoded[materialID])
		if err != nil {
			return nil, fmt.Errorf("fixed bake material %d processed albedo decode: %w", materialID, ErrSource)
		}

		materials = append(materials, worldlightmapbake.Material{ID: materialID, Albedo: albedo})
	}

	return materials, nil
}

func fixedBakeMaterialIDs(document *world.Document) ([]uint32, error) {
	surfaces, err := worldlightmap.Surfaces(document)
	if err != nil {
		return nil, err
	}

	var ids []uint32

	seen := make(map[uint32]bool)

	for _, surface := range surfaces {
		if len(surface.Polygons) == 0 {
			continue
		}

		binding := surface.Binding

		var materialID uint32

		switch binding.Kind {
		case "sector-floor":
			materialID = document.Sectors[binding.Index].FloorMaterial
		case "sector-ceiling":
			materialID = document.Sectors[binding.Index].CeilingMaterial
		case "sector-wall":
			materialID = document.Sectors[binding.Index].Walls[binding.Edge].Material
		case "solid-top":
			materialID = document.StaticSolids.Items[binding.Index].TopMaterial
		case "solid-bottom":
			materialID = document.StaticSolids.Items[binding.Index].BottomMaterial
		case "solid-side":
			materialID = document.StaticSolids.Items[binding.Index].SideMaterial
		default:
			return nil, ErrSource
		}

		if materialID != 0 && !seen[materialID] {
			ids = append(ids, materialID)
			seen[materialID] = true
		}
	}

	slices.Sort(ids)

	return ids, nil
}

func loadChunkBakeMaterials(
	directory string,
	definition manifest,
	document *world.Document,
	sources []level.SourceEntry,
) ([]worldlightmapbake.Material, error) {
	if definition.Lightmap != nil && definition.Lightmap.Fixed {
		return loadFixedBakeMaterials(document, sources)
	}

	_, materials, err := loadBakeMaterials(directory, definition)

	return materials, err
}
