package source_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

const uvRoomYAML = `rooms:
  - id: room
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 4, y: 0}, material: wall}
      - {id: east, start: {x: 4, y: 0}, end: {x: 4, y: 4}, material: wall}
      - {id: north, start: {x: 4, y: 4}, end: {x: 0, y: 4}, material: wall}
      - {id: west, start: {x: 0, y: 4}, end: {x: 0, y: 0}, material: wall}
    floor: {c: 0}
    ceiling: {c: 6}
    floor_material: floor
    ceiling_material: ceiling
`

func TestSourceMappingVersionDefaultsAndLegacyAbsence(t *testing.T) {
	t.Parallel()

	for version := 1; version <= 5; version++ {
		expanded, err := source.Decode([]byte(fmt.Sprintf("version: %d\n", version) + uvRoomYAML))
		if err != nil {
			t.Fatal(err)
		}

		room := expanded.Rooms[0]
		if version < 5 {
			if expanded.UV != nil || room.FloorUV != nil || room.CeilingUV != nil || room.WallUV != nil || room.Boundary[0].UV != nil {
				t.Fatalf("legacy source v%d gained UV mapping", version)
			}

			continue
		}

		for _, settings := range []*worldsource.UVSettings{expanded.UV, room.FloorUV, room.CeilingUV, room.WallUV, room.Boundary[0].UV} {
			if settings == nil || settings.Mode != worldsource.UVTriplanar || settings.Anchor != worldsource.UVWorld ||
				*settings.Scale != (worldsource.Vec2{X: 1, Y: 1}) || *settings.Offset != (worldsource.Vec2{}) || *settings.RotationDegrees != 0 {
				t.Fatalf("v5 omitted/default settings: %+v", settings)
			}
		}
	}
}

