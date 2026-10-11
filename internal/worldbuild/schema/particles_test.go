package schema_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/schema"
)

const particleYAML = `particles:
  version: 1
  presets:
    - name: sparkle
      capacity: 8
      emission: {rate: 4, duration_seconds: 1}
      lifetime_seconds: {min: 0.3, max: 0.5}
      spawn: {shape: point}
      sprite: {frames: [spark], playback: cycle, facing: camera-facing, blend: additive}
      size_over_life: [{age: 0, value: 0.05}, {age: 1, value: 0}]
      color_over_life: [{age: 0, rgba: [0, 1, 0, 1]}, {age: 1, rgba: [0, 1, 0, 0]}]
  emitters:
    - {id: pool, preset: sparkle, position: {x: 1, y: 1, z: 1}, seed: 42, playback: {mode: cycle, period_seconds: 1}}
`

func TestParticleSchemaVersionPresenceAndCatalog(t *testing.T) {
	t.Parallel()

	input := strings.Replace(tinyWorld, "version: 6", "version: 10", 1) + particleYAML

	value, err := schema.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	if err := schema.Validate(value, schema.Level(value, []string{"surface", "spark"})); err != nil {
		t.Fatal(err)
	}

	if err := schema.Validate(value, schema.Level(value, []string{"surface"})); err == nil {
		t.Fatal("undeclared particle frame accepted")
	}

	for _, test := range []struct{ from, to string }{
		{"version: 10", "version: 9"}, {"frames: [spark]", "frames: null"}, {"capacity: 8", "capacity: 4097"},
		{"rgba: [0, 1, 0, 1]", "rgba: [0, 1, 0]"}, {"preset: sparkle", "preset: missing"}, {"shape: point", "shape: sphere"},
		{"mode: cycle", "mode: unknown"}, {"blend: additive", "blend: alpha"}, {"min: 0.3", "min: 0"}, {"max: 0.5", "max: 3601"},
		{"seed: 42", "seed: 42, scale: 0.0000001"}, {"period_seconds: 1", "period_seconds: 0"},
		{"mode: cycle", "mode: once"}, {"frames: [spark]", "frames: [spark, spark]"},
	} {
		changed := strings.Replace(input, test.from, test.to, 1)

		value, err := schema.Parse([]byte(changed))
		if err != nil {
			t.Fatal(err)
		}

		if err := schema.Validate(value, schema.Level(value, []string{"surface", "spark"})); err == nil {
			t.Fatalf("schema accepted %s", test.to)
		}
	}

	for version := 1; version < 10; version++ {
		value, err := schema.Parse([]byte(fmt.Sprintf("version: %d\nparticles: null\n", version)))
		if err != nil {
			t.Fatal(err)
		}

		if err := schema.Validate(value, schema.Level(value, nil)); err == nil {
			t.Fatalf("oldversion%d accepted particles:null", version)
		}
	}
}
