package source_test

import (
	"errors"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestSkySourceVersionPresenceAndPartialDefaults(t *testing.T) {
	t.Parallel()

	for _, field := range []string{"sky: {}", "sky: null"} {
		if _, err := source.Decode(
			[]byte("version: 11\nenvironment: {preset: daylight, " + field + "}\n" + uvRoomYAML),
		); !errors.Is(
			err,
			worldsource.ErrVersion,
		) {
			t.Fatalf("old source sky presence bypass: %v", err)
		}
	}

	for _, field := range []string{"sky: null", "sky: {horizon: null}", "sky: {settings: null}", "sky: {settings: {horizon_haze: null}}"} {
		if _, err := source.Decode([]byte("version: 12\nenvironment: {preset: daylight, " + field + "}\n" + uvRoomYAML)); err == nil {
			t.Fatal("null sky input accepted", field)
		}
	}

	expanded, err := source.Decode(
		[]byte(
			"version: 12\nenvironment: {preset: daylight, sky: {horizon: sky/mountains.png, settings: {horizon_haze: 0.35}}}\n" + uvRoomYAML,
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	if expanded.Sky == nil || expanded.Sky.Horizon != "sky/mountains.png" ||
		expanded.Sky.ResolvedSettings().CloudUVScale != [2]float64{1, 1} ||
		expanded.Sky.ResolvedSettings().HorizonHaze != .35 || expanded.Sky.ResolvedSettings().HorizonUVScale != 1 {
		t.Fatalf("global sky or omitted setting default lost: %+v", expanded.Sky)
	}
}
