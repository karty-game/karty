package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/pelletier/go-toml/v2"
)

// Pointers preserve explicit zero while omitted overrides leave preset defaults intact.
type materializeSettings struct {
	Preset         string   `toml:"preset"`
	NormalStrength *float64 `toml:"normal_strength"`
	HeightContrast *float64 `toml:"height_contrast"`
	MetallicScale  *float64 `toml:"metallic_scale"`
	AODepthScale   *float64 `toml:"ao_depth_scale"`
}

func (s materializeSettings) options() assetpipeline.MaterializeOptions {
	return assetpipeline.MaterializeOptions{Preset: s.Preset, NormalStrength: s.NormalStrength,
		HeightContrast: s.HeightContrast, MetallicScale: s.MetallicScale, AODepthScale: s.AODepthScale}
}

// Limit strict decoding to the new authoring section; retain existing manifest compatibility.
func validateMaterializeKeys(contents []byte) error {
	var raw struct {
		Textures []struct {
			Name        string         `toml:"name"`
			Materialize map[string]any `toml:"materialize"`
		} `toml:"textures"`
	}
	if err := toml.Unmarshal(contents, &raw); err != nil {
		return fmt.Errorf("materialize settings: %w: %w", err, ErrManifest)
	}

	for _, texture := range raw.Textures {
		for key := range texture.Materialize {
			if !slices.Contains([]string{"preset", "normal_strength", "height_contrast", "metallic_scale", "ao_depth_scale"}, key) {
				return fmt.Errorf("texture %q unknown materialize option %q: %w", texture.Name, key, ErrManifest)
			}
		}
	}

	return nil
}

func validateMaterializeSettings(definition manifest, assets *assetBuild) error {
	for _, texture := range definition.Textures {
		if texture.Materialize == nil {
			continue
		}

		if definition.World.Source == "" || assets == nil ||
			(!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV1) &&
				!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV2)) {
			return fmt.Errorf(
				"texture %q materialize settings require a world and material atlas capability: %w",
				texture.Name,
				ErrManifest,
			)
		}

		if err := texture.Materialize.options().Validate(); err != nil {
			return fmt.Errorf("texture %q materialize settings: %w: %w", texture.Name, err, ErrManifest)
		}
	}

	return nil
}

func resolveMaterializeSettings(entries []textureEntry, textures []metadataTexture,
	document world.Document) (map[uint32]assetpipeline.MaterializeOptions, error) {
	result := make(map[uint32]assetpipeline.MaterializeOptions)

	ids := make(map[string]uint32, len(textures))
	for _, texture := range textures {
		ids[texture.Name] = texture.ID
	}

	used := assetpipeline.WorldMaterialIDs(document)

	for _, texture := range entries {
		if texture.Materialize == nil {
			continue
		}

		materialID := ids[texture.Name]
		// Prefab material overrides can replace an imported surface completely.
		// Its package recipe remains valid, but no atlas tile needs generation.
		if texture.Imported && materialID != 0 && !slices.Contains(used, materialID) {
			continue
		}

		if materialID == 0 || !slices.Contains(used, materialID) {
			return nil, fmt.Errorf("texture %q materialize settings require a used world surface: %w", texture.Name, ErrManifest)
		}

		if assetpipeline.WorldMaterialColorOnly(document, materialID) {
			return nil, fmt.Errorf(
				"texture %q materialize settings require generated material data; secondary-only surfaces use color only: %w",
				texture.Name,
				ErrManifest,
			)
		}

		options := texture.Materialize.options()
		if err := options.Validate(); err != nil {
			return nil, fmt.Errorf("texture %q materialize settings: %w: %w", texture.Name, err, ErrManifest)
		}

		result[materialID] = options
	}

	return result, nil
}
