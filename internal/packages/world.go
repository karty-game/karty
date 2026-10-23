package packages

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/assetpipeline"
	worldschema "github.com/karty-game/karty/internal/worldbuild/schema"
	"github.com/karty-game/karty/internal/worldbuild/source"
	"go.yaml.in/yaml/v3"
)

// V1 exports static solids, nested prefab instances, and PNG textures.
// Other reference kinds require explicit namespace/closure semantics first.
func fragment(ctx context.Context, root *os.Root, definition Manifest) ([]map[string]any, error) {
	if definition.Prefabs == "" {
		return nil, nil
	}

	data, err := Read(ctx, root, definition.Prefabs, world.MaxEncodedSize)
	if err != nil {
		return nil, err
	}

	value, err := worldschema.Parse(data)
	if err != nil {
		return nil, err
	}

	object, isMapping := value.(map[string]any)
	if !isMapping || len(object) != 1 || object["prefabs"] == nil {
		return nil, fmt.Errorf("%w: prefab file must contain only a prefabs list", ErrPackage)
	}

	items, isMapping := object["prefabs"].([]any)
	if !isMapping || len(items) == 0 || len(items) > 256 {
		return nil, ErrPackage
	}

	result := records(items)
	if len(result) != len(items) {
		return nil, ErrPackage
	}

	seen := map[string]bool{}

	for _, prefab := range result {
		id, _ := prefab["id"].(string)
		if !namePattern.MatchString(id) || seen[id] {
			return nil, fmt.Errorf("%w: invalid or duplicate prefab %q", ErrPackage, id)
		}

		seen[id] = true

		for key := range prefab {
			if key != "id" && key != "solids" && key != "instances" && key != "renderable" {
				return nil, fmt.Errorf("%w: package format 1 does not support prefab field %q", ErrPackage, key)
			}
		}
	}

	return result, nil
}

// Validate complete exports even when no consuming level selects them. The
// synthetic room exists only in memory to use the existing full-world validators.
func validatePrefabs(ctx context.Context, root *os.Root, definition Manifest) error {
	prefabs, err := fragment(ctx, root, definition)
	if err != nil || len(prefabs) == 0 {
		return err
	}

	value, err := worldschema.Parse([]byte(`version: 11
rooms:
  - id: validation
    floor: {c: 0}
    ceiling: {c: 4}
    floor_material: validation-surface
    ceiling_material: validation-surface
    boundary:
      - {id: a, start: {x: 0, y: 0}, end: {x: 1, y: 0}, material: validation-surface}
      - {id: b, start: {x: 1, y: 0}, end: {x: 1, y: 1}, material: validation-surface}
      - {id: c, start: {x: 1, y: 1}, end: {x: 0, y: 1}, material: validation-surface}
      - {id: d, start: {x: 0, y: 1}, end: {x: 0, y: 0}, material: validation-surface}
`))
	if err != nil {
		return err
	}

	object, ok := value.(map[string]any)
	if !ok {
		return ErrPackage
	}

	items := make([]any, 0, len(prefabs))
	for _, prefab := range prefabs {
		items = append(items, prefab)
	}

	object["prefabs"] = items

	names := []string{"validation-surface"}
	for _, texture := range definition.Textures {
		names = append(names, texture.Name)
	}

	if err := worldschema.Validate(object, worldschema.Level(object, names)); err != nil {
		return fmt.Errorf("%s: %w", definition.Prefabs, err)
	}

	data, err := yaml.Marshal(object)
	if err != nil {
		return err
	}

	_, err = source.Decode(data)

	return err
}

type WorldTexture struct {
	Name, Profile string
	Data          []byte
	Materialize   *assetpipeline.MaterializeOptions
}

// ResolveWorld merges selected static prefab exports into an ordinary authored
// world. Scalar types/null presence survive until the existing schema checks.
func ResolveWorld(ctx context.Context, set *Set, contents []byte, localTextures []string) ([]byte, []WorldTexture, error) {
	value, err := worldschema.Parse(contents)
	if err != nil {
		return nil, nil, err
	}

	object, ok := value.(map[string]any)
	if !ok {
		return nil, nil, ErrPackage
	}

	prefabs, textures, err := collectWorldExports(ctx, set)
	if err != nil {
		return nil, nil, err
	}

	if err := selectWorldPrefabs(object, prefabs); err != nil {
		return nil, nil, err
	}

	imported, allNames, err := selectWorldTextures(ctx, object, textures, localTextures)
	if err != nil {
		return nil, nil, err
	}

	if err := worldschema.Validate(object, worldschema.Level(object, allNames)); err != nil {
		return nil, nil, err
	}

	merged, err := yaml.Marshal(object)
	if err != nil {
		return nil, nil, err
	}

	if len(merged) > world.MaxEncodedSize {
		return nil, nil, ErrPackage
	}

	if _, err := source.Decode(merged); err != nil {
		return nil, nil, err
	}

	return merged, imported, nil
}

