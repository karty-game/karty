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

type terrainSeabedGroup struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Count  int    `json:"count"`
	Seed   uint32 `json:"seed"`
	Preset string `json:"preset"`
}

func TestTerrainSampleSeabedKeepsPlantsSubmergedAndBounded(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "samples", "terrain")

	data, err := os.ReadFile(filepath.Join(root, "levels", "ground", "terrain.bin"))
	if err != nil {
		t.Fatal(err)
	}

	terrain, err := worldterrain.Decode(data)
	if err != nil {
		t.Fatal(err)
	}

	var details, land []terrainSeabedGroup

	for name, target := range map[string]*[]terrainSeabedGroup{"seabed-groups.json": &details, "groundcover-groups.json": &land} {
		data, err = os.ReadFile(filepath.Join("..", "..", "scripts", "terrain", "data", name))
		if err != nil {
			t.Fatal(err)
		}

		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}

	counts := make(map[string]int)

	for _, cohort := range details {
		assertTerrainSampleSubmergedCohort(t, &terrain, cohort)
		counts[cohort.Preset] += cohort.Count
	}

	for _, name := range []string{"coastal-seagrass", "coastal-ribbon-kelp"} {
		if counts[name] == 0 {
			t.Fatalf("missing %s", name)
		}
	}

	total := 76

	for _, cohort := range append(land, details...) {
		total += cohort.Count
	}

	if len(land)+len(details)+10 > worldvegetation.MaxLayers || total*2 > worldvegetation.MaxInstances {
		t.Fatal("combined land/seabed decoration exceeds existing bounds")
	}
}

func assertTerrainSampleSubmergedCohort(t *testing.T, terrain *worldterrain.Document, cohort terrainSeabedGroup) {
	t.Helper()

	hash := func(value uint32) uint32 {
		value ^= value >> 16
		value *= 2246822519
		value ^= value >> 13
		value *= 3266489917

		return value ^ (value >> 16)
	}

	if cohort.X < 1 || cohort.X > 4 || cohort.Y < 1 || cohort.Y > 4 || cohort.Count < 1 || cohort.Count > 32 {
		t.Fatalf("unbounded detail cohort: %+v", cohort)
	}

	minHeight, maxHeight, maxTip, reach := -8.0, 26.0, 0.0, .25

	switch cohort.Preset {
	case "coastal-seagrass":
		minHeight, maxHeight, maxTip, reach = -7, -1.05, .7, .45
	case "coastal-ribbon-kelp":
		maxHeight, maxTip, reach = -2.2, 1.65, .55
	default:
		t.Fatalf("unknown seabed/detail species %s", cohort.Preset)
	}

	for ordinal := range cohort.Count {
		identity := hash(cohort.Seed ^ hash(uint32(ordinal)+977))
		worldX := float64(float32(-64+(cohort.X-1)*32) + 32*float32(hash(identity^1)>>8)/16777216)
		worldY := float64(float32(-64+(cohort.Y-1)*32) + 32*float32(hash(identity^2)>>8)/16777216)

		for _, offset := range [][2]float64{{0, 0}, {reach, 0}, {-reach, 0}, {0, reach}, {0, -reach}} {
			height, solid := worldterrain.HeightAt(terrain, worldX+offset[0], worldY+offset[1])
			if !solid || height < minHeight || height > maxHeight || (maxTip > 0 && height+maxTip > -.25) {
				t.Fatalf(
					"%s seed %d root %d escaped submerged solid terrain: height=%g solid=%v",
					cohort.Preset,
					cohort.Seed,
					ordinal,
					height,
					solid,
				)
			}
		}

		x0, a := worldterrain.HeightAt(terrain, worldX-.3, worldY)
		x1, b := worldterrain.HeightAt(terrain, worldX+.3, worldY)
		y0, c := worldterrain.HeightAt(terrain, worldX, worldY-.3)

		y1, d := worldterrain.HeightAt(terrain, worldX, worldY+.3)
		if !a || !b || !c || !d || math.Hypot(x1-x0, y1-y0)/.6 > .8 {
			t.Fatal("detail root left gently sloping terrain")
		}
	}
}
