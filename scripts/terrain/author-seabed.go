//go:build ignore

// Offline seed selection for submerged plants.
// Runtime receives only sector cohorts; no positions or density mask are kept.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/karty-game/karty-sdk/format/worldterrain"
)

type detailGroup struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Preset string `json:"preset"`
	Count  int    `json:"count"`
	Seed   uint32 `json:"seed"`
}

func main() {
	data, err := os.ReadFile("samples/terrain/levels/ground/terrain.bin")
	if err != nil {
		panic(err)
	}
	terrain, err := worldterrain.Decode(data)
	if err != nil {
		panic(err)
	}
	var groups []detailGroup
	counts := map[string]int{}
	for kind, species := range []struct {
		name                                   string
		minHeight, maxHeight, reach, perSector float64
	}{
		{"coastal-seagrass", -7, -1.05, .45, 48},
		{"coastal-ribbon-kelp", -8, -2.2, .55, 24},
	} {
		allows := func(x, y float64) bool {
			for _, offset := range [][2]float64{{0, 0}, {species.reach, 0}, {-species.reach, 0}, {0, species.reach}, {0, -species.reach}} {
				px, py := x+offset[0], y+offset[1]
				h, solid := worldterrain.HeightAt(&terrain, px, py)
				if !solid || h < species.minHeight || h > species.maxHeight {
					return false
				}
				x0, a := worldterrain.HeightAt(&terrain, px-.3, py)
				x1, b := worldterrain.HeightAt(&terrain, px+.3, py)
				y0, c := worldterrain.HeightAt(&terrain, px, py-.3)
				y1, d := worldterrain.HeightAt(&terrain, px, py+.3)
				if !a || !b || !c || !d || math.Hypot(x1-x0, y1-y0)/.6 > .8 {
					return false
				}
			}
			return true
		}
		for y := 1; y <= 4; y++ {
			for x := 1; x <= 4; x++ {
				valid := 0
				for j := 0; j < 64; j++ {
					for i := 0; i < 64; i++ {
						if allows(float64(-64+(x-1)*32)+float64(i)*.5+.25, float64(-64+(y-1)*32)+float64(j)*.5+.25) {
							valid++
						}
					}
				}
				fraction := float64(valid) / 4096
				if fraction < .04 {
					continue
				}
				count := min(32, int(math.Round(species.perSector*fraction)))
				// Keep the offline search bounded in narrow shoreline sectors.
				// Larger open seabed sectors can retain larger seed prefixes.
				if fraction < .999 {
					count = min(count, max(1, int(math.Log(.00001)/math.Log(fraction))))
				}
				if count < 1 {
					continue
				}
				seed := uint32(7000000 + kind*1000000 + y*10000 + x*1000)
				found := false
				for attempt := 0; attempt < 4000000; attempt++ {
					seed++
					fits := true
					for ordinal := 0; ordinal < count; ordinal++ {
						px, py := detailPosition(seed, ordinal, x, y)
						if !allows(px, py) {
							fits = false
							break
						}
					}
					if fits {
						found = true
						break
					}
				}
				if !found {
					panic(fmt.Sprintf("bounded detail seed search exhausted: %s %d/%d", species.name, x, y))
				}
				groups = append(groups, detailGroup{x, y, species.name, count, seed})
				counts[species.name] += count
			}
		}
	}
	data, err = json.MarshalIndent(groups, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile("scripts/terrain/data/seabed-groups.json", append(data, '\n'), 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("Authored seabed/detail cohorts: %v (%d layers); GPU derives roots at runtime\n", counts, len(groups))
}

func detailPosition(seed uint32, ordinal, x, y int) (float64, float64) {
	hash := func(v uint32) uint32 {
		v ^= v >> 16
		v *= 2246822519
		v ^= v >> 13
		v *= 3266489917
		return v ^ (v >> 16)
	}
	identity := hash(seed ^ hash(uint32(ordinal)+977))
	return float64(
			float32(-64+(x-1)*32) + 32*float32(hash(identity^1)>>8)/16777216,
		), float64(
			float32(-64+(y-1)*32) + 32*float32(hash(identity^2)>>8)/16777216,
		)
}
