package source

import (
	"github.com/karty-game/karty-sdk/format/worldsource"
	"go.yaml.in/yaml/v3"
)

// Presence is checked before decoding, so false and null cannot bypass source
// version gates and explicit null never silently selects an environment default.
func validateEnvironmentPresence(document *yaml.Node) error {
	if len(document.Content) == 0 {
		return nil
	}

	root := document.Content[0]

	var version uint16

	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "version" {
			if err := root.Content[i+1].Decode(&version); err != nil {
				return ErrYAML
			}
		}
	}

	return visitEnvironmentPresence(root, version)
}

func visitEnvironmentPresence(node *yaml.Node, version uint16) error {
	if node.Kind == yaml.MappingNode {
		if err := validateEnvironmentMapping(node, version); err != nil {
			return err
		}
	}

	for _, child := range node.Content {
		if err := visitEnvironmentPresence(child, version); err != nil {
			return err
		}
	}

	return nil
}

func validateEnvironmentMapping(node *yaml.Node, version uint16) error {
	for i := 0; i+1 < len(node.Content); i += 2 {
		name, value := node.Content[i].Value, node.Content[i+1]
		if name != "environment" && name != "sky" {
			continue
		}

		if version < worldsource.EnvironmentVersion {
			return worldsource.ErrVersion
		}

		if value.Tag == "!!null" {
			return ErrYAML
		}

		if name == "environment" {
			if err := validateEnvironmentNode(value); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateEnvironmentNode(node *yaml.Node) error {
	for i := 0; i+1 < len(node.Content); i += 2 {
		name, value := node.Content[i].Value, node.Content[i+1]
		if value.Tag == "!!null" {
			return ErrYAML
		}

		if name == "sun" {
			if err := validateEnvironmentSunNode(value); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateEnvironmentSunNode(node *yaml.Node) error {
	azimuth, elevation := false, false

	for i := 0; i+1 < len(node.Content); i += 2 {
		name, value := node.Content[i].Value, node.Content[i+1]
		if value.Tag == "!!null" {
			return ErrYAML
		}

		azimuth = azimuth || name == "azimuth"
		elevation = elevation || name == "elevation"
	}

	if !azimuth || !elevation {
		return ErrYAML
	}

	return nil
}
