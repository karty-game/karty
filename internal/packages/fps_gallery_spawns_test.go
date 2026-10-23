package packages

import (
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/worldbuild/source"
)

type galleryFootprint struct {
	name                   string
	minX, minY, maxX, maxY float64
}

func (a galleryFootprint) near(b galleryFootprint) bool {
	const clearance = .25

	return a.minX < b.maxX+clearance && a.maxX > b.minX-clearance &&
		a.minY < b.maxY+clearance && a.maxY > b.minY-clearance
}

// Read the helper's actual placement table so changes to props and spawns are
// checked together, rather than testing a duplicate list of expected positions.
func galleryHelperSpawns(t *testing.T) []galleryFootprint {
	t.Helper()

	contents := read(t, filepath.Join(fpsKitSource(t), "examples/crates.go.tmpl"))
	rows := regexp.MustCompile(`\{"([a-z]+)",\s*([^}]+)\}`).FindAllSubmatch(contents, -1)

	if len(rows) != 5 {
		t.Fatal("gallery helper must contain five numeric crate placements")
	}

	result := make([]galleryFootprint, 0, len(rows))
	for _, row := range rows {
		fields := strings.Split(string(row[2]), ",")
		if len(fields) != 6 {
			t.Fatal("gallery placement must specify position, dimensions and mass")
		}

		var values [6]float64

		for index, field := range fields {
			value, err := strconv.ParseFloat(strings.TrimSpace(field), 64)
			if err != nil || value <= 0 {
				t.Fatalf("invalid helper placement %q: %v", row[0], err)
			}

			values[index] = value
		}

		result = append(result, galleryFootprint{string(row[1]),
			values[0] - values[2]/2, values[1] - values[3]/2,
			values[0] + values[2]/2, values[1] + values[3]/2})
	}

	return result
}

func assertFPSKitGallerySpawnClearance(t *testing.T, expanded source.Expanded) {
	t.Helper()

	if len(expanded.Rooms) != 1 || len(expanded.Rooms[0].Boundary) != 4 {
		t.Fatal("spawn clearance check expects the rectangular gallery")
	}

	room := galleryFootprint{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	for _, edge := range expanded.Rooms[0].Boundary {
		room.minX, room.minY = math.Min(room.minX, edge.Start.X), math.Min(room.minY, edge.Start.Y)
		room.maxX, room.maxY = math.Max(room.maxX, edge.Start.X), math.Max(room.maxY, edge.Start.Y)
	}

	obstacles := make([]galleryFootprint, 0, len(expanded.Solids)+6)
	obstacles = append(obstacles, galleryFootprint{"player", 1.65, 1.65, 2.35, 2.35})

	for _, solid := range expanded.Solids {
		bounds := galleryFootprint{solid.ID, math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
		for _, point := range solid.Footprint {
			bounds.minX, bounds.minY = math.Min(bounds.minX, point.X), math.Min(bounds.minY, point.Y)
			bounds.maxX, bounds.maxY = math.Max(bounds.maxX, point.X), math.Max(bounds.maxY, point.Y)
		}

		obstacles = append(obstacles, bounds)
	}

	for _, spawn := range galleryHelperSpawns(t) {
		if spawn.minX <= room.minX+.25 || spawn.minY <= room.minY+.25 ||
			spawn.maxX >= room.maxX-.25 || spawn.maxY >= room.maxY-.25 {
			t.Fatalf("dynamic %s intersects or approaches a gallery wall", spawn.name)
		}

		for _, obstacle := range obstacles {
			if spawn.near(obstacle) {
				t.Fatalf("dynamic %s is too close to gallery %s", spawn.name, obstacle.name)
			}
		}

		obstacles = append(obstacles, spawn)
	}
}