func records(value any) []map[string]any {
	var result []map[string]any

	if items, ok := value.([]any); ok {
		for _, item := range items {
			if object, ok := item.(map[string]any); ok {
				result = append(result, object)
			}
		}
	}

	return result
}
func materialKey(key string) bool {
	switch key {
	case "floor_material", "ceiling_material", "material", "side_material", "top_material", "bottom_material", "from", "to":
		return true
	}

	return false
}
func visit(value any, transform func(string, any) any) {
	switch object := value.(type) {
	case map[string]any:
		for key, value := range object {
			value = transform(key, value)
			object[key] = value
			visit(value, transform)
		}
	case []any:
		for _, value := range object {
			visit(value, transform)
		}
	}
}

type textureExport struct {
	texture Texture
	item    *Resolved
}

func collectWorldExports(ctx context.Context, set *Set) (map[string]map[string]any, map[string]textureExport, error) {
	prefabs := map[string]map[string]any{}

	textures := map[string]textureExport{}

	for _, packageName := range set.Names() {
		item := set.Items[packageName]

		items, err := fragment(ctx, item.Root, item.Manifest)
		if err != nil {
			return nil, nil, fmt.Errorf("package %s: %w", packageName, err)
		}

		for _, prefab := range items {
			localID, _ := prefab["id"].(string)
			qualifiedID := packageName + "." + localID
			prefab["id"] = qualifiedID
			visit(prefab, func(key string, value any) any {
				if key == "prefab" || materialKey(key) {
					if name, ok := value.(string); ok {
						return packageName + "." + name
					}
				}

				return value
			})
			prefabs[qualifiedID] = prefab
		}

		for _, texture := range item.Manifest.Textures {
			textures[packageName+"."+texture.Name] = textureExport{texture, item}
		}
	}

	return prefabs, textures, nil
}

func selectWorldPrefabs(object map[string]any, prefabs map[string]map[string]any) error {
	selected := map[string]bool{}

	locals := records(object["prefabs"])
	if raw, exists := object["prefabs"]; exists && raw != nil {
		if _, isList := raw.([]any); !isList {
			return fmt.Errorf("%w: local prefabs must be a list", ErrPackage)
		}
	}

	if raw, ok := object["prefabs"].([]any); ok && len(raw) != len(locals) {
		return fmt.Errorf("%w: invalid local prefab record", ErrPackage)
	}

	for _, prefab := range locals {
		prefabID, _ := prefab["id"].(string)
		if _, exists := prefabs[prefabID]; exists {
			return fmt.Errorf("%w: prefab collision %s", ErrPackage, prefabID)
		}

		prefabs[prefabID] = prefab
		selected[prefabID] = true
	}

	var selectInstances func(map[string]any) error

	selectInstances = func(scope map[string]any) error {
		for _, instance := range records(scope["instances"]) {
			name, _ := instance["prefab"].(string)
			if selected[name] {
				continue
			}

			prefab, exists := prefabs[name]
			if !exists {
				return fmt.Errorf("%w: unknown prefab %q", ErrPackage, name)
			}

			selected[name] = true

			if err := selectInstances(prefab); err != nil {
				return err
			}
		}

		return nil
	}
	if err := selectInstances(object); err != nil {
		return err
	}

	for _, prefab := range locals {
		if err := selectInstances(prefab); err != nil {
			return err
		}
	}

	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}

	slices.Sort(names)

	items := make([]any, 0, len(names))
	for _, name := range names {
		items = append(items, prefabs[name])
	}

	if len(items) > 0 {
		object["prefabs"] = items
	}

	return nil
}

func selectWorldTextures(
	ctx context.Context,
	object map[string]any,
	textures map[string]textureExport,
	localTextures []string,
) ([]WorldTexture, []string, error) {
	needed := map[string]bool{}

	visit(object, func(key string, value any) any {
		if materialKey(key) {
			if name, ok := value.(string); ok {
				needed[name] = true
			}
		}

		// Actor sprites select PNG exports just like surfaces. Keep this scoped
		// to actor.sprite.texture rather than treating every texture-like key
		// as a package reference.
		if key == "actor" {
			actor, _ := value.(map[string]any)
			sprite, _ := actor["sprite"].(map[string]any)

			if name, ok := sprite["texture"].(string); ok {
				needed[name] = true
			}
		}

		return value
	})

	for _, name := range localTextures {
		if _, exists := textures[name]; exists {
			return nil, nil, fmt.Errorf("%w: texture collision %s", ErrPackage, name)
		}
	}

	var names []string

	for name := range needed {
		if _, exists := textures[name]; exists {
			names = append(names, name)
		}
	}

	slices.Sort(names)

	var (
		imported []WorldTexture
		total    int
	)

	for _, name := range names {
		entry := textures[name]

		data, err := Read(ctx, entry.item.Root, entry.texture.Source, MaxFileBytes)
		if err != nil {
			return nil, nil, err
		}

		total += len(data)
		if total > MaxTotalBytes {
			return nil, nil, ErrPackage
		}

		imported = append(imported, WorldTexture{
			Name: name, Profile: entry.texture.Profile, Data: data, Materialize: entry.texture.Materialize,
		})
	}

	allNames := append(slices.Clone(localTextures), names...)

	return imported, allNames, nil
}
