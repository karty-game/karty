package geometry

import (
	"fmt"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

// CompileParticles produces the separate bounded particle resource after convex
// sector compilation, resolving named frames and whole-envelope ownership.
func CompileParticles(
	expanded source.Expanded,
	compiledWorld *world.Document,
	textures map[string]uint32,
) (*worldparticles.Document, error) {
	if expanded.Particles == nil {
		return nil, nil //nolint:nilnil // Absent optional particles intentionally produces no payload.
	}

	if compiledWorld == nil {
		return nil, worldparticles.ErrConfinement
	}

	if expanded.Particles.Version != worldparticles.Version {
		return nil, worldparticles.ErrVersion
	}

	if len(expanded.Particles.Presets) > worldparticles.MaxPresets || len(expanded.ParticleEmitters) > worldparticles.MaxEmitters {
		return nil, worldparticles.ErrBounds
	}

	document := &worldparticles.Document{Version: expanded.Particles.Version}
	presets := map[string]worldparticles.Preset{}

	for _, preset := range expanded.Particles.Presets {
		compiled, err := worldsource.CompileParticlePreset(preset, textures)
		if err != nil {
			return nil, err
		}

		presets[preset.Name] = compiled
		document.Presets = append(document.Presets, compiled)
	}

	for _, authored := range expanded.ParticleEmitters {
		preset, ok := presets[authored.Preset]

		if !ok {
			return nil, worldsource.ErrReference
		}

		emitter := worldparticles.Emitter{
			ID:         authored.ID,
			Preset:     authored.Preset,
			Position:   world.Vec3(authored.Position),
			YawDegrees: authored.YawDegrees,
			Scale:      authored.Scale,
			Seed:       authored.Seed,
			Playback:   authored.Playback,
		}

		if emitter.Scale == 0 {
			emitter.Scale = 1
		}

		emitter.Bounds = worldparticles.ConservativeEmitterBounds(preset, emitter)
		found := false
		owner := expanded.ParticleEmitterRooms[authored.ID]

		for sectorIndex := range compiledWorld.Sectors {
			if owner != "" && particleSectorOwner(compiledWorld.Sectors[sectorIndex]) != owner {
				continue
			}

			if worldparticles.BoundsInSector(emitter.Bounds, &compiledWorld.Sectors[sectorIndex]) {
				emitter.Sector = uint32(sectorIndex)
				found = true

				break
			}
		}

		if !found {
			location := "in the level"
			if owner != "" {
				location = fmt.Sprintf("in room %q", owner)
			}

			return nil, fmt.Errorf(
				"emitter %q envelope %+v must fit one compiled convex sector %s; move it inward or reduce spawn extent, speed, lifetime or size: %w",
				emitter.ID,
				emitter.Bounds,
				location,
				worldparticles.ErrConfinement,
			)
		}

		document.Emitters = append(document.Emitters, emitter)
	}

	if err := worldparticles.ValidateForWorld(document, compiledWorld); err != nil {
		return nil, err
	}

	return document, nil
}

func particleSectorOwner(sector world.Sector) string {
	if sector.Instance == "" {
		return sector.SourceRoom
	}

	return sector.Instance + "/" + sector.SourceRoom
}
