package schema_test

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/schema"
)

func TestVegetationSchemaVersionPresenceAndCatalog(t *testing.T) {
	t.Parallel()

	input := strings.Replace(tinyWorld, "version: 6", "version: 13", 1)
	input = strings.Replace(
		input,
		"    ceiling: {c: 3}",
		"    ceiling: {c: 3}\n    vegetation_layers: [{preset: grass, density: 4, seed: 0, surface: floor}]",
		1,
	)

	input += "vegetation:\n  version: 1\n  presets:\n    - {name: grass, texture: surface, width: {min: 0.2, max: 0.4}, height: {min: 0.2, max: 0.5}, wind_amplitude: 0.08, wind_frequency: 0.7, draw_distance: 32}\n"
	for _, test := range []struct {
		from, to string
		valid    bool
	}{
		{"", "", true}, {"version: 13", "version: 12", false}, {"seed: 0, ", "", false}, {"surface: floor", "surface: wall", false}, {"texture: surface", "texture: missing", false}, {"preset: grass", "preset: missing", false}, {"density: 4", "density: 0", true}, {"density: 4", "density: 17", false}, {"wind_frequency: 0.7, ", "", false},
	} {
		changed := input
		if test.from != "" {
			changed = strings.Replace(changed, test.from, test.to, 1)
		}

		value, err := schema.Parse([]byte(changed))
		if err != nil {
			t.Fatal(err)
		}

		err = schema.Validate(value, schema.Level(value, []string{"surface"}))
		if (err == nil) != test.valid {
			t.Fatalf("%s: %v", test.to, err)
		}
	}
}

func TestShrubGeometrySchemaVersionGate(t *testing.T) {
	t.Parallel()

	base := strings.Replace(tinyWorld, "version: 6", "version: 14", 1)
	base += "vegetation:\n  version: 2\n  presets:\n    - {name: bush, geometry: shrub, texture: surface, width: {min: 1, max: 2}, height: {min: 1, max: 2}, wind_amplitude: 0.08, wind_frequency: 0.7, draw_distance: 32}\n"

	for _, test := range []struct {
		from, to string
		valid    bool
	}{
		{"", "", true}, {"version: 14", "version: 13", false}, {"version: 2", "version: 1", false}, {"geometry: shrub", "geometry: tree", false}, {"geometry: shrub", "geometry: crossed_cards", true}, {"geometry: shrub, ", "", true},
	} {
		changed := base
		if test.from != "" {
			changed = strings.Replace(base, test.from, test.to, 1)
		}

		value, err := schema.Parse([]byte(changed))
		if err != nil {
			t.Fatal(err)
		}

		err = schema.Validate(value, schema.Level(value, []string{"surface"}))
		if (err == nil) != test.valid {
			t.Fatalf("%s: %v", test.to, err)
		}
	}
}

func TestVegetationNormalSchemaVersionAndCatalog(t *testing.T) {
	t.Parallel()

	base := strings.Replace(tinyWorld, "version: 6", "version: 15", 1)
	base += "vegetation:\n  version: 3\n  presets:\n    - {name: grass, texture: surface, normal_texture: normal, width: {min: 1, max: 2}, height: {min: 1, max: 2}, wind_amplitude: 0.08, wind_frequency: 0.7, draw_distance: 32}\n"

	for _, test := range []struct {
		from, to string
		valid    bool
	}{
		{"", "", true}, {"version: 15", "version: 14", false}, {"version: 3", "version: 2", false}, {"normal_texture: normal", "normal_texture: missing", false}, {"normal_texture: normal", "normal_texture: ''", true}, {"normal_texture: normal, ", "", true},
	} {
		changed := base
		if test.from != "" {
			changed = strings.Replace(base, test.from, test.to, 1)
		}

		value, err := schema.Parse([]byte(changed))
		if err != nil {
			t.Fatal(err)
		}

		err = schema.Validate(value, schema.Level(value, []string{"surface", "normal"}))
		if (err == nil) != test.valid {
			t.Fatalf("%s: %v", test.to, err)
		}
	}
}

func TestTreeSchemaSourceAndGeometryGate(t *testing.T) {
	t.Parallel()

	base := strings.Replace(tinyWorld, "version: 6", "version: 16", 1)
	base += "vegetation:\n  version: 4\n  presets:\n    - {name: tree, geometry: tree, texture: surface, normal_texture: normal, width: {min: 1, max: 2}, height: {min: 1, max: 2}, wind_amplitude: 0.08, wind_frequency: 0.7, draw_distance: 32}\n"

	for _, test := range []struct {
		from, to string
		valid    bool
	}{
		{"", "", true}, {"version: 16", "version: 15", false}, {"version: 4", "version: 3", false}, {"geometry: tree", "geometry: shrub", true}, {"geometry: tree", "geometry: crossed_cards", true}, {"geometry: tree", "geometry: conifer", false},
	} {
		changed := base
		if test.from != "" {
			changed = strings.Replace(base, test.from, test.to, 1)
		}

		value, err := schema.Parse([]byte(changed))
		if err != nil {
			t.Fatal(err)
		}

		err = schema.Validate(value, schema.Level(value, []string{"surface", "normal"}))
		if (err == nil) != test.valid {
			t.Fatalf("%s: %v", test.to, err)
		}
	}
}

func TestPalmSchemaSourceAndGeometryGate(t *testing.T) {
	t.Parallel()

	base := strings.Replace(tinyWorld, "version: 6", "version: 20", 1)

	base += "vegetation:\n  version: 5\n  presets:\n    - {name: palm, geometry: palm, texture: surface, normal_texture: normal, width: {min: 1, max: 2}, height: {min: 1, max: 2}, wind_amplitude: 0.08, wind_frequency: 0.7, draw_distance: 32}\n"
	for _, test := range []struct {
		from, to string
		valid    bool
	}{
		{"", "", true}, {"version: 20", "version: 19", false}, {"version: 5", "version: 4", false},
		{"geometry: palm", "geometry: tree", true}, {"geometry: palm", "geometry: shrub", true},
		{"geometry: palm", "geometry: crossed_cards", true}, {"geometry: palm", "geometry: coconut", false},
		{"geometry: palm", "geometry: null", false}, {"normal_texture: normal", "normal_texture: absent", false},
	} {
		changed := base
		if test.from != "" {
			changed = strings.Replace(base, test.from, test.to, 1)
		}

		value, err := schema.Parse([]byte(changed))
		if err != nil {
			t.Fatal(err)
		}

		err = schema.Validate(value, schema.Level(value, []string{"surface", "normal"}))
		if (err == nil) != test.valid {
			t.Fatalf("%s: %v", test.to, err)
		}
	}
}
