package source

import (
	"math"

	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"go.yaml.in/yaml/v3"
)

func rootParticleEmitters(particles *worldsource.Particles) []worldsource.ParticleEmitter {
	if particles == nil {
		return nil
	}

	return particles.Emitters
}

func transformParticleEmitter(emitter worldsource.ParticleEmitter, prefix string, transform affine) worldsource.ParticleEmitter {
	emitter.ID = joinIdentity(prefix, emitter.ID)
	emitter.Position = transformPoint3(emitter.Position, transform)
	emitter.YawDegrees = math.Remainder(emitter.YawDegrees+transform.yaw*180/math.Pi, 360)

	if emitter.Scale == 0 {
		emitter.Scale = 1
	}

	emitter.Scale *= transform.scale

	return emitter
}

func validateParticlePresence(document *yaml.Node) error {
	if len(document.Content) == 0 {
		return nil
	}

	root := document.Content[0]
	version := uint16(0)

	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "version" {
			if err := root.Content[i+1].Decode(&version); err != nil {
				return ErrYAML
			}
		}
	}

	return visitParticlePresence(root, version)
}

func visitParticlePresence(node *yaml.Node, version uint16) error {
	if node.Kind == yaml.MappingNode {
		if err := validateParticleMapping(node, version); err != nil {
			return err
		}
	}

	for _, child := range node.Content {
		if err := visitParticlePresence(child, version); err != nil {
			return err
		}
	}

	return nil
}

func validateParticleMapping(node *yaml.Node, version uint16) error {
	for i := 0; i+1 < len(node.Content); i += 2 {
		name, value := node.Content[i].Value, node.Content[i+1]

		if name != "particles" && name != "particle_emitters" {
			continue
		}

		if version < worldsource.ParticlesVersion {
			return worldsource.ErrVersion
		}

		if err := rejectParticleNulls(value); err != nil {
			return err
		}

		kind := "Particles"
		if name == "particle_emitters" {
			kind = "[]ParticleEmitter"
		}

		if err := validateParticleYAMLFields(value, kind); err != nil {
			return err
		}
	}

	return nil
}

func rejectParticleNulls(node *yaml.Node) error {
	if node.Tag == "!!null" {
		return worldsource.ErrContent
	}

	for _, child := range node.Content {
		if err := rejectParticleNulls(child); err != nil {
			return err
		}
	}

	return nil
}

func appendParticleEmitter(result *scopeResult, emitter worldsource.ParticleEmitter, room string) error {
	if len(result.particleEmitters) >= worldparticles.MaxEmitters {
		return ErrExpansion
	}

	result.particleEmitters = append(result.particleEmitters, emitter)

	if room != "" {
		if result.particleEmitterRooms == nil {
			result.particleEmitterRooms = map[string]string{}
		}

		result.particleEmitterRooms[emitter.ID] = room
	}

	return nil
}

func appendScopedParticleEmitters(
	result *scopeResult,
	emitters []worldsource.ParticleEmitter,
	prefix string,
	transform affine,
	room string,
) error {
	for _, emitter := range emitters {
		if err := appendParticleEmitter(result, transformParticleEmitter(emitter, prefix, transform), room); err != nil {
			return err
		}
	}

	return nil
}

func mergeParticleEmitterRooms(result *scopeResult, rooms map[string]string) {
	for id, room := range rooms {
		if result.particleEmitterRooms == nil {
			result.particleEmitterRooms = map[string]string{}
		}

		result.particleEmitterRooms[id] = room
	}
}
