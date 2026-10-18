package schema_test

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/schema"
)

func TestFluidSchemaVersionAndBounds(t *testing.T) {
	t.Parallel()

	base := strings.Replace(
		tinyWorld,
		"version: 6",
		"version: 17",
		1,
	) + "fluids: {version: 1, items: [{id: water, solids: [pool], density: 1000, foam_width: 0.2}]}\n"

	for name, input := range map[string]string{"valid": base, "old": strings.Replace(base, "version: 17", "version: 16", 1), "density": strings.Replace(base, "density: 1000", "density: -1", 1), "null": strings.Replace(base, "density: 1000", "density: null", 1), "empty": strings.Replace(base, "solids: [pool]", "solids: []", 1), "unknown": strings.Replace(base, "foam_width: 0.2", "foam_width: 0.2, mystery: 1", 1)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			value, err := schema.Parse([]byte(input))
			if err == nil {
				err = schema.Validate(value, schema.Base())
			}

			if (err == nil) != (name == "valid") {
				t.Fatal("schema result", err)
			}
		})
	}
}
