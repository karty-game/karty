package schema_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/schema"
)

const tinyWorld = `version: 6
rooms:
  - id: room
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 2, y: 0}, material: surface}
      - {id: east, start: {x: 2, y: 0}, end: {x: 0, y: 2}, material: surface}
      - {id: west, start: {x: 0, y: 2}, end: {x: 0, y: 0}, material: surface}
    ceiling: {c: 3}
    floor_material: surface
    ceiling_material: surface
prefabs:
  - id: marker
    contents:
      - id: sprite
        kind: prop
        actor:
          sprite: {texture: placeholder, facing: upright, alpha: cutout, width: 1, height: 1}
instances:
  - id: placed
    prefab: marker
    materials: [{from: placeholder, to: surface}]
`

func TestLevelSchemaAcceptsDefaultsAndDeclaredPrefabMaterials(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		tinyWorld,
		tinyWorld + "lighting: {version: 1, lights: null}\n",
		strings.Replace(tinyWorld, "    contents:", "    rooms: null\n    contents:", 1),
	} {
		value, err := schema.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}

		if err := schema.Validate(value, schema.Level(value, []string{"surface"})); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLevelSchemaRetainsReleasedSourceVersions(t *testing.T) {
	t.Parallel()

	rooms, _, _ := strings.Cut(tinyWorld, "prefabs:")
	for version := 1; version <= 6; version++ {
		input := strings.Replace(rooms, "version: 6", fmt.Sprintf("version: %d", version), 1)

		value, err := schema.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}

		if err := schema.Validate(value, schema.Level(value, []string{"surface"})); err != nil {
			t.Fatalf("source version %d: %v", version, err)
		}
	}
}

func TestLevelSchemaRejectsMistakesInEveryAuthoredScope(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"unknown root":            tinyWorld + "typo: true\n",
		"unknown nested":          strings.Replace(tinyWorld, "material: surface}", "material: surface, typo: true}", 1),
		"quoted version":          strings.Replace(tinyWorld, "version: 6", "version: '6'", 1),
		"quoted number":           strings.Replace(tinyWorld, "width: 1", "width: '1'", 1),
		"numeric identifier":      strings.Replace(tinyWorld, "id: placed", "id: 123", 1),
		"unknown material":        strings.Replace(tinyWorld, "floor_material: surface", "floor_material: typo", 1),
		"unknown prefab sprite":   strings.Replace(tinyWorld, "texture: placeholder", "texture: typo", 1),
		"unknown prefab":          strings.Replace(tinyWorld, "prefab: marker", "prefab: typo", 1),
		"unknown override target": strings.Replace(tinyWorld, "to: surface", "to: typo", 1),
		"unknown sprite mode":     strings.Replace(tinyWorld, "facing: upright", "facing: typo", 1),
		"bounds":                  strings.Replace(tinyWorld, "width: 1", "width: 1000001", 1),
		"version gate":            strings.Replace(tinyWorld, "version: 6", "version: 5", 1),
		"null v6 list":            tinyWorld + "solids: null\n",
		"horizontal wrap":         strings.Replace(tinyWorld, "    ceiling:", "    floor_uv: {mode: wrap}\n    ceiling:", 1),
		"missing reference pair":  tinyWorld + "connections: [{id: portal, a: {room: room}, b: {instance: placed, port: entry}}]\n",
		"both reference pairs":    tinyWorld + "connections: [{id: portal, a: {room: room, edge: east, instance: placed, port: entry}, b: {room: room, edge: west}}]\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			value, err := schema.Parse([]byte(input))
			if err != nil {
				t.Fatal(err)
			}

			if err := schema.Validate(value, schema.Level(value, []string{"surface"})); !errors.Is(err, schema.ErrValidation) {
				t.Fatalf("accepted malformed world: %v", err)
			}
		})
	}
}

func TestYAMLParsingRejectsAmbiguousAndUnboundedInputs(t *testing.T) {
	t.Parallel()

	for name, input := range map[string]string{
		"empty":              "",
		"duplicate":          "version: 6\nversion: 6\n",
		"multiple documents": "version: 6\n---\nversion: 6\n",
		"anchor":             "version: 6\nrooms: &rooms []\n",
		"alias":              "version: 6\nrooms: &rooms []\nprefabs: *rooms\n",
		"nonstring key":      "version: 6\n1: value\n",
		"nonfinite":          "version: 6\nvalue: .nan\n",
		"custom tag":         "version: 6\nvalue: !custom test\n",
		"depth":              "version: 6\nvalue: " + strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := schema.Parse([]byte(input)); !errors.Is(err, schema.ErrValidation) {
				t.Fatalf("accepted invalid YAML: %v", err)
			}
		})
	}
}

func TestDirectBandSchemaTextureChoicesAndLegacyRejection(t *testing.T) {
	t.Parallel()

	rooms, _, _ := strings.Cut(tinyWorld, "prefabs:")
	input := strings.Replace(rooms, "version: 6", "version: 7", 1)

	input = strings.Replace(
		input,
		"    ceiling:",
		"    wall_bands: {top: {texture: trim, height: 0.5}, bottom: {texture: surface, height: 0.25}}\n    ceiling:",
		1,
	)
	for name, fixture := range map[string]string{
		"valid":           input,
		"unknown texture": strings.Replace(input, "texture: trim", "texture: missing", 1),
		"null band":       strings.Replace(input, "top: {texture: trim, height: 0.5}", "top: null", 1),
		"null enabled":    strings.Replace(input, "height: 0.5", "height: 0.5, enabled: null", 1),
		"null bands":      strings.Replace(input, "wall_bands: {top: {texture: trim, height: 0.5}, bottom: {texture: surface, height: 0.25}}", "wall_bands: null", 1),
		"legacy frame":    strings.Replace(input, "top: {texture: trim, height: 0.5}", "frame: trim", 1),
		"vertical":        strings.Replace(input, "top: {texture: trim, height: 0.5}", "vertical: {width: 0.2}", 1),
		"old source":      strings.Replace(input, "version: 7", "version: 6", 1),
	} {
		value, err := schema.Parse([]byte(fixture))
		if err != nil {
			t.Fatal(err)
		}

		err = schema.Validate(value, schema.Level(value, []string{"surface", "trim"}))
		if (name == "valid") != (err == nil) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
