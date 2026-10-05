package source_test

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestLightMotionYAMLRemainsGlobalAndRejectsInvalidTail(t *testing.T) {
	t.Parallel()

	input := "version: 4\n" + lightingPrefabYAML + strings.Replace(
		lightingYAML, "radius: 2}",
		"radius: 2, motion: {version: 1, offset: {x: 12, y: 0, z: 0.8}, period_seconds: 12}}", 1,
	)

	expanded, err := source.Decode([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	light := expanded.Lighting.Lights[1]
	if light.Position.X != 99 || light.Motion == nil || light.Motion.Offset.X != 12 || light.Motion.Offset.Z != .8 ||
		light.Motion.PeriodSeconds != 12 {
		t.Fatalf("motion transformed or dropped: %+v", light)
	}

	for _, malformed := range []string{
		strings.Replace(input, "period_seconds: 12", "period_seconds: 0", 1),
		strings.Replace(input, "period_seconds: 12", "period_seconds: 12, typo: true", 1),
		strings.Replace(input, "offset: {x: 12", "offset: {x: 1000000", 1),
	} {
		if _, err := source.Decode([]byte(malformed)); err == nil {
			t.Fatal("malformed last light motion accepted")
		}
	}
}
