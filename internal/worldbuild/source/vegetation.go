package source

import (
	"fmt"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
	"go.yaml.in/yaml/v3"
)

// YAML presence checks preserve authored zero density and zero seeds while
// rejecting omitted required fields and explicit nulls before Go decoding.
func validateVegetationPresence(document *yaml.Node) error {
	if len(document.Content) == 0 {
		return nil
	}

	root := document.Content[0]

	var version uint16
	if node := particleYAMLField(root, "version"); node != nil {
		if err := node.Decode(&version); err != nil {
			return ErrYAML
		}
	}

	return visitVegetationPresence(root, version)
}

func visitVegetationPresence(node *yaml.Node, version uint16) error {
	if node.Kind == yaml.MappingNode {
		if err := validateVegetationMapping(node, version); err != nil {
			return err
		}
	}

	for _, child := range node.Content {
		if err := visitVegetationPresence(child, version); err != nil {
			return err
		}
	}

	return nil
}

func validateVegetationMapping(node *yaml.Node, version uint16) error {
	for i := 0; i+1 < len(node.Content); i += 2 {
		name, value := node.Content[i].Value, node.Content[i+1]
		if name != "vegetation" && name != "vegetation_layers" {
			continue
		}

		if version < worldsource.VegetationVersion {
			return worldsource.ErrVersion
		}

		if err := rejectParticleNulls(value); err != nil {
			return err
		}

		kind := "Vegetation"
		if name == "vegetation_layers" {
			kind = "[]VegetationLayer"
		}

		if err := validateVegetationFields(value, kind); err != nil {
			return err
		}
	}

	return nil
}

func validateVegetationFields(node *yaml.Node, kind string) error {
	if kind == "[]VegetationLayer" || kind == "[]VegetationPreset" {
		if node.Kind != yaml.SequenceNode {
			return worldsource.ErrContent
		}

		for _, child := range node.Content {
			if err := validateVegetationFields(child, kind[2:]); err != nil {
				return err
			}
		}

		return nil
	}

	if node.Kind != yaml.MappingNode {
		return worldsource.ErrContent
	}

	if kind == "Vegetation" {
		if err := validateVegetationNormalPresence(node); err != nil {
			return err
		}
	}

	required := map[string][]string{
		"Vegetation":       {"version", "presets"},
		"VegetationPreset": {"name", "texture", "width", "height", "wind_amplitude", "wind_frequency", "draw_distance"},
		"VegetationLayer":  {"preset", "density", "seed", "surface"},
		"Range":            {"min", "max"},
	}[kind]
	for _, field := range required {
		if particleYAMLField(node, field) == nil {
			return fmt.Errorf("vegetation %s requires %q: %w", kind, field, worldsource.ErrContent)
		}
	}

	for _, field := range []string{"presets", "width", "height"} {
		if child := particleYAMLField(node, field); child != nil {
			childKind := "Range"
			if field == "presets" {
				childKind = "[]VegetationPreset"
			}

			if err := validateVegetationFields(child, childKind); err != nil {
				return err
			}
		}
	}

	return nil
}

func appendVegetationLayerCount(result *scopeResult, count int) error {
	if count > worldvegetation.MaxLayers-result.vegetationLayerCount {
		return ErrExpansion
	}

	result.vegetationLayerCount += count

	return nil
}

func validateVegetationNormalPresence(node *yaml.Node) error {
	var version uint16
	if field := particleYAMLField(node, "version"); field != nil {
		if err := field.Decode(&version); err != nil {
			return ErrYAML
		}
	}

	if version >= worldvegetation.NormalVersion {
		return nil
	}

	presets := particleYAMLField(node, "presets")
	if presets == nil {
		return nil
	}

	for _, preset := range presets.Content {
		if particleYAMLField(preset, "normal_texture") != nil {
			return worldsource.ErrVersion
		}
	}

	return nil
}
