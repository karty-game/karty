package levelbuild

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldterrain"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
)

func TestTerrainSampleGroundCoverKeepsRootsOnPaintedSoil(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "samples", "terrain")

	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "terrain", "data", "groundcover-groups.json"))
	if err != nil {
		t.Fatal(err)
	}

	var groups []struct {
		X      int    `json:"x"`
		Y      int    `json:"y"`
		Count  int    `json:"count"`
		Seed   uint32 `json:"seed"`
		Preset string `json:"preset"`
	}
	if err := json.Unmarshal(data, &groups); err != nil {
		t.Fatal(err)
	}

	data, err = os.ReadFile(filepath.Join(root, "levels", "ground", "terrain.bin"))
	if err != nil {
		t.Fatal(err)
	}

	terrain, err := worldterrain.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	// Independent replay of the shader's seed/ordinal contract. No authored
	// world positions or mask can silently replace the runtime GPU placement.
	hash := func(value uint32) uint32 {
		value ^= value >> 16
		value *= 2246822519
		value ^= value >> 13
		value *= 3266489917

		return value ^ (value >> 16)
	}
	counts := make(map[string]int)

	for _, group := range groups {
		if group.X < 1 || group.X > 4 || group.Y < 3 || group.Y > 4 || group.Count < 1 || group.Count > 32 {
			t.Fatalf("unbounded ground-cover cohort: %+v", group)
		}

		if group.Preset != "coastal-grass" && group.Preset != "coastal-shrub" && group.Preset != "coastal-tall-grass" &&
			group.Preset != "coastal-red-fruit" {
			t.Fatalf("unexpected ground-cover species: %s", group.Preset)
		}

		counts[group.Preset] += group.Count

		for ordinal := range group.Count {
			identity := hash(group.Seed ^ hash(uint32(ordinal)+977))
			worldX := float64(float32(-64+(group.X-1)*32) + 32*float32(hash(identity^1)>>8)/16777216)
			worldY := float64(float32(-64+(group.Y-1)*32) + 32*float32(hash(identity^2)>>8)/16777216)

			height, solid := worldterrain.HeightAt(&terrain, worldX, worldY)
			if !solid || height < .7 || terrainSampleSoil(&terrain, worldX, worldY) < 128 {
				t.Fatalf(
					"%s seed %d ordinal %d left painted land at %g/%g: height=%g solid=%v",
					group.Preset,
					group.Seed,
					ordinal,
					worldX,
					worldY,
					height,
					solid,
				)
			}
		}
	}

	if counts["coastal-grass"] != 6400 || counts["coastal-tall-grass"] != 192 || counts["coastal-red-fruit"] != 24 ||
		counts["coastal-shrub"] != 24 {
		t.Fatalf("ground cover lost doubled grass / sparse tall grass / rarer red fruit composition: %v", counts)
	}

	total := 76

	for _, count := range counts {
		total += count
	}

	if len(groups)+10 >= worldvegetation.MaxLayers || total*2 > worldvegetation.MaxInstances {
		t.Fatal("complete sample ground cover exceeds the unchanged layer/card budget")
	}
}

func terrainSampleSoil(terrain *worldterrain.Document, worldX, worldY float64) byte {
	gridX, gridY := (worldX-float64(terrain.Origin[0]))/float64(
		terrain.CellSize,
	), (worldY-float64(terrain.Origin[1]))/float64(
		terrain.CellSize,
	)

	for _, patch := range terrain.Patches {
		if gridX < float64(patch.X) || gridY < float64(patch.Y) || gridX > float64(patch.X+patch.Columns) ||
			gridY > float64(patch.Y+patch.Rows) {
			continue
		}

		u := int(math.Round((gridX - float64(patch.X)) / float64(patch.Columns) * float64(patch.Weights.Width-1)))
		v := int(math.Round((gridY - float64(patch.Y)) / float64(patch.Rows) * float64(patch.Weights.Height-1)))

		for channel, material := range patch.Palette {
			if material == 2 {
				return patch.Weights.Pixels[(v*int(patch.Weights.Width)+u)*4+channel]
			}
		}
	}

	return 0
}
