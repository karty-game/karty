package levelbuild

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldterrain"
)

func fixedTerrainFixture() worldterrain.Document {
	image := func(width, height int, rgba [4]byte) worldterrain.Image {
		return worldterrain.Image{Width: uint16(width), Height: uint16(height), Pixels: bytes.Repeat(rgba[:], width*height)}
	}

	document := worldterrain.Document{Version: worldterrain.Version, CellSize: 1, Columns: 5, Rows: 3,
		Heights: make([]float32, 15), Normals: make([][3]float32, 15), Holes: make([]byte, 8), ExteriorSectors: []uint16{0, 1},
		Materials: []worldterrain.Material{{Scale: 1, NormalStrength: 1,
			Albedo: image(256, 256, [4]byte{180, 160, 140, 255}), Normal: image(256, 256, [4]byte{128, 128, 255, 255})}},
		Patches:   []worldterrain.Patch{{Columns: 4, Rows: 2, Weights: image(5, 3, [4]byte{255, 0, 0, 0})}},
		FarColour: image(5, 3, [4]byte{180, 160, 140, 255})}
	for index := range document.Normals {
		document.Normals[index][2] = 1
	}

	return document
}

func TestFixedTerrainCropRejectsLatticeRoundingChanges(t *testing.T) {
	t.Parallel()

	terrain := fixedTerrainFixture()
	terrain.CellSize = .1
	terrain.Columns = 9
	terrain.Heights = make([]float32, 27)

	terrain.Normals = make([][3]float32, 27)
	for index := range terrain.Normals {
		terrain.Normals[index][2] = 1
	}

	terrain.Holes = make([]byte, 16)
	terrain.Patches[0].Columns = 8

	owners := []int{0, 0, 0, 1, 1, 1, 1, 1, 0, 0, 0, 1, 1, 1, 1, 1}
	if _, err := cropChunkTerrain(&terrain, owners, 1, []int{1}); !errors.Is(err, worldterrain.ErrTerrain) ||
		!strings.Contains(err.Error(), "cannot preserve the exact X lattice") {
		t.Fatal("terrain crop silently changed shared vertex positions", err)
	}
}

func TestFixedTerrainPartitionRejectsCellsAcrossChunkBoundary(t *testing.T) {
	t.Parallel()

	terrain := fixedTerrainFixture()
	document := world.Document{Version: world.Version}

	for index, interval := range [][2]float64{{0, 1.5}, {1.5, 4}} {
		points := []world.Vec2{{X: interval[0]}, {X: interval[1]}, {X: interval[1], Y: 2}, {X: interval[0], Y: 2}}

		sector := world.Sector{Sky: true, Ceiling: world.Plane{C: 4}}
		for edge, point := range points {
			sector.Walls = append(sector.Walls, world.Wall{Start: point, End: points[(edge+1)%4], Portal: -1})
		}

		if index == 0 {
			sector.Walls[1].Portal = 1
		} else {
			sector.Walls[3].Portal = 0
		}

		document.Sectors = append(document.Sectors, sector)
	}

	if err := worldterrain.ValidateForWorld(&terrain, &document); err != nil {
		t.Fatal("full terrain should be valid before partition", err)
	}

	chunks := []worldchunk.PlannedChunk{{ID: 0, Sectors: []int{0}}, {ID: 1, Sectors: []int{1}}}
	if _, err := partitionWorldChunkTerrain(&terrain, &document, chunks); !errors.Is(err, worldterrain.ErrTerrain) ||
		!strings.Contains(err.Error(), "boundary must align with the terrain grid") {
		t.Fatal("CLI published terrain that runtime chunk admission would reject", err)
	}
}
