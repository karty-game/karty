package schema_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/schema"
)

func TestEnvironmentSchemaControlsAndVersion(t *testing.T) {
	t.Parallel()

	base := strings.Replace(tinyWorld, "version: 6", "version: 9", 1)
	for _, test := range []struct {
		fields string
		valid  bool
	}{
		{"preset: daylight", true}, {"preset: space", true}, {"preset: cartoon, palette: twilight", true},
		{"preset: fantasy, palette: violet, sun: {azimuth: 0, elevation: 90}", true},
		{"preset: daylight, time_of_day: 0, cloud_amount: 0", true},
		{"preset: daylight, time_of_day: 23.99, cloud_amount: 1", true},
		{"preset: daylight, time_of_day: 24", false}, {"preset: daylight, time_of_day: -1", false},
		{"preset: daylight, cloud_amount: -0.1", false}, {"preset: daylight, cloud_amount: 1.01", false},
		{"preset: daylight, time_of_day: null", false}, {"preset: daylight, cloud_amount: null", false},
		{"preset: daylight, time_of_day: 15, sun: {azimuth: 0, elevation: 40}", false},
		{"preset: unknown", false}, {"preset: daylight, palette: sunset", false}, {"preset: daylight, sun: null", false},
		{"preset: daylight, sun: {azimuth: 360, elevation: 40}", false}, {"preset: daylight, sun: {azimuth: 0, elevation: 91}", false},
		{"preset: daylight, sun: {azimuth: 0}", false}, {"preset: daylight, sun: {azimuth: '1', elevation: 40}", false},
	} {
		input := base + "environment: {" + test.fields + "}\n"

		value, err := schema.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}

		err = schema.Validate(value, schema.Level(value, []string{"surface"}))
		if (err == nil) != test.valid {
			t.Fatalf("%s: valid=%v err=%v", test.fields, test.valid, err)
		}
	}

	for _, input := range []string{
		strings.Replace(base, "version: 9", "version: 8", 1) + "environment: null\n",
		strings.Replace(strings.Replace(base, "version: 9", "version: 8", 1), "- id: room", "- id: room\n    sky: false", 1),
	} {
		value, err := schema.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}

		if err := schema.Validate(value, schema.Level(value, []string{"surface"})); err == nil {
			t.Fatal("old source accepted environment field")
		}
	}

	for name, input := range map[string]string{
		"missing environment":      strings.Replace(base, "    ceiling:", "    sky: true\n    ceiling:", 1),
		"sky edge in covered room": strings.Replace(base, "material: surface}", "material: surface, sky: true}", 1) + "environment: {preset: daylight}\n",
	} {
		value, err := schema.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}

		if err := schema.Validate(value, schema.Level(value, []string{"surface"})); err == nil {
			t.Fatal("invalid sky declaration accepted", name)
		}
	}
}

func TestEnvironmentBackdropSchema(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		version uint16
		value   string
		valid   bool
	}{
		{worldsource.EnvironmentBackdropVersion, "none", true},
		{worldsource.EnvironmentBackdropVersion, "mountains", true},
		{worldsource.EnvironmentBackdropVersion, "city", true},
		{worldsource.EnvironmentBackdropVersion, "''", true},
		{worldsource.EnvironmentBackdropVersion, "null", false},
		{worldsource.EnvironmentBackdropVersion, "forest", false},
		{worldsource.EnvironmentBackdropVersion - 1, "mountains", false},
		{worldsource.EnvironmentBackdropVersion - 1, "''", false},
		{worldsource.EnvironmentBackdropVersion - 1, "null", false},
	} {
		input := strings.Replace(
			tinyWorld,
			"version: 6",
			"version: "+strconv.Itoa(int(test.version)),
			1,
		) + "environment: {preset: daylight, backdrop: " + test.value + "}\n"

		value, err := schema.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}

		err = schema.Validate(value, schema.Level(value, []string{"surface"}))
		if (err == nil) != test.valid {
			t.Fatalf("v%d backdrop %s: valid=%v err=%v", test.version, test.value, test.valid, err)
		}
	}
}
