//nolint:gocognit,nestif // Presence and inheritance walk the same small optional source vocabulary.
package source

import (
	"github.com/karty-game/karty-sdk/format/worldsource"
	"go.yaml.in/yaml/v3"
)

func validateMaterialPresence(document *yaml.Node) error {
	version := uint16(0)

	if len(document.Content) == 0 {
		return nil
	}

	root := document.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "version" {
			if err := root.Content[i+1].Decode(&version); err != nil {
				return ErrYAML
			}
		}
	}

	var visit func(*yaml.Node, bool) error

	visit = func(node *yaml.Node, inLayer bool) error {
		if inLayer && node.Tag == "!!null" {
			return ErrYAML
		}

		if node.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(node.Content); i += 2 {
				name, value := node.Content[i].Value, node.Content[i+1]
				layer := inLayer

				switch name {
				case "wall_bands", "bands", "floor_secondary", "ceiling_secondary", "wall_secondary", "secondary":
					if version < worldsource.MaterialsVersion {
						return worldsource.ErrVersion
					}

					layer = true
				}

				if err := visit(value, layer); err != nil {
					return err
				}
			}
		} else {
			for _, child := range node.Content {
				if err := visit(child, inLayer); err != nil {
					return err
				}
			}
		}

		return nil
	}

	return visit(root, false)
}

func transformBands(authored *worldsource.BandSettings, material func(string) string) *worldsource.BandSettings {
	result := worldsource.MergeBandSettings(nil, authored)
	if result != nil {
		for _, band := range []*worldsource.HorizontalBandSettings{result.Top, result.Bottom} {
			if band != nil && band.Texture != "" {
				band.Texture = material(band.Texture)
			}
		}
	}

	return result
}

func mergeSecondary(base, override *worldsource.SecondarySettings) *worldsource.SecondarySettings {
	if base == nil && override == nil {
		return nil
	}

	result := worldsource.SecondarySettings{}
	if base != nil {
		result = *base
	}

	if override != nil {
		if override.Enabled != nil {
			result.Enabled = override.Enabled
		}

		if override.Texture != "" {
			result.Texture = override.Texture
		}

		if override.Strength != nil {
			result.Strength = override.Strength
		}

		if override.UV != nil {
			if result.UV == nil {
				result.UV = inheritUV(worldsource.UVSettings{}, override.UV)
			} else {
				result.UV = inheritUV(*result.UV, override.UV)
			}
		}
	}

	return &result
}

func transformSecondary(authored *worldsource.SecondarySettings, material func(string) string) *worldsource.SecondarySettings {
	result := mergeSecondary(nil, authored)
	if result != nil && result.Texture != "" {
		result.Texture = material(result.Texture)
	}

	return result
}
