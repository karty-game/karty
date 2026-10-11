package schema_test

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/schema"
)

func TestAnimationSchemaVersionAndTextureCatalog(t *testing.T) {
	t.Parallel()

	input := strings.Replace(
		tinyWorld,
		"version: 6",
		"version: 8",
		1,
	) + "animations:\n  version: 1\n  presets: [{name: glitch, kind: flipbook, frames: [surface, glitch], fps: 30, interval_seconds: 3}]\n  materials: [{material: surface, preset: glitch}]\n"

	value, err := schema.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	if err := schema.Validate(value, schema.Level(value, []string{"surface", "glitch"})); err != nil {
		t.Fatal(err)
	}

	if err := schema.Validate(value, schema.Level(value, []string{"surface"})); err == nil {
		t.Fatal("undeclared frame accepted by editor schema")
	}

	for version := 1; version <= 7; version++ {
		old := strings.Replace(input, "version: 8", "version: "+string(rune('0'+version)), 1)

		value, err := schema.Parse([]byte(old))
		if err != nil {
			t.Fatal(err)
		}

		if err := schema.Validate(value, schema.Level(value, []string{"surface", "glitch"})); err == nil {
			t.Fatalf("source v%d accepted animation", version)
		}
	}
}

func TestLiquidSchemaBoundsAndPresence(t *testing.T) {
	t.Parallel()

	for name, fields := range map[string]string{
		"valid":             "surface_amplitude: 0.045, opacity: 0.78, pixel_size: 4",
		"transparent":       "opacity: 0",
		"too high":          "surface_amplitude: 0.251",
		"null opacity":      "opacity: null",
		"null amplitude":    "surface_amplitude: null",
		"null pixels":       "pixel_size: null",
		"too opaque":        "opacity: 1.01",
		"too many pixels":   "pixel_size: 33",
		"fractional pixels": "pixel_size: 1.5",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			input := strings.Replace(
				tinyWorld,
				"version: 6",
				"version: 8",
				1,
			) + "animations: {version: 1, presets: [{name: ripple, kind: liquid, frequency: 1, " + fields + "}]}\n"

			value, err := schema.Parse([]byte(input))
			if err != nil {
				t.Fatal(err)
			}

			err = schema.Validate(value, schema.Level(value, []string{"surface"}))

			valid := name == "valid" || name == "transparent"
			if (err == nil) != valid {
				t.Fatalf("schema result valid=%v: %v", valid, err)
			}
		})
	}
}
