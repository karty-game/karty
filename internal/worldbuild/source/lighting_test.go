package source_test

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	sdkworld "github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

const lightingPrefabYAML = `prefabs:
  - id: cell
    rooms:
      - id: room
        boundary:
          - {id: south, start: {x: 0, y: 0}, end: {x: 2, y: 0}, material: wall}
          - {id: east, start: {x: 2, y: 0}, end: {x: 2, y: 2}, material: wall}
          - {id: north, start: {x: 2, y: 2}, end: {x: 0, y: 2}, material: wall}
          - {id: west, start: {x: 0, y: 2}, end: {x: 0, y: 0}, material: wall}
        floor: {c: 0}
        ceiling: {c: 4}
        floor_material: wall
        ceiling_material: wall
instances:
  - id: transformed
    prefab: cell
    transform: {translation: {x: 10, y: 20, z: 3}, yaw_degrees: 90, scale: 2}
`

const lightingYAML = `lighting:
  version: 1
  ambient: {x: 0.1, y: 0.2, z: 0.3}
  lights:
    - {id: z-first, position: {x: -5, y: 3, z: 7}, color: {x: 1, y: 0.4, z: 0.2}, radius: 6}
    - {id: a-second, position: {x: 99, y: -4, z: 2}, color: {x: 0, y: 0, z: 1}, radius: 2}
`

func TestDecodePreservesGlobalLightingAcrossPrefabTransforms(t *testing.T) {
	t.Parallel()

	input := []byte("version: 4\n" + lightingPrefabYAML + lightingYAML)

	expanded, err := source.Decode(input)
	if err != nil {
		t.Fatal(err)
	}

	want := &sdkworld.Lighting{
		Version: 1, Ambient: sdkworld.Vec3{X: .1, Y: .2, Z: .3},
		Lights: []sdkworld.PointLight{
			{ID: "z-first", Position: sdkworld.Vec3{X: -5, Y: 3, Z: 7}, Color: sdkworld.Vec3{X: 1, Y: .4, Z: .2}, Radius: 6},
			{ID: "a-second", Position: sdkworld.Vec3{X: 99, Y: -4, Z: 2}, Color: sdkworld.Vec3{Z: 1}, Radius: 2},
		},
	}
	if !reflect.DeepEqual(expanded.Lighting, want) || expanded.Rooms[0].Boundary[0].Start.X != 10 {
		t.Fatalf("global lighting was transformed, reordered or dropped: %+v", expanded)
	}

	expanded.Lighting.Lights[0].Position.X = 42

	again, err := source.Decode(input)
	if err != nil || !reflect.DeepEqual(again.Lighting, want) {
		t.Fatalf("lighting ownership changed later decoding: %+v, %v", again.Lighting, err)
	}
}

func TestDecodeLightingBoundsAndCompleteValidation(t *testing.T) {
	t.Parallel()

	for _, count := range []int{0, 1, 50, 51} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			t.Parallel()

			var lighting strings.Builder
			lighting.WriteString("lighting:\n  version: 1\n  lights:\n")

			for index := range count {
				fmt.Fprintf(&lighting, "    - {id: light-%d, radius: 1}\n", index)
			}

			expanded, err := source.Decode([]byte("version: 4\n" + lightingPrefabYAML + lighting.String()))
			if count > 50 {
				if !errors.Is(err, worldsource.ErrBounds) {
					t.Fatalf("51 lights accepted: %v", err)
				}

				return
			}

			if err != nil || expanded.Lighting == nil || len(expanded.Lighting.Lights) != count {
				t.Fatalf("bounded lighting lost: %+v, %v", expanded.Lighting, err)
			}
		})
	}

	for name, mutation := range map[string][2]string{
		"version":           {"version: 1\n  ambient", "version: 2\n  ambient"},
		"ambient negative":  {"x: 0.1", "x: -0.1"},
		"ambient nonfinite": {"y: 0.2", "y: .nan"},
		"last duplicate":    {"id: a-second", "id: z-first"},
		"last empty ID":     {"id: a-second", `id: ""`},
		"last radius":       {"radius: 2}", "radius: 0}"},
		"last position":     {"x: 99", "x: 1000001"},
		"last nonfinite":    {"y: -4", "y: .inf"},
		"last color":        {"z: 1}", "z: 1.1}"},
		"unknown lighting":  {"  ambient:", "  brightness: 1\n  ambient:"},
		"unknown light":     {"id: a-second", "id: a-second, brightness: 1"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			input := "version: 4\n" + lightingPrefabYAML + strings.Replace(lightingYAML, mutation[0], mutation[1], 1)
			if _, err := source.Decode([]byte(input)); err == nil {
				t.Fatal("invalid lighting accepted")
			}
		})
	}
}

func TestDecodeLightingVersionOptInAndAbsence(t *testing.T) {
	t.Parallel()

	for version := 1; version <= 4; version++ {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			t.Parallel()

			input := fmt.Sprintf("version: %d\n%s", version, lightingPrefabYAML)

			expanded, err := source.Decode([]byte(input))
			if err != nil || expanded.Lighting != nil {
				t.Fatalf("absent lighting changed v%d: %+v, %v", version, expanded.Lighting, err)
			}

			if version < 4 {
				if _, err := source.Decode([]byte(input + lightingYAML)); !errors.Is(err, worldsource.ErrVersion) {
					t.Fatalf("old source version accepted lighting: %v", err)
				}
			}
		})
	}
}

func TestDecodeDirectionalAmbientAndActorOptIn(t *testing.T) {
	t.Parallel()

	extension := "  actors: true\n  ambient_cube:\n    positive_x: {x: 1}\n    negative_x: {y: 1}\n    positive_y: {z: 1}\n    negative_y: {x: 1, y: 1}\n    positive_z: {y: 1, z: 1}\n    negative_z: {x: 1, z: 1}\n"
	input := "version: 4\n" + lightingPrefabYAML + strings.Replace(lightingYAML, "  lights:", extension+"  lights:", 1)

	expanded, err := source.Decode([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	if !expanded.Lighting.Actors || expanded.Lighting.AmbientCube == nil ||
		expanded.Lighting.AmbientCube.NegativeZ != (sdkworld.Vec3{X: 1, Z: 1}) {
		t.Fatalf("cube/actors lost: %+v", expanded.Lighting)
	}

	for _, bad := range []string{"{x: -0.001, z: 1}", "{x: 1, z: 1.001}", "{x: 1, z: .nan}", "{x: 1, z: .inf}"} {
		if _, err := source.Decode([]byte(strings.Replace(input, "{x: 1, z: 1}", bad, 1))); err == nil {
			t.Fatalf("invalid last face %s accepted", bad)
		}
	}

	expanded.Lighting.AmbientCube.NegativeZ.X = .3

	again, err := source.Decode([]byte(input))
	if err != nil || again.Lighting.AmbientCube.NegativeZ.X != 1 {
		t.Fatal("decoded cube ownership lost")
	}
}
