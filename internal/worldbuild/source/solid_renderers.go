package source

import (
	"fmt"
	"slices"

	sdkworld "github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"go.yaml.in/yaml/v3"
)

type SolidPrefab struct {
	ID     string
	Solids []worldsource.Solid
}

func validateSolidRendererPresence(document *yaml.Node) error {
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

	return visitSolidRendererPresence(root, version)
}

func visitSolidRendererPresence(node *yaml.Node, version uint16) error {
	if node.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(node.Content); index += 2 {
			if err := validateSolidRendererField(node.Content[index].Value, node.Content[index+1], version); err != nil {
				return err
			}
		}
	}

	for _, child := range node.Content {
		if err := visitSolidRendererPresence(child, version); err != nil {
			return err
		}
	}

	return nil
}

func validateSolidRendererField(key string, value *yaml.Node, version uint16) error {
	if key != "solid_renderer" && key != "renderable" {
		return nil
	}

	if version < worldsource.SolidRenderersVersion {
		return worldsource.ErrVersion
	}

	if value.Kind != yaml.ScalarNode ||
		key == "solid_renderer" && (value.Tag != "!!str" || value.Value == "") ||
		key == "renderable" && value.Tag != "!!bool" {
		return ErrYAML
	}

	return nil
}

func expandSolidRenderers(expanded *Expanded, document *worldsource.Document, prefabs map[string]*worldsource.Prefab) error {
	used := make(map[string]bool)

	for _, prefab := range document.Prefabs {
		if prefab.Renderable {
			used[prefab.ID] = true
		}
	}

	collect := func(contents []Content) {
		for _, content := range contents {
			if content.Actor != nil && content.Actor.SolidRenderer != "" {
				used[content.Actor.SolidRenderer] = true
			}
		}
	}
	collect(expanded.Contents)

	for _, room := range expanded.Rooms {
		collect(room.Contents)
	}

	if len(used) > sdkworld.MaxSolidPrefabs {
		return ErrExpansion
	}

	ids := make([]string, 0, len(used))
	for id := range used {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	total := 0

	for _, prefabID := range ids {
		prefab := prefabs[prefabID]
		if prefab == nil {
			return fmt.Errorf("renderer prefab %q: %w", prefabID, ErrExpansion)
		}

		result, err := (&expander{prefabs: prefabs}).expandScope(
			"",
			nil,
			prefab.Instances,
			nil,
			nil,
			prefab.Solids,
			nil,
			nil,
			affine{scale: 1},
			func(value string) string { return value },
			nil,
		)
		if err != nil {
			return err
		}

		total += len(result.solids)
		if total > sdkworld.MaxStaticSolids {
			return ErrExpansion
		}

		local := Expanded{Solids: result.solids}
		expandMaterialUV(&local, document.UV)
		expanded.SolidPrefabs = append(expanded.SolidPrefabs, SolidPrefab{ID: prefabID, Solids: local.Solids})
	}

	return nil
}
