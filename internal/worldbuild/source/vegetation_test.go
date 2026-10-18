package source_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

const vegetationSource = `version: 13
vegetation:
  version: 1
  presets:
    - {name: grass, texture: wall, width: {min: 0.2, max: 0.4}, height: {min: 0.2, max: 0.5}, wind_amplitude: 0.08, wind_frequency: 0.7, draw_distance: 32}
rooms:
  - id: room
    ceiling: {c: 6}
    floor_material: wall
    ceiling_material: wall
    vegetation_layers:
      - {preset: grass, density: 4, seed: 0, surface: floor}
    boundary:
      - {id: a, start: {x: 0, y: 0}, end: {x: 10, y: 0}, material: wall}
      - {id: b, start: {x: 10, y: 0}, end: {x: 10, y: 10}, material: wall}
      - {id: c, start: {x: 10, y: 10}, end: {x: 0, y: 10}, material: wall}
      - {id: d, start: {x: 0, y: 10}, end: {x: 0, y: 0}, material: wall}
`

func TestVegetationPresenceAndPrefabRoomLayers(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ from, to string }{
		{"seed: 0, ", ""},
		{"name: grass, ", ""}, {"texture: wall, ", ""},
		{"width: {min: 0.2, max: 0.4}, ", ""}, {"height: {min: 0.2, max: 0.5}, ", ""},
		{"min: 0.2, ", ""}, {", max: 0.4", ""},
		{"wind_frequency: 0.7, ", ""}, {", draw_distance: 32", ""}, {"preset: grass, ", ""}, {"density: 4, ", ""}, {"surface: floor", "surface: null"}, {"density: 4", "density: 17"}, {"wind_amplitude: 0.08, ", ""}, {"preset: grass", "preset: absent"}, {"width: {min: 0.2, max: 0.4}", "width: {min: 0.4, max: 0.2}"},
	} {
		if _, err := source.Decode([]byte(strings.Replace(vegetationSource, test.from, test.to, 1))); err == nil {
			t.Fatalf("accepted %s", test.to)
		}
	}

	for version := 1; version < 13; version++ {
		input := fmt.Sprintf("version: %d\nvegetation: null\n", version)
		if _, err := source.Decode([]byte(input)); !errors.Is(err, worldsource.ErrVersion) {
			t.Fatalf("v%d: %v", version, err)
		}
	}

	for _, input := range []string{"version: 13\nvegetation: null\n", strings.Replace(vegetationSource, "vegetation_layers:", "vegetation_layers: null\n    unused:", 1)} {
		if _, err := source.Decode([]byte(input)); err == nil {
			t.Fatal("null accepted")
		}
	}

	expanded, err := source.Decode([]byte(strings.Replace(vegetationSource, "density: 4", "density: 0", 1)))
	if err != nil || expanded.Rooms[0].VegetationLayers[0].Seed != 0 {
		t.Fatalf("explicit zero rejected: %v", err)
	}

	header, rooms, _ := strings.Cut(vegetationSource, "rooms:\n")
	input := header + "prefabs:\n  - id: patch\n    rooms:\n    " + strings.ReplaceAll(
		rooms,
		"\n",
		"\n    ",
	) + "\ninstances:\n  - {id: placed, prefab: patch, transform: {translation: {x: 20}, scale: 2}}\n"

	expanded, err = source.Decode([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	if expanded.Rooms[0].ID != "placed/room" || expanded.Rooms[0].VegetationLayers[0].Density != 4 {
		t.Fatalf("room layer lost: %+v", expanded.Rooms)
	}
}

func TestVegetationExpandedLayerBound(t *testing.T) {
	t.Parallel()

	header, rooms, _ := strings.Cut(vegetationSource, "rooms:\n")

	var input strings.Builder
	input.WriteString(header + "prefabs:\n  - id: patch\n    rooms:\n    " + strings.ReplaceAll(rooms, "\n", "\n    ") + "\ninstances:\n")

	for index := range 1025 {
		fmt.Fprintf(&input, "  - {id: patch%d, prefab: patch}\n", index)
	}

	if _, err := source.Decode([]byte(input.String())); !errors.Is(err, source.ErrExpansion) {
		t.Fatalf("expanded layer budget lost: %v", err)
	}
}

func TestVegetationNormalPresenceAndVersionGate(t *testing.T) {
	t.Parallel()

	base := strings.Replace(vegetationSource, "version: 13", "version: 15", 1)
	base = strings.Replace(base, "version: 1\n", "version: 3\n", 1)
	base = strings.Replace(base, "texture: wall,", "texture: wall, normal_texture: normal,", 1)

	for _, test := range []struct {
		from, to string
		valid    bool
	}{
		{"", "", true}, {"version: 15", "version: 14", false}, {"version: 3", "version: 2", false}, {"normal_texture: normal", "normal_texture: null", false}, {"normal_texture: normal", "normal_texture: ''", true},
	} {
		changed := base
		if test.from != "" {
			changed = strings.Replace(changed, test.from, test.to, 1)
		}

		_, err := source.Decode([]byte(changed))
		if (err == nil) != test.valid {
			t.Fatalf("%s: %v", test.to, err)
		}
	}

	legacyEmpty := strings.Replace(base, "version: 3", "version: 2", 1)

	legacyEmpty = strings.Replace(legacyEmpty, "normal_texture: normal", "normal_texture: ''", 1)
	if _, err := source.Decode([]byte(legacyEmpty)); !errors.Is(err, worldsource.ErrVersion) {
		t.Fatal("legacy explicit empty normal accepted", err)
	}
}

func TestPalmSourcePresenceAndVersionGate(t *testing.T) {
	t.Parallel()

	base := strings.Replace(vegetationSource, "version: 13", "version: 20", 1)
	base = strings.Replace(base, "version: 1\n", "version: 5\n", 1)

	base = strings.Replace(base, "name: grass,", "name: grass, geometry: palm,", 1)
	for _, test := range []struct {
		from, to string
		valid    bool
	}{
		{"", "", true}, {"version: 20", "version: 19", false}, {"version: 5", "version: 4", false},
		{"geometry: palm", "geometry: null", false}, {"geometry: palm", "geometry: coconut", false},
		{"geometry: palm", "geometry: tree", true},
	} {
		input := base
		if test.from != "" {
			input = strings.Replace(base, test.from, test.to, 1)
		}

		_, err := source.Decode([]byte(input))
		if (err == nil) != test.valid {
			t.Fatalf("%s: %v", test.to, err)
		}
	}
}
