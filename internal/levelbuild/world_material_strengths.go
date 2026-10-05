package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
)

// Pointers distinguish explicit zero from an omitted, default-one channel.
type materialStrengths struct {
	Normal *float64 `toml:"normal"`
	Height *float64 `toml:"height"`
	AO     *float64 `toml:"ao"`
	Rim    *float64 `toml:"rim"`
}

func (s materialStrengths) resolved() (worldmaterial.Strengths, error) {
	result := worldmaterial.Strengths{Normal: 1, Height: 1, AO: 1, Rim: 1}

	targets := [4]*float64{&result.Normal, &result.Height, &result.AO, &result.Rim}
	for index, value := range [4]*float64{s.Normal, s.Height, s.AO, s.Rim} {
		if value != nil {
			*targets[index] = *value
		}
	}

	return result, result.Validate()
}

func validateMaterialStrengths(definition manifest, assets *assetBuild) error {
	for _, texture := range definition.Textures {
		if texture.MaterialStrengths == nil {
			continue
		}

		if definition.World.Source == "" || assets == nil ||
			!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV1) {
			return fmt.Errorf("texture %q material strengths require a world and %s: %w", texture.Name, worldmaterial.Feature, ErrManifest)
		}

		if _, err := texture.MaterialStrengths.resolved(); err != nil {
			return fmt.Errorf("texture %q material strengths: %w", texture.Name, ErrManifest)
		}
	}

	return nil
}

func resolveMaterialStrengths(
	entries []textureEntry,
	textures []metadataTexture,
	document world.Document,
) (map[uint32]*worldmaterial.Strengths, error) {
	result := make(map[uint32]*worldmaterial.Strengths)

	ids := make(map[string]uint32, len(textures))
	for _, texture := range textures {
		ids[texture.Name] = texture.ID
	}

	used := assetpipeline.WorldMaterialIDs(document)

	for _, texture := range entries {
		if texture.MaterialStrengths == nil {
			continue
		}

		materialID := ids[texture.Name]
		if materialID == 0 || !slices.Contains(used, materialID) {
			return nil, fmt.Errorf("texture %q material strengths require a used world surface: %w", texture.Name, ErrManifest)
		}

		value, err := texture.MaterialStrengths.resolved()
		if err != nil {
			return nil, ErrManifest
		}

		result[materialID] = &value
	}

	if len(result) != 0 {
		layout, err := worldmaterial.NewLayout(used)
		if err != nil {
			return nil, ErrManifest
		}

		layout, err = worldmaterial.NewMipLayout(layout)
		if err != nil || applyMaterialStrengths(layout, result).Validate() != nil {
			return nil, ErrManifest
		}
	}

	return result, nil
}

func applyMaterialStrengths(layout worldmaterial.Layout, strengths map[uint32]*worldmaterial.Strengths) worldmaterial.Layout {
	layout.Materials = append([]worldmaterial.Rect(nil), layout.Materials...)
	for index := range layout.Materials {
		if value := strengths[layout.Materials[index].MaterialID]; value != nil {
			owned := *value
			layout.Materials[index].Strengths = &owned
		}
	}

	return layout
}
