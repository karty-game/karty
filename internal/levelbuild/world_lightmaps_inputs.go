package levelbuild

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	"slices"
	"strings"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func loadBakeMaterials(directory string, definition manifest) ([]metadataTexture, []worldlightmapbake.Material, error) {
	entries := slices.Clone(definition.Textures)
	slices.SortFunc(entries, func(a, b textureEntry) int { return strings.Compare(a.Name, b.Name) })
	metadata := make([]metadataTexture, 0, len(entries))
	materials := make([]worldlightmapbake.Material, 0, len(entries))
	decodedBytes := int64(0)

	for index, entry := range entries {
		if entry.Name == "" || index > 0 && entry.Name == entries[index-1].Name {
			return nil, nil, ErrSource
		}

		contents, err := readConfinedFile(directory, entry.Source, asset.MaxSourceAssetBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("material %q: %w", entry.Name, err)
		}

		config, _, err := image.DecodeConfig(bytes.NewReader(contents))
		if err != nil || config.Width < 1 || config.Height < 1 ||
			int64(config.Width)*int64(config.Height) > asset.MaxDecodedTextureBytes/bakeTextureChannels {
			return nil, nil, fmt.Errorf("material %q dimensions: %w", entry.Name, ErrSource)
		}

		decodedBytes += int64(config.Width) * int64(config.Height) * bakeTextureChannels
		if decodedBytes > asset.MaxDecodedTextures {
			return nil, nil, fmt.Errorf("bake source texture budget: %w", ErrSource)
		}

		pixels, _, err := image.Decode(bytes.NewReader(contents))
		if err != nil {
			return nil, nil, fmt.Errorf("material %q: %w", entry.Name, err)
		}

		id := uint32(index + 1)
		metadata = append(
			metadata,
			metadataTexture{ID: id, Name: entry.Name, Width: config.Width, Height: config.Height, Coverage: bandCoverage(pixels)},
		)
		materials = append(materials, worldlightmapbake.Material{ID: id, Albedo: pixels})
	}

	return metadata, materials, nil
}

func validateOfflineReflectance(directory string, definition manifest, document world.Document,
	layout worldlightmap.Layout, prebake worldlightmap.PrebakeManifest) error {
	metadata, materials, err := loadBakeMaterials(directory, definition)
	if err != nil {
		return err
	}

	vegetation, err := prepareAuthoredVegetationBake(directory, definition, &document, metadata, materials)
	if err != nil {
		return err
	}

	actual, err := worldlightmapbake.ReflectanceDigestForLayout(document, layout, materials, worldlightmapbake.Options{
		Samples:        prebake.Samples,
		Bounces:        prebake.Bounces,
		Seed:           prebake.Seed,
		MaxRayDistance: prebake.MaxRayDistance,
		Vegetation:     vegetation,
	})
	if err != nil {
		return err
	}

	if actual != prebake.ReflectanceSHA256 {
		return fmt.Errorf("original albedo or surface inputs changed: %w", worldlightmap.ErrLayout)
	}

	return nil
}
