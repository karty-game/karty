package geometry_test

import (
	"math"
	"os"
	"path/filepath"
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
