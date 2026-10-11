package source_test

import (
	"errors"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestOldSourceRejectsExplicitNullAnimationFields(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"version: 7\nanimations: null\n",
		"version: 7\ncontents: [{id: marker, kind: actor, actor: {animation: null}}]\n",
		"version: 7\nprefabs: [{id: marker, contents: [{id: part, kind: actor, actor: {animation: null}}]}]\n",
	} {
		if _, err := source.Decode([]byte(input)); !errors.Is(err, worldsource.ErrVersion) {
			t.Fatalf("explicit null bypassed animation version gate: %v", err)
		}
	}
}

func TestLiquidSourceRejectsNullAndOtherKindFields(t *testing.T) {
	t.Parallel()

	for _, field := range []string{"surface_amplitude", "opacity", "pixel_size"} {
		for _, value := range []string{"null", "0"} {
			kind := "spin"
			if value == "null" {
				kind = "liquid"
			}

			input := "version: 8\nanimations: {version: 1, presets: [{name: effect, kind: " + kind + ", " + field + ": " + value + "}]}\n"
			if _, err := source.Decode([]byte(input)); !errors.Is(err, worldsource.ErrContent) {
				t.Fatalf("%s/%s bypassed presence gate: %v", field, value, err)
			}
		}
	}
}
