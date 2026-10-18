package schema_test

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/schema"
)

func TestExplicitEnvironmentEditorSchema(t *testing.T) {
	t.Parallel()

	base := strings.Replace(tinyWorld, "version: 6", "version: 18", 1)

	valid := base + "environment: {lighting: {sun_color: {x: 1, y: 1, z: 1}, sun_intensity: 2, ambient: {x: 0.2, y: 0.3, z: 0.4}}}\n"
	for _, test := range []struct {
		input string
		valid bool
	}{{valid, true}, {strings.Replace(valid, "lighting:", "preset: cartoon, lighting:", 1), false}, {strings.Replace(valid, "version: 18", "version: 17", 1), false}, {strings.Replace(valid, "sun_intensity: 2, ", "", 1), false}, {strings.Replace(valid, "sun_intensity: 2", "sun_intensity: 17", 1), false}} {
		value, err := schema.Parse([]byte(test.input))
		if err != nil {
			t.Fatal(err)
		}

		err = schema.Validate(value, schema.Base())
		if (err == nil) != test.valid {
			t.Fatal("editor explicit environment", err, test.input)
		}
	}
}
