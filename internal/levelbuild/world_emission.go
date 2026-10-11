package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
)

type materialEmission struct {
	Intensity float64                `toml:"intensity"`
	Pulse     *materialEmissionPulse `toml:"pulse"`
	Lights    []string               `toml:"lights"`
}

type materialEmissionPulse struct {
	Depth         float64 `toml:"depth"`
	PeriodSeconds float64 `toml:"period_seconds"`
	PhaseSeconds  float64 `toml:"phase_seconds"`
}

func (e materialEmission) compiled(materialID uint32) (world.MaterialEmission, error) {
	code, err := world.EncodeEmissionIntensity(e.Intensity)
	if err != nil {
		return world.MaterialEmission{}, err
	}

	result := world.MaterialEmission{Material: materialID, Intensity: code, Lights: slices.Clone(e.Lights)}
	if e.Pulse != nil {
		result.Pulse = &world.EmissionPulse{Depth: e.Pulse.Depth, PeriodSeconds: e.Pulse.PeriodSeconds, PhaseSeconds: e.Pulse.PhaseSeconds}
		if err := result.Pulse.Validate(); err != nil {
			return world.MaterialEmission{}, err
		}
	}

	return result, nil
}

func validateMaterialEmission(definition manifest, assets *assetBuild) error {
	for _, texture := range definition.Textures {
		if texture.Emission == nil {
			continue
		}

		if definition.World.Source == "" || assets == nil ||
			!slices.Contains(assets.manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldEmissionV1) {
			return fmt.Errorf("texture %q emission requires a world and %s: %w", texture.Name, world.FeatureEmission, ErrManifest)
		}

		if _, err := texture.Emission.compiled(1); err != nil {
			return fmt.Errorf("texture %q emission: %w", texture.Name, ErrManifest)
		}
	}

	return nil
}

func compileMaterialEmission(document *world.Document, entries []textureEntry, textures []metadataTexture) error {
	ids := make(map[string]uint32, len(textures))
	for _, texture := range textures {
		ids[texture.Name] = texture.ID
	}

	for _, texture := range entries {
		if texture.Emission == nil {
			continue
		}

		emission, err := texture.Emission.compiled(ids[texture.Name])
		if err != nil {
			return fmt.Errorf("texture %q emission: %w", texture.Name, ErrManifest)
		}

		if document.Emission == nil {
			document.Emission = &world.Emission{Version: world.EmissionVersion}
		}

		document.Emission.Materials = append(document.Emission.Materials, emission)
	}

	if err := world.ValidateEmission(document); err != nil {
		return fmt.Errorf("material emission: %w: %w", err, ErrManifest)
	}

	return nil
}