func TestSourceMappingFieldInheritanceAndOwnedValues(t *testing.T) {
	t.Parallel()

	roomYAML := strings.Replace(uvRoomYAML, "    floor: {c: 0}", `    floor_uv: {scale: {x: 4, y: 5}, rotation_degrees: 0}
    wall_uv: {mode: planar, offset: {x: 0.75, y: 1}}
    floor: {c: 0}`, 1)
	roomYAML = strings.Replace(roomYAML, "material: wall}", "material: wall, uv: {mode: triplanar, anchor: top, rotation_degrees: 0}}", 1)

	expanded, err := source.Decode(
		[]byte(
			"version: 5\nuv: {mode: wrap, anchor: bottom, scale: {x: 2, y: 3}, offset: {x: 0.25, y: 0.5}, rotation_degrees: 90}\n" + roomYAML,
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	room := expanded.Rooms[0]
	if room.FloorUV.Mode != worldsource.UVPlanar || room.FloorUV.Anchor != worldsource.UVWorld ||
		*room.FloorUV.Scale != (worldsource.Vec2{X: 4, Y: 5}) || *room.FloorUV.Offset != (worldsource.Vec2{X: .25, Y: .5}) || *room.FloorUV.RotationDegrees != 0 {
		t.Fatalf("floor world fallback/override: %+v", room.FloorUV)
	}

	if room.CeilingUV.Mode != worldsource.UVPlanar || room.CeilingUV.Anchor != worldsource.UVWorld ||
		*room.CeilingUV.RotationDegrees != 90 {
		t.Fatalf("ceiling root wrap/anchor fallback: %+v", room.CeilingUV)
	}

	if room.WallUV.Mode != worldsource.UVPlanar || room.WallUV.Anchor != worldsource.UVBottom ||
		*room.WallUV.Scale != (worldsource.Vec2{X: 2, Y: 3}) || *room.WallUV.Offset != (worldsource.Vec2{X: .75, Y: 1}) || *room.WallUV.RotationDegrees != 90 {
		t.Fatalf("room wall field inheritance: %+v", room.WallUV)
	}

	first, second := room.Boundary[0].UV, room.Boundary[1].UV
	if first.Mode != worldsource.UVTriplanar || first.Anchor != worldsource.UVTop || *first.RotationDegrees != 0 ||
		!reflect.DeepEqual(second, room.WallUV) {
		t.Fatal("edge override or inheritance lost")
	}

	first.Scale.X, first.Offset.Y, *first.RotationDegrees = 77, 88, 99
	if second.Scale.X != 2 || second.Offset.Y != 1 || *second.RotationDegrees != 90 || room.WallUV.Scale.X != 2 ||
		expanded.UV.Scale.X != 2 {
		t.Fatal("effective UV settings alias another surface/root")
	}
}

func TestSourceMappingPrefabTransformsDoNotScaleControls(t *testing.T) {
	t.Parallel()

	input := "version: 5\nuv: {scale: {x: 2, y: 3}}\nprefabs:\n  - id: cell\n" +
		strings.ReplaceAll(uvRoomYAML, "\n", "\n    ") + `
instances:
  - {id: large, prefab: cell, transform: {translation: {x: 10, y: 7, z: 2}, yaw_degrees: 90, scale: 3}}
  - {id: small, prefab: cell, transform: {translation: {x: -5, y: -6, z: 0}, scale: 0.5}}
`
	// The prefab's rooms key needs the same four-space nesting as its body.
	input = strings.Replace(input, "\nrooms:", "\n    rooms:", 1)

	expanded, err := source.Decode([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	if len(expanded.Rooms) != 2 || expanded.Rooms[0].Boundary[0].Start != (worldsource.Vec2{X: 10, Y: 7}) ||
		expanded.Rooms[0].Boundary[0].End != (worldsource.Vec2{X: 10, Y: 19}) {
		t.Fatalf("geometry expansion changed: %+v", expanded.Rooms)
	}

	for _, room := range expanded.Rooms {
		if *room.FloorUV.Scale != (worldsource.Vec2{X: 2, Y: 3}) || *room.FloorUV.RotationDegrees != 0 ||
			*room.WallUV.Scale != (worldsource.Vec2{X: 2, Y: 3}) {
			t.Fatal("prefab scale/yaw changed world-space texture controls")
		}
	}
}

func TestSourceMappingRejectsInvalidFinalControlsAndOldVersions(t *testing.T) {
	t.Parallel()

	for _, settings := range []string{
		"{mode: future}", "{anchor: sector}", "{scale: {x: 0, y: 1}}", "{scale: {x: 1, y: 0.0009}}",
		"{scale: {x: 1, y: .nan}}", "{offset: {x: 1000001, y: 0}}", "{rotation_degrees: .inf}",
	} {
		input := strings.Replace(uvRoomYAML, "material: wall}", "material: wall, uv: "+settings+"}", 1)

		expanded, err := source.Decode([]byte("version: 5\n" + input))
		if err == nil || len(expanded.Rooms) != 0 || expanded.UV != nil {
			t.Fatalf("invalid complete UV payload published: %s", settings)
		}
	}

	for version := 1; version <= 4; version++ {
		if _, err := source.Decode(
			[]byte(fmt.Sprintf("version: %d\nuv: {mode: planar}\n", version) + uvRoomYAML),
		); !errors.Is(
			err,
			worldsource.ErrVersion,
		) {
			t.Fatalf("source v%d accepted projection controls: %v", version, err)
		}
	}

	for _, settings := range []string{"{mode: wrap}", "{anchor: top}", "{anchor: bottom}"} {
		input := strings.Replace(uvRoomYAML, "    floor: {c: 0}", "    floor_uv: "+settings+"\n    floor: {c: 0}", 1)
		if _, err := source.Decode([]byte("version: 5\n" + input)); !errors.Is(err, worldsource.ErrUVMapping) {
			t.Fatalf("horizontal surface accepted wall-only mapping: %s %v", settings, err)
		}
	}
}
