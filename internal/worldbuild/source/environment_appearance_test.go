package source_test

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestExplicitEnvironmentSourcePresenceAndOwnership(t *testing.T) {
	t.Parallel()

	lighting := "lighting: {sun_color: {x: 0.2, y: 0.5, z: 1}, sun_intensity: 2, ambient: {x: 0.4, y: 0.5, z: 0.6}}"
	input := "version: 18\nenvironment: {" + lighting + "}\n" + uvRoomYAML

	expanded, err := source.Decode([]byte(input))
	if err != nil || expanded.Environment.Appearance == nil || expanded.Environment.Preset != "" ||
		expanded.Environment.Appearance.Lighting.SunIntensity != 2 {
		t.Fatal("explicit environment", err)
	}

	for _, invalid := range []string{strings.Replace(input, "version: 18", "version: 17", 1), strings.Replace(input, "lighting:", "preset: cartoon, lighting:", 1), strings.Replace(input, "sun_intensity: 2, ", "", 1), strings.Replace(input, "sun_intensity: 2", "sun_intensity: null", 1), strings.Replace(input, "sun_intensity: 2", "sun_intensity: 17", 1), strings.Replace(input, "environment: {", "environment: {palette: '', ", 1)} {
		if _, err := source.Decode([]byte(invalid)); err == nil {
			t.Fatal("invalid explicit environment accepted", invalid)
		}
	}
}
