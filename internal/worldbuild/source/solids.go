package source

import (
	"github.com/karty-game/karty-sdk/format/worldsource"
	"go.yaml.in/yaml/v3"
)

func transformSolid(authored worldsource.Solid, prefix string, transform affine, material func(string) string) worldsource.Solid {
	result := authored
	result.ID = joinIdentity(prefix, authored.ID)

	result.Footprint = make([]worldsource.Vec2, len(authored.Footprint))
	for index, point := range authored.Footprint {
		result.Footprint[index] = transformPoint(point, transform)
	}

	result.Bottom, result.Top = transformPlane(authored.Bottom, transform), transformPlane(authored.Top, transform)
	result.SideMaterial, result.TopMaterial, result.BottomMaterial = material(
		authored.SideMaterial,
	), material(
		authored.TopMaterial,
	), material(
		authored.BottomMaterial,
	)

	return result
}

func transformLooseContent(
	authored worldsource.Content,
	prefix string,
	transform affine,
	material func(string) string,
	tags []string,
) Content {
	result := Content{
		ID:       joinIdentity(prefix, authored.ID),
		SourceID: authored.ID,
		Instance: prefix,
		Kind:     authored.Kind,
		Position: transformPoint3(authored.Position, transform),
		Actor:    transformActor(authored.Actor, transform, tags),
	}
	if result.Actor != nil && result.Actor.Sprite != nil {
		result.Actor.Sprite.Texture = material(result.Actor.Sprite.Texture)
	}

	return result
}

// Extra top-level lists are source-v6 fields even when authored as [] or null.
// Typed validation cannot distinguish an omitted list from explicit YAML null.
func validateExtrasPresence(document *yaml.Node) error {
	if len(document.Content) == 0 {
		return nil
	}

	root := document.Content[0]
	version := uint16(0)

	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value == "version" {
			if err := root.Content[index+1].Decode(&version); err != nil {
				return ErrYAML
			}
		}
	}

	if err := validateScopeExtras(root, version); err != nil {
		return err
	}

	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value == "prefabs" {
			for _, prefab := range root.Content[index+1].Content {
				if err := validateScopeExtras(prefab, version); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func validateScopeExtras(scope *yaml.Node, version uint16) error {
	if scope.Kind != yaml.MappingNode {
		return nil
	}

	for index := 0; index+1 < len(scope.Content); index += 2 {
		field, value := scope.Content[index].Value, scope.Content[index+1]
		if field == "solids" || field == "contents" {
			if version < worldsource.SolidsVersion {
				return worldsource.ErrVersion
			}

			if value.Kind != yaml.SequenceNode {
				return ErrYAML
			}
		}
	}

	return nil
}
