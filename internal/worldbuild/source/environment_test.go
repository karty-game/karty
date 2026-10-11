package source_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestEnvironmentSourcePresenceGates(t *testing.T) {
	t.Parallel()

	for version := 1; version < int(worldsource.EnvironmentVersion); version++ {
		for _, field := range []string{"environment: null", "rooms: [{sky: false}]", "prefabs: [{rooms: [{boundary: [{sky: false}]}]}]"} {
			input := fmt.Sprintf("version: %d\n%s\n", version, field)
			if _, err := source.Decode([]byte(input)); !errors.Is(err, worldsource.ErrVersion) {
				t.Fatalf("v%d presence bypass: %v", version, err)
			}
		}
	}

	for _, field := range []string{
		"environment: null", "environment: {preset: daylight, sun: null}",
		"environment: {preset: daylight, sun: {azimuth: 10}}", "environment: {preset: daylight, sun: {elevation: 40}}",
		"environment: {preset: daylight, sun: {azimuth: null, elevation: 40}}", "rooms: [{sky: null}]",
		"environment: {preset: daylight, time_of_day: null}", "environment: {preset: daylight, cloud_amount: null}",
	} {
		if _, err := source.Decode([]byte("version: 9\n" + field + "\n")); !errors.Is(err, source.ErrYAML) {
			t.Fatalf("invalid environment presence bypass: %s: %v", field, err)
		}
	}
}

func TestEnvironmentPrefabExpansionKeepsGlobalSunAndSky(t *testing.T) {
	t.Parallel()

	const input = `version: 9
environment: {preset: fantasy, palette: violet, sun: {azimuth: 135, elevation: 40}, cloud_amount: 0}
prefabs:
  - id: outdoors
    rooms:
      - id: ground
        sky: true
        floor: {c: 0}
        ceiling: {c: 8}
        floor_material: wall
        ceiling_material: wall
        boundary:
          - {id: south, start: {x: 0, y: 0}, end: {x: 2, y: 0}, material: wall}
          - {id: east, start: {x: 2, y: 0}, end: {x: 2, y: 2}, material: wall, sky: true}
          - {id: north, start: {x: 2, y: 2}, end: {x: 0, y: 2}, material: wall}
          - {id: west, start: {x: 0, y: 2}, end: {x: 0, y: 0}, material: wall}
instances:
  - {id: rotated, prefab: outdoors, transform: {translation: {x: 10}, yaw_degrees: 90}}
`

	expanded, err := source.Decode([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	if len(expanded.Rooms) != 1 || !expanded.Rooms[0].Sky || !expanded.Rooms[0].Boundary[1].Sky ||
		expanded.Environment.Sun.Azimuth != 135 || expanded.Environment.Sun.Elevation != 40 ||
		expanded.Environment.CloudAmount == nil || *expanded.Environment.CloudAmount != 0 {
		t.Fatal("prefab transformed or lost the global environment", expanded)
	}
}

func TestEnvironmentSourceTimeAndCloudControls(t *testing.T) {
	t.Parallel()

	for _, fields := range []string{"time_of_day: 0, cloud_amount: 0", "time_of_day: 23.9, cloud_amount: 1"} {
		expanded, err := source.Decode([]byte("version: 9\nenvironment: {preset: daylight, " + fields + "}\n" + uvRoomYAML))
		if err != nil || expanded.Environment.TimeOfDay == nil || expanded.Environment.CloudAmount == nil {
			t.Fatalf("controls lost: %s: %v", fields, err)
		}
	}
}
