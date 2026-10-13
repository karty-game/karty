package schema_test

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/schema"
)

func TestAlphaParticleSchemaVersionGates(t *testing.T) {
	t.Parallel()

	base := strings.Replace(tinyWorld, "version: 6", "version: 11", 1) + strings.Replace(particleYAML, "version: 1", "version: 2", 1)
	base = strings.Replace(base, "blend: additive", "blend: alpha", 1)

	for _, test := range []struct {
		name, input string
		ok          bool
	}{
		{"source11 alpha2", base, true},
		{"source11 premultiplied2", strings.Replace(base, "blend: alpha", "blend: premultiplied", 1), true},
		{"source10 premultiplied2", strings.Replace(strings.Replace(base, "version: 11", "version: 10", 1), "blend: alpha", "blend: premultiplied", 1), false},
		{"source11 premultiplied1", strings.Replace(strings.Replace(base, "version: 2", "version: 1", 1), "blend: alpha", "blend: premultiplied", 1), false},
		{"source10 alpha2", strings.Replace(base, "version: 11", "version: 10", 1), false},
		{"source11 alpha1", strings.Replace(base, "version: 2", "version: 1", 1), false},
		{"source11 additive1", strings.Replace(strings.Replace(base, "version: 2", "version: 1", 1), "blend: alpha", "blend: additive", 1), true},
		{"source11 additive2", strings.Replace(base, "blend: alpha", "blend: additive", 1), true},
		{"unknown particle version", strings.Replace(base, "version: 2", "version: 3", 1), false},
		{"unknown blend", strings.Replace(base, "blend: alpha", "blend: multiply", 1), false},
		{"null frames", strings.Replace(base, "frames: [spark]", "frames: null", 1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			value, err := schema.Parse([]byte(test.input))
			if err != nil {
				t.Fatal(err)
			}

			err = schema.Validate(value, schema.Level(value, []string{"surface", "spark"}))
			if (err == nil) != test.ok {
				t.Fatalf("schema gate: %v", err)
			}
		})
	}
}
