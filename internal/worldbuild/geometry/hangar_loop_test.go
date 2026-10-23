package geometry_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
	"github.com/pelletier/go-toml/v2"
)

func TestHangarExtractionLoop(t *testing.T) {
	t.Parallel()

	directory := filepath.Join("..", "..", "..", "samples", "hangar", "levels", "hangar")

	data, err := os.ReadFile(filepath.Join(directory, "level.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var manifest struct{ Textures []struct{ Name string } }

	if err := toml.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}

	materials := make(map[string]uint32, len(manifest.Textures))
	for index, texture := range manifest.Textures {
		materials[texture.Name] = uint32(index + 1)
	}

	expanded, err := source.Load(directory, "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	document, err := geometry.Compile(expanded, materials)
	if err != nil {
		t.Fatal(err)
	}

	assertHangarExtractionApproach(t, document)
	assertHangarPoolRamps(t, document)
	assertHangarRampMaterialMapping(t, document)

	// Inspect the authored apertures after convex decomposition. Both outgoing
	// links must survive and land in the room left of arrival, rather than the
	// ordinary egress corridor beside extraction.
	portals := make(map[string]int)

	for sectorIndex, sector := range document.Sectors {
		for _, wall := range sector.Walls {
			if wall.SourceEdge != "loop-portal" {
				continue
			}

			portals[sector.SourceRoom] = sectorIndex
			assertHangarLoopAperture(t, document, sectorIndex, wall)
		}
	}

	if len(portals) != 2 {
		t.Fatalf("loop endpoints: %+v", portals)
	}

	bay, bayOK := portals["window-bay"]
	extraction, extractionOK := portals["extraction"]

	if !bayOK || !extractionOK {
		t.Fatalf("wrong loop rooms: %+v", portals)
	}

	if !hangarReachable(document, bay, "arrival") || !hangarReachable(document, extraction, "arrival") {
		t.Fatal("ordinary passages must still connect both ends to arrival without using the loop")
	}
}

func assertHangarLoopAperture(t *testing.T, document world.Document, sectorIndex int, wall world.Wall) {
	t.Helper()

	if wall.Portal < 0 || wall.PortalWall == 0 {
		t.Fatalf("closed loop aperture: %+v", wall)
	}

	sector := document.Sectors[sectorIndex]
	target := document.Sectors[wall.Portal]
	reciprocal := target.Walls[wall.PortalWall-1]

	if reciprocal.Portal != int32(sectorIndex) || reciprocal.SourceEdge != "loop-portal" {
		t.Fatalf("loop has no reciprocal aperture: %+v", reciprocal)
	}

	dx, dy := wall.End.X-wall.Start.X, wall.End.Y-wall.Start.Y
	tx, ty := reciprocal.End.X-reciprocal.Start.X, reciprocal.End.Y-reciprocal.Start.Y

	if math.Abs(math.Hypot(dx, dy)-3) > 1e-9 || math.Abs(math.Hypot(tx, ty)-3) > 1e-9 || dx*tx+dy*ty != 0 {
		t.Fatal("loop must retain matching three-meter apertures and a quarter turn")
	}

	if min(sector.Ceiling.C-sector.Floor.C, target.Ceiling.C-target.Floor.C) < 1.7 {
		t.Fatal("loop has insufficient player clearance")
	}
}

func hangarReachable(document world.Document, start int, room string) bool {
	seen := make([]bool, len(document.Sectors))
	queue := []int{start}
	seen[start] = true

	for len(queue) > 0 {
		sector := document.Sectors[queue[0]]
		queue = queue[1:]

		if sector.SourceRoom == room {
			return true
		}

		for _, wall := range sector.Walls {
			if wall.Portal < 0 || wall.SourceEdge == "loop-portal" || seen[wall.Portal] {
				continue
			}

			seen[wall.Portal] = true
			queue = append(queue, int(wall.Portal))
		}
	}

	return false
}

// The raised extraction platform must leave the full walking box a route to
// the portal. Solid tops are not walkable floors in the sample controller.
func assertHangarExtractionApproach(t *testing.T, document world.Document) {
	t.Helper()

	if document.StaticSolids == nil {
		t.Fatal("Hangar collision solids are missing")
	}

	for _, solid := range document.StaticSolids.Items {
		if solid.Bottom.C >= 1.7 || solid.Top.C <= 0 || !solid.Collision {
			continue
		}

		minX, maxX, minY, maxY := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
		for _, point := range solid.Footprint {
			minX, maxX, minY, maxY = min(minX, point.X), max(maxX, point.X), min(minY, point.Y), max(maxY, point.Y)
		}

		if minX < 29.8 && maxX > 29.2 && minY < 46 && maxY > 43 {
			t.Fatalf("solid %s blocks the portal approach", solid.ID)
		}
	}
}

// Keep the authored ramp checks without generating a second runtime world map.
func assertHangarPoolRamps(t *testing.T, document world.Document) {
	t.Helper()

	for _, route := range []struct {
		name                         string
		x0, y0, x1, y1, top, ceiling float64
	}{
		{"trench-stair-", 40, 22, 46, 22, .2, 3.2},
		{"west-trench-stair-", 23, 26, 23, 32, 0, 3.1},
	} {
		count := 0

		for _, cell := range document.Sectors {
			if !strings.HasPrefix(cell.ID, route.name) {
				continue
			}

			count++

			if math.Abs(hangarPlaneZ(cell.Floor, route.x0, route.y0)+3.5) > .00001 ||
				math.Abs(hangarPlaneZ(cell.Floor, route.x1, route.y1)-route.top) > .00001 ||
				math.Abs(hangarPlaneZ(cell.Ceiling, route.x1, route.y1)-route.ceiling) > .00001 {
				t.Fatalf("discontinuous ramp plane: %+v", cell)
			}

			for _, wall := range cell.Walls {
				if hangarPlaneZ(cell.Ceiling, wall.Start.X, wall.Start.Y)-hangarPlaneZ(cell.Floor, wall.Start.X, wall.Start.Y) < 2 {
					t.Fatalf("ramp lacks player headroom: %s", cell.ID)
				}
			}
		}

		if count != 6 {
			t.Fatalf("%s: %d strips", route.name, count)
		}
	}
}

func hangarPlaneZ(plane world.Plane, x, y float64) float64 {
	return plane.A*x + plane.B*y + plane.C
}

// Flat ramp surfaces opt into one world projection; intentional triplanar
// materials elsewhere keep their authored blend. Physics and portal checks
// above continue to cover the same ramp geometry.
func assertHangarRampMaterialMapping(t *testing.T, document world.Document) {
	t.Helper()

	assertPlanar := func(mapping *world.SurfaceUV) {
		t.Helper()

		if mapping == nil || len(mapping.Projections) != 1 || len(mapping.Weights) != 1 || mapping.Weights[0] != 1 ||
			mapping.Projections[0] != (world.UVProjection{U: world.UVPlane{X: .5}, V: world.UVPlane{Y: .5}}) {
			t.Fatalf("ramp lost its single world-anchored, two-meter material repeat: %+v", mapping)
		}
	}

	for _, sector := range document.Sectors {
		if strings.HasPrefix(sector.ID, "trench-stair-") || strings.HasPrefix(sector.ID, "west-trench-stair-") {
			assertPlanar(sector.FloorUV)
		}
	}

	cornices := 0

	for _, solid := range document.StaticSolids.Items {
		if strings.HasPrefix(solid.ID, "trench-ramp-") && strings.HasSuffix(solid.ID, "-cornice") {
			cornices++

			assertPlanar(solid.TopUV)
			assertPlanar(solid.BottomUV)
		}
	}

	if cornices != 4 {
		t.Fatalf("ramp cornices: %d, want 4", cornices)
	}
}
