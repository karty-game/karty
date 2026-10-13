package schema_test

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/schema"
)

func TestSkySchemaVersionSettingsAndNullPresence(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		version, sky string
		valid        bool
	}{
		{"12", "{}", true},
		{"12", "{horizon: sky/mountains.png, settings: {horizon_haze: 0.35}}", true},
		{"12", "{horizon: sky/mountains.png, settings: {horizon_uv_scale: 1.5}}", true},
		{"12", "{settings: {horizon_uv_scale: 0}}", false},
		{"12", "{settings: {horizon_uv_scale: 4.01}}", false},
		{"12", "{settings: {horizon_uv_scale: null}}", false},
		{"12", "{settings: {cloud_uv_scale: [1, 0.95], cloud_parallax: [0, 0]}}", true},
		{"11", "{}", false}, {"11", "null", false}, {"12", "null", false},
		{"12", "{horizon: null}", false}, {"12", "{settings: null}", false},
		{"12", "{settings: {horizon_haze: null}}", false},
		{"12", "{settings: {horizon_haze: 1.01}}", false},
		{"12", "{settings: {cloud_uv_scale: [0, 1]}}", false},
		{"12", "{settings: {cloud_uv_scale: [1]}}", false},
		{"12", "{settings: {cloud_parallax: [0, 0.02]}}", false},
	} {
		input := strings.Replace(
			tinyWorld,
			"version: 6",
			"version: "+test.version,
			1,
		) + "environment: {preset: daylight, sky: " + test.sky + "}\n"

		value, err := schema.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}

		err = schema.Validate(value, schema.Level(value, []string{"surface"}))
		if (err == nil) != test.valid {
			t.Fatalf("v%s sky %s: valid=%v err=%v", test.version, test.sky, test.valid, err)
		}
	}
}
