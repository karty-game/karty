package source

import (
	"github.com/karty-game/karty-sdk/format/worldsource"
	"go.yaml.in/yaml/v3"
)

// Preserve source version and null gates before decoding optional values.
func validateAnimationPresence(document *yaml.Node) error {
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

	return visitAnimationPresence(root, version)
}

func visitAnimationPresence(node *yaml.Node, version uint16) error {
	if node.Kind == yaml.MappingNode {
		if err := validateAnimationMapping(node, version); err != nil {
			return err
		}
	}

	for _, child := range node.Content {
		if err := visitAnimationPresence(child, version); err != nil {
			return err
		}
	}

	return nil
}

func validateAnimationMapping(node *yaml.Node, version uint16) error {
	kind := ""

	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "kind" {
			kind = node.Content[i+1].Value
		}
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		name, value := node.Content[i].Value, node.Content[i+1]
		if version < worldsource.AnimationsVersion && (name == "animation" || name == "animations") {
			return worldsource.ErrVersion
		}

		switch name {
		case "surface_amplitude", "opacity", "pixel_size":
			if kind != "liquid" || value.Tag == "!!null" {
				return worldsource.ErrContent
			}
		}
	}

	return nil
}
