package levelbuild

import (
	"fmt"
	"slices"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldparticles"
	worldgeometry "github.com/karty-game/karty/internal/worldbuild/geometry"
	worldsource "github.com/karty-game/karty/internal/worldbuild/source"
)

func compileWorldParticles(
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

	if expanded.Particles == nil {
		return nil, nil, nil
	}

	feature, err := worldparticles.FeatureForVersion(expanded.Particles.Version)
	if err != nil {
		return nil, nil, err
	}

	capability := asset.Capability(feature)
	if assets == nil || !slices.Contains(assets.manifest.Assets.Capabilities.Runtime, capability) {
		return nil, nil, fmt.Errorf("particles require SDK advertising %s: %w", capability, ErrManifest)
	}

	ids := map[string]uint32{}

	for _, texture := range textures {
		ids[texture.Name] = texture.ID
	}

	particles, err := worldgeometry.CompileParticles(expanded, compiledWorld, ids)
	if err != nil {
		return nil, nil, err
	}

	if err := validateParticleFrames(particles, textures); err != nil {
		return nil, nil, err
	}

	encoded, err := worldparticles.Encode(*particles)
	if err != nil {
		return nil, nil, err
	}

	return []level.SourceEntry{
		{Name: worldparticles.EntryName, Kind: level.EntryData, Data: encoded},
	}, []string{
		feature,
	}, nil
}

func validateParticleFrames(particles *worldparticles.Document, textures []metadataTexture) error {
	lookup := map[uint32]metadataTexture{}

	for _, texture := range textures {
		lookup[texture.ID] = texture
	}

	for _, preset := range particles.Presets {
		first, ok := lookup[preset.Sprite.Frames[0]]

		if !ok || first.Width != first.Height || first.Width == 0 {
			return fmt.Errorf("particle preset %q frames must be square: %w", preset.Name, ErrSource)
		}

		for _, id := range preset.Sprite.Frames {
			texture, ok := lookup[id]

			if !ok || texture.Width != first.Width || texture.Height != first.Height {
				return fmt.Errorf("particle preset %q frames must match dimensions: %w", preset.Name, ErrSource)
			}
		}
	}

	return nil
}
