package geometry_test

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

const uvConcaveRoom = `rooms:
  - id: room
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 4, y: 0}, material: wall}
      - {id: east, start: {x: 4, y: 0}, end: {x: 4, y: 2}, material: wall}
      - {id: inset-south, start: {x: 4, y: 2}, end: {x: 2, y: 2}, material: wall}
      - {id: inset-east, start: {x: 2, y: 2}, end: {x: 2, y: 4}, material: wall}
      - {id: north, start: {x: 2, y: 4}, end: {x: 0, y: 4}, material: wall}
      - {id: west, start: {x: 0, y: 4}, end: {x: 0, y: 0}, material: wall}
    floor: {a: 0.25, c: 0}
    ceiling: {b: 0.5, c: 8}
    floor_material: floor
    ceiling_material: ceiling
`

func compileUVFixture(t *testing.T, input string) (source.Expanded, world.Document) {
	t.Helper()

	expanded, err := source.Decode([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := geometry.Compile(expanded, map[string]uint32{"floor": 1, "ceiling": 2, "wall": 3})
	if err != nil {
		t.Fatal(err)
	}

	return expanded, compiled
}

func TestCompileMappingDefaultAxisPlanesAndFourthPowerWeights(t *testing.T) {
	t.Parallel()

	_, compiled := compileUVFixture(t, "version: 5\n"+uvConcaveRoom)
	if compiled.MaterialMapping == nil || compiled.MaterialMapping.Version != 1 || len(compiled.Sectors) < 2 {
		t.Fatal("default mapping marker or concave decomposition lost")
	}

	for _, sector := range compiled.Sectors {
		for _, settings := range []*world.SurfaceUV{sector.FloorUV, sector.CeilingUV} {
			if len(settings.Projections) != 3 || world.ValidateSurfaceUV(settings) != nil {
				t.Fatal("horizontal mapping is incomplete")
			}

			want := []world.UVProjection{
				{U: world.UVPlane{X: 1}, V: world.UVPlane{Y: 1}},
				{U: world.UVPlane{Y: 1}, V: world.UVPlane{Z: 1}},
				{U: world.UVPlane{X: 1}, V: world.UVPlane{Z: 1}},
			}
			for index, projection := range want {
				if settings.Projections[index] != projection {
					t.Fatalf("compiler inferred non-global/signed axis planes: %+v", settings)
				}
			}
		}
		// Floor normal is proportional to (-.25,0,1); ceiling to (0,.5,-1).
		assertUVNear(t, sector.FloorUV.Weights[0], 256.0/257)
		assertUVNear(t, sector.FloorUV.Weights[1], 1.0/257)
		assertUVNear(t, sector.FloorUV.Weights[2], 0)
		assertUVNear(t, sector.CeilingUV.Weights[0], 16.0/17)
		assertUVNear(t, sector.CeilingUV.Weights[1], 0)
		assertUVNear(t, sector.CeilingUV.Weights[2], 1.0/17)

		for _, wall := range sector.Walls {
			if wall.UV == nil || len(wall.UV.Projections) != 3 || world.ValidateSurfaceUV(wall.UV) != nil {
				t.Fatal("generated/internal wall lacks complete mapping")
			}

			deltaX, deltaY := wall.End.X-wall.Start.X, wall.End.Y-wall.Start.Y
			weightX, weightY := math.Pow(deltaY, 4), math.Pow(deltaX, 4)

			assertUVNear(t, wall.UV.Weights[0], 0)
			assertUVNear(t, wall.UV.Weights[1], weightX/(weightX+weightY))
			assertUVNear(t, wall.UV.Weights[2], weightY/(weightX+weightY))
		}
	}
}

func TestCompileWrapAuthoredPerimeterAndFloorAnchorSurviveDecomposition(t *testing.T) {
	t.Parallel()
	expanded, compiled := compileUVFixture(
		t,
		"version: 5\nuv: {mode: wrap, anchor: bottom, scale: {x: 2, y: 3}, offset: {x: 0.25, y: 0.5}}\n"+uvConcaveRoom,
	)
	room := expanded.Rooms[0]
	byName := make(map[string]world.Wall)
	internal := 0

	for _, sector := range compiled.Sectors {
		if len(sector.FloorUV.Projections) != 1 || len(sector.CeilingUV.Projections) != 1 {
			t.Fatal("root wrap did not fall back to horizontal planar mapping")
		}

		for _, wall := range sector.Walls {
			if wall.SourceEdge == "" {
				internal++

				if wall.UV == nil || world.ValidateSurfaceUV(wall.UV) != nil {
					t.Fatal("internal placeholder mapping is incomplete")
				}

				continue
			}

			byName[wall.SourceEdge] = wall
		}
	}

	if internal == 0 || len(byName) != len(room.Boundary) {
		t.Fatal("decomposition changed authored boundary identities")
	}

	perimeter := 0.0

	for _, edge := range room.Boundary {
		mapping := byName[edge.ID].UV
		if len(mapping.Projections) != 1 {
			t.Fatal("wrap produced multiple projections")
		}

		projection := mapping.Projections[0]
		start := world.Vec3{X: edge.Start.X, Y: edge.Start.Y}
		end := world.Vec3{X: edge.End.X, Y: edge.End.Y}
		length := math.Hypot(end.X-start.X, end.Y-start.Y)
		assertUVNear(t, evaluateUV(projection.U, start), perimeter/2+.25)
		assertUVNear(t, evaluateUV(projection.U, end), (perimeter+length)/2+.25)

		floorHeight := room.Floor.A*start.X + room.Floor.B*start.Y + room.Floor.C
		start.Z = floorHeight
		assertUVNear(t, evaluateUV(projection.V, start), .5)
		start.Z += 3
		assertUVNear(t, evaluateUV(projection.V, start), 1.5)

		perimeter += length
	}

	assertUVNear(t, perimeter, 16)
}

func TestCompileMappingDensityIndependentOfPrefabScaleAndYaw(t *testing.T) {
	t.Parallel()

	input := "version: 5\nuv: {mode: planar, scale: {x: 2, y: 4}}\nprefabs:\n  - id: cell\n    " + strings.ReplaceAll(
		uvConcaveRoom,
		"\n",
		"\n    ",
	) + `
instances:
  - {id: large, prefab: cell, transform: {translation: {x: 10, y: 7, z: 2}, yaw_degrees: 90, scale: 3}}
  - {id: small, prefab: cell, transform: {translation: {x: -5, y: -6, z: 0}, scale: 0.5}}
`

	_, compiled := compileUVFixture(t, input)
	for _, sector := range compiled.Sectors {
		projection := sector.FloorUV.Projections[0]
		if projection.U != (world.UVPlane{X: .5}) || projection.V != (world.UVPlane{Y: .25}) {
			t.Fatalf("geometry scaling/rotation stretched world-space floor texture: %+v", projection)
		}

		for _, wall := range sector.Walls {
			if wall.SourceEdge != "south" {
				continue
			}

			projection := wall.UV.Projections[0]
			start, end := world.Vec3{X: wall.Start.X, Y: wall.Start.Y}, world.Vec3{X: wall.End.X, Y: wall.End.Y}
			length := math.Hypot(end.X-start.X, end.Y-start.Y)
			assertUVNear(t, evaluateUV(projection.U, end)-evaluateUV(projection.U, start), length/2)

			if sector.Instance == "large" {
				assertUVNear(t, projection.U.X, 0)
				assertUVNear(t, projection.U.Y, .5)
				assertUVNear(t, evaluateUV(projection.U, start), start.Y/2)
			}
		}
	}
}

func TestCompileMappingRotationAndTopAnchorUseOriginalRoomPlane(t *testing.T) {
	t.Parallel()

	_, compiled := compileUVFixture(
		t,
		"version: 5\nuv: {mode: planar, anchor: top, scale: {x: 2, y: 4}, offset: {x: 0.25, y: -0.5}, rotation_degrees: 90}\n"+uvConcaveRoom,
	)
	for _, sector := range compiled.Sectors {
		projection := sector.FloorUV.Projections[0]
		point := world.Vec3{X: 2, Y: 4, Z: 2}
		assertUVNear(t, evaluateUV(projection.U, point), -.75)
		assertUVNear(t, evaluateUV(projection.V, point), .5)

		for _, wall := range sector.Walls {
			if wall.SourceEdge != "south" {
				continue
			}

			projection := wall.UV.Projections[0]
			point = world.Vec3{X: 2, Y: 0, Z: 8}
			assertUVNear(t, evaluateUV(projection.U, point), .25)
			assertUVNear(t, evaluateUV(projection.V, point), .5)
			point.Z -= 4
			assertUVNear(t, evaluateUV(projection.U, point), -.75)
		}
	}
}

func TestCompileMappingLegacyEncodingAndCompletePreflight(t *testing.T) {
	t.Parallel()
	_, legacy := compileUVFixture(t, "version: 4\n"+uvConcaveRoom)
	_, mapped := compileUVFixture(t, "version: 5\n"+uvConcaveRoom)

	legacyBytes, err := world.Encode(legacy)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(legacyBytes, []byte(`"material_mapping"`)) || bytes.Contains(legacyBytes, []byte(`"floor_uv"`)) {
		t.Fatal("old source gained mapping fields")
	}

	mapped.MaterialMapping = nil
	for index := range mapped.Sectors {
		sector := &mapped.Sectors[index]

		sector.FloorUV, sector.CeilingUV = nil, nil
		for wall := range sector.Walls {
			sector.Walls[wall].UV = nil
		}
	}

	withoutUV, err := world.Encode(mapped)
	if err != nil || !bytes.Equal(withoutUV, legacyBytes) {
		t.Fatal("projection compilation altered geometry, depth, portals or identities")
	}

	for _, mutate := range []func(*source.Expanded){
		func(e *source.Expanded) { e.Rooms[0].Boundary[5].UV.Scale.Y = math.NaN() },
		func(e *source.Expanded) { e.Rooms[0].FloorUV = nil },
		func(e *source.Expanded) { e.Rooms[0].FloorUV.Mode = worldsource.UVWrap },
		func(e *source.Expanded) { e.UV = nil },
	} {
		expanded, _ := compileUVFixture(t, "version: 5\n"+uvConcaveRoom)
		mutate(&expanded)

		result, err := geometry.Compile(expanded, map[string]uint32{"floor": 1, "ceiling": 2, "wall": 3})
		if err == nil || len(result.Sectors) != 0 || result.MaterialMapping != nil {
			t.Fatal("forged final UV settings published a partial world")
		}
	}
}

func evaluateUV(plane world.UVPlane, point world.Vec3) float64 {
	return plane.X*point.X + plane.Y*point.Y + plane.Z*point.Z + plane.Offset
}

func assertUVNear(t *testing.T, got, want float64) {
	t.Helper()

	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("UV plane result %.12g, want %.12g", got, want)
	}
}

func TestCompilePortalSpanRetainsOwnTopBottomAnchor(t *testing.T) {
	t.Parallel()

	const rooms = `rooms:
  - id: high
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 2, y: 0}, material: wall}
      - {id: doorway, start: {x: 2, y: 0}, end: {x: 2, y: 2}, material: wall}
      - {id: north, start: {x: 2, y: 2}, end: {x: 0, y: 2}, material: wall}
      - {id: west, start: {x: 0, y: 2}, end: {x: 0, y: 0}, material: wall}
    floor: {c: 0}
    ceiling: {c: 8}
    floor_material: floor
    ceiling_material: ceiling
  - id: low
    boundary:
      - {id: south, start: {x: 2, y: 0}, end: {x: 4, y: 0}, material: wall}
      - {id: east, start: {x: 4, y: 0}, end: {x: 4, y: 2}, material: wall}
      - {id: north, start: {x: 4, y: 2}, end: {x: 2, y: 2}, material: wall}
      - {id: doorway, start: {x: 2, y: 2}, end: {x: 2, y: 0}, material: wall}
    floor: {c: 2}
    ceiling: {c: 6}
    floor_material: floor
    ceiling_material: ceiling
connections:
  - {id: opening, a: {room: high, edge: doorway}, b: {room: low, edge: doorway}}
`
	for _, anchor := range []string{"top", "bottom"} {
		_, compiled := compileUVFixture(t, "version: 5\nuv: {mode: wrap, anchor: "+anchor+"}\n"+rooms)
		found := false

		for _, sector := range compiled.Sectors {
			if sector.SourceRoom != "high" {
				continue
			}

			for _, wall := range sector.Walls {
				if wall.SourceEdge != "doorway" {
					continue
				}

				found = true

				if wall.Portal < 0 {
					t.Fatal("mapping changed portal connectivity")
				}

				point := world.Vec3{X: 2, Y: 1, Z: 6}
				if anchor == "bottom" {
					point.Z = 2
				}
				// The adjoining portal clips at z2/z6. UV zero remains on the
				// original high room's own z0/z8, never the clipped span.
				assertUVNear(t, evaluateUV(wall.UV.Projections[0].V, point), 2)
			}
		}

		if !found {
			t.Fatal("mapped portal wall was lost")
		}
	}
}
