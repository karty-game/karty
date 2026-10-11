package source

import (
	"fmt"
	"strings"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"go.yaml.in/yaml/v3"
)

type particleYAMLFields struct {
	required []string
	children map[string]string
}

// These are the particle schema's presence rules. Go zero values cannot prove
// that required fields such as a zero-age key, zero-position or seed were authored.
func particleYAMLObject(kind string) particleYAMLFields {
	switch kind {
	case "Particles":
		return particleYAMLFields{
			required: []string{"version", "presets"},
			children: map[string]string{"presets": "[]ParticlePreset", "emitters": "[]ParticleEmitter"},
		}
	case "ParticlePreset":
		return particleYAMLFields{
			required: []string{"name", "capacity", "emission", "lifetime_seconds", "spawn", "sprite", "size_over_life", "color_over_life"},
			children: map[string]string{
				"emission":               "Emission",
				"lifetime_seconds":       "Range",
				"spawn":                  "ParticleSpawn",
				"sprite":                 "ParticleSprite",
				"size_over_life":         "[]ScalarKey",
				"color_over_life":        "[]ColorKey",
				"rotation_degrees":       "Range",
				"rotation_speed_degrees": "Range",
			},
		}
	case "ParticleEmitter":
		return particleYAMLFields{
			required: []string{"id", "preset", "position", "seed", "playback"},
			children: map[string]string{"playback": "Playback"},
		}
	case "Emission":
		return particleYAMLFields{children: map[string]string{"bursts": "[]Burst"}}
	case "Range":
		return particleYAMLFields{required: []string{"min", "max"}}
	case "ParticleSpawn":
		return particleYAMLFields{required: []string{"shape"}}
	case "ParticleSprite":
		return particleYAMLFields{required: []string{"frames", "playback", "facing", "blend"}}
	case "ScalarKey":
		return particleYAMLFields{required: []string{"age", "value"}}
	case "ColorKey":
		return particleYAMLFields{required: []string{"age", "rgba"}}
	case "Burst":
		return particleYAMLFields{required: []string{"at_seconds", "count"}}
	case "Playback":
		return particleYAMLFields{required: []string{"mode"}}
	default:
		return particleYAMLFields{}
	}
}

func validateParticleYAMLFields(node *yaml.Node, kind string) error {
	if itemKind, sequence := strings.CutPrefix(kind, "[]"); sequence {
		if node.Kind != yaml.SequenceNode {
			return worldsource.ErrContent
		}

		for _, item := range node.Content {
			if err := validateParticleYAMLFields(item, itemKind); err != nil {
				return err
			}
		}

		return nil
	}

	if node.Kind != yaml.MappingNode {
		return worldsource.ErrContent
	}

	definition := particleYAMLObject(kind)
	if len(definition.required) == 0 && len(definition.children) == 0 {
		return worldsource.ErrContent
	}

	for _, name := range definition.required {
		if particleYAMLField(node, name) == nil {
			return fmt.Errorf("particle %s requires %q: %w", kind, name, worldsource.ErrContent)
		}
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		if childKind := definition.children[node.Content[i].Value]; childKind != "" {
			if err := validateParticleYAMLFields(node.Content[i+1], childKind); err != nil {
				return err
			}
		}
	}

	return nil
}

func particleYAMLField(node *yaml.Node, name string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == name {
			return node.Content[i+1]
		}
	}

	return nil
}
