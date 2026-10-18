package levelbuild

import (
	"math"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldterrain"
)

// Exercise the exported heightfield and paint, rather than only checking that
// the authoring scripts happen to share a named route or a particular plank count.
func assertTerrainSamplePierApproach(t *testing.T, terrain *worldterrain.Document, compiled *world.Document) {
	t.Helper()

	for _, x := range []float64{9.65, 11.4, 13.15} {
		for _, sample := range [][3]float64{{16.1, 1.075, 1.12}, {18.2, 1.04, 1.12}, {21.1, .94, 1.04}} {
			h, solid := worldterrain.HeightAt(terrain, x, sample[0])
			if !solid || h < sample[1] || h > sample[2] {
				t.Fatalf("pier approach must support boarding and bury its toe: %g/%g height=%g solid=%v", x, sample[0], h, solid)
			}
		}
	}

	// A person arriving from the trail follows its tangent straight onto the
	// ramp; the previous transverse gravel stripe is absent beside the timber.

	for _, p := range [][2]float64{{11.4, 20.2}, {11.4, 21.2}, {11.4, 22.3}, {10.9, 23.1}, {9.8, 23.4}, {8.6, 23.2}, {7, 22.6}, {3.5, 20.3}, {0, 18}} {
		h, solid := worldterrain.HeightAt(terrain, p[0], p[1])
		if !solid || math.Abs(h-1) > .08 || terrainSampleMaterial(terrain, p[0], p[1], 3) < 128 {
			t.Fatalf("gravel trail must continuously reach the buried ramp toe: %v height=%g solid=%v", p, h, solid)
		}
	}

	for _, p := range [][2]float64{{8, 18}, {15, 18}} {
		if terrainSampleMaterial(terrain, p[0], p[1], 0) < 192 || terrainSampleMaterial(terrain, p[0], p[1], 3) > 32 {
			t.Fatalf("old transverse gravel stripe remains beside the ramp at %v", p)
		}
	}

	bearing, landing, ramp := false, false, false

	for _, solid := range compiled.StaticSolids.Items {
		switch solid.ID {
		case "pier-walk-bearing":
			bearing = solid.Collision && math.Abs(solid.Top.C-1.07) < .001
		case "pier-beach-ramp-landing":
			landing = !solid.Collision
		case "pier-beach-ramp-bearing":
			ramp = !solid.Collision
		}
	}

	if !bearing || !landing || !ramp {
		t.Fatal("pier requires a colliding deck and a terrain-supported approach without colliding slab end walls")
	}
}

func terrainSampleMaterial(terrain *worldterrain.Document, x, y float64, material uint16) byte {
	gridX, gridY := (x-float64(terrain.Origin[0]))/float64(terrain.CellSize), (y-float64(terrain.Origin[1]))/float64(terrain.CellSize)

	for _, patch := range terrain.Patches {
		if gridX < float64(patch.X) || gridY < float64(patch.Y) || gridX > float64(patch.X+patch.Columns) ||
			gridY > float64(patch.Y+patch.Rows) {
			continue
		}

		u := int(math.Round((gridX - float64(patch.X)) / float64(patch.Columns) * float64(patch.Weights.Width-1)))
		v := int(math.Round((gridY - float64(patch.Y)) / float64(patch.Rows) * float64(patch.Weights.Height-1)))

		for channel, index := range patch.Palette {
			if index == material {
				return patch.Weights.Pixels[(v*int(patch.Weights.Width)+u)*4+channel]
			}
		}
	}

	return 0
}
