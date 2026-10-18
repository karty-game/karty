//go:build ignore

// Offline asset preparation and seed authoring. Runtime retains sector cohorts,
// never the candidate positions inspected here or a ground-cover density mask.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/karty-game/karty-sdk/format/worldterrain"
)

const (
	coverRoot   = "samples/terrain"
	coverData   = "scripts/terrain/data"
	coverAssets = "scripts/terrain/assets"
)

type coverGroup struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Preset string `json:"preset"`
	Count  int    `json:"count"`
	Seed   uint32 `json:"seed"`
}

func main() {
	var err error
	switch {
	case len(os.Args) == 1:
		err = authorCover()
	case len(os.Args) == 2 && os.Args[1] == "--repair":
		err = repairCover()
	default:
		err = fmt.Errorf("usage: author-groundcover [--repair]")
	}
	if err != nil {
		panic(err)
	}
}

// Repair preserves authored per-sector counts and every valid cohort after a
// local terrain edit. Only seeds whose roots no longer fit are replaced.
func repairCover() error {
	data, err := os.ReadFile(filepath.Join(coverRoot, "levels/ground/terrain.bin"))
	if err != nil {
		return err
	}
	terrain, err := worldterrain.Decode(data)
	if err != nil {
		return err
	}
	data, err = os.ReadFile(filepath.Join(coverData, "groundcover-groups.json"))
	if err != nil {
		return err
	}
	var groups []coverGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return err
	}
	if len(groups) == 0 || len(groups) > 1024 {
		return fmt.Errorf("invalid retained ground-cover cohort count")
	}
	reaches := map[string]float64{"coastal-grass": 0, "coastal-tall-grass": .35, "coastal-red-fruit": .4, "coastal-shrub": 1.35}
	type seedKey struct {
		x, y   int
		preset string
		seed   uint32
	}
	used := make(map[seedKey]bool, len(groups))
	total := 76
	for _, group := range groups {
		if _, ok := reaches[group.Preset]; !ok || group.X < 1 || group.X > 4 || group.Y < 3 || group.Y > 4 || group.Count < 1 ||
			group.Count > 32 {
			return fmt.Errorf("invalid retained ground-cover cohort: %+v", group)
		}
		used[seedKey{group.X, group.Y, group.Preset, group.Seed}] = true
		total += group.Count
	}
	if len(groups)+10 > 1024 || total*2 > 16384 {
		return fmt.Errorf("retained ground-cover cohorts exceed layer or card limit")
	}
	repaired := 0
	for index, group := range groups {
		fits := func(seed uint32) bool {
			for ordinal := range group.Count {
				x, y := coverPosition(seed, ordinal, group.X, group.Y)
				if !groundAllows(&terrain, x, y, reaches[group.Preset], 128) {
					return false
				}
			}
			return true
		}
		if fits(group.Seed) {
			continue
		}
		seed := group.Seed
		for attempt := uint64(0); ; attempt++ {
			if attempt >= 4000000000 {
				return fmt.Errorf("repair seed search exhausted for %s sector %d/%d", group.Preset, group.X, group.Y)
			}
			seed++
			key := seedKey{group.X, group.Y, group.Preset, seed}
			if !used[key] && fits(seed) {
				groups[index].Seed = seed
				used[key] = true
				repaired++
				break
			}
		}
	}
	data, err = json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(coverData, "groundcover-groups.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("Repaired %d of %d ground-cover cohorts; counts and valid seeds preserved\n", repaired, len(groups))
	return nil
}

func authorCover() error {
	for _, asset := range []struct{ master, name string }{
		{"grass-patch-master-v4.png", "grass-patch-v4"},
		{"tall-grass-master-v1.png", "tall-grass-v1"},
		{"red-fruit-master-v1.png", "red-fruit-v1"},
		{"seaweed-short-master-v1.png", "seaweed-short-v1"},
		{"seaweed-tall-master-v1.png", "seaweed-tall-v1"},
	} {
		if err := prepareCover(asset.master, asset.name); err != nil {
			return err
		}
	}
	encoded, err := os.ReadFile(filepath.Join(coverRoot, "levels/ground/terrain.bin"))
	if err != nil {
		return err
	}
	terrain, err := worldterrain.Decode(encoded)
	if err != nil {
		return err
	}
	var cached []coverGroup
	if data, err := os.ReadFile(filepath.Join(coverData, "groundcover-groups.json")); err == nil {
		if json.Unmarshal(data, &cached) != nil {
			cached = nil
		}
	}
	var groups []coverGroup
	counts := make(map[string]int)
	for _, species := range []struct {
		name                     string
		reach                    float64
		slots, cohort, perSector int
	}{{"coastal-grass", 0, 53, 0, 0}, {"coastal-tall-grass", .35, 4, 6, 24}, {"coastal-red-fruit", .4, 1, 3, 3}, {"coastal-shrub", 1.35, 1, 3, 3}} {
		// Quarter-metre conservative authoring lookup, discarded after choosing
		// seeds. The immutable terrain's existing paint decides admissibility.
		const grid = 512
		allowed := make([]bool, grid*grid)
		for y := range grid {
			for x := range grid {
				allowed[y*grid+x] = groundAllows(&terrain, -64+(float64(x)+.5)/4, -64+(float64(y)+.5)/4, species.reach, 128)
			}
		}
		for y := 3; y <= 4; y++ {
			for x := 1; x <= 4; x++ {
				valid := 0
				for j := range 128 {
					for i := range 128 {
						if allowed[((y-1)*128+j)*grid+(x-1)*128+i] {
							valid++
						}
					}
				}
				fraction := float64(valid) / (128 * 128)
				if fraction < .025 {
					continue
				}
				remaining := species.perSector
				if species.name == "coastal-grass" {
					// Double the previous strict planting area's actual root count.
					previous := 0
					for j := range 128 {
						for i := range 128 {
							if groundAllows(&terrain, -64+float64((x-1)*128+i)/4+.125, -64+float64((y-1)*128+j)/4+.125, .8, 166) {
								previous++
							}
						}
					}
					remaining = 2 * int(math.Round(float64(previous)/16*1.5))
				}
				// Reuse retained cohorts only after replaying every GPU root
				// against the current terrain and requested composition.
				var reusable []coverGroup
				retained := 0
				validCache := true
				for _, group := range cached {
					if group.X != x || group.Y != y || group.Preset != species.name {
						continue
					}
					reusable = append(reusable, group)
					retained += group.Count
					if group.Count <= 0 || group.Count > 32 {
						validCache = false
						break
					}
					for ordinal := range group.Count {
						px, py := coverPosition(group.Seed, ordinal, x, y)
						if !groundAllows(&terrain, px, py, species.reach, 128) {
							validCache = false
							break
						}
					}
				}
				if validCache && retained == remaining && len(reusable) > 0 && len(reusable) <= species.slots {
					groups = append(groups, reusable...)
					counts[species.name] += retained
					fmt.Printf("%s sector %d/%d: reused %d verified cohorts\n", species.name, x, y, len(reusable))
					continue
				}
				seed := uint32(100000 + y*10000 + x*1000)
				if species.name != "coastal-grass" {
					seed += uint32(1000000 * len(counts))
				}
				slots := species.slots
				attempts := 0
				for remaining > 0 {
					count := min(species.cohort, remaining)
					if species.name == "coastal-grass" {
						count = (remaining + slots - 1) / slots
					}
					seed++
					attempts++
					if uint64(attempts) > 4000000000 {
						return fmt.Errorf("seed search exhausted for %s sector %d/%d", species.name, x, y)
					}
					fits := true
					for ordinal := range count {
						px, py := coverPosition(seed, ordinal, x, y)
						ix, iy := int((px+64)*4), int((py+64)*4)
						if ix < 0 || iy < 0 || ix >= grid || iy >= grid || !allowed[iy*grid+ix] {
							fits = false
							break
						}
					}
					if !fits {
						continue
					}
					// Verify actual roots too: the coarse offline lookup is only a search accelerator.
					for ordinal := range count {
						px, py := coverPosition(seed, ordinal, x, y)
						if !groundAllows(&terrain, px, py, species.reach, 128) {
							fits = false
							break
						}
					}
					if !fits {
						continue
					}
					groups = append(groups, coverGroup{x, y, species.name, count, seed})
					remaining -= count
					slots--
					counts[species.name] += count
				}
				fmt.Printf("%s sector %d/%d: %d seed attempts\n", species.name, x, y, attempts)
			}
		}
	}
	total := 76
	for _, count := range counts {
		total += count
	}
	if len(groups)+10 > 1024 || total*2 > 16384 {
		return fmt.Errorf("ground cover exceeds existing cohort/card bounds")
	}
	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(coverData, "groundcover-groups.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("Authored %v in %d seeded sector cohorts; no runtime mask or plant positions\n", counts, len(groups))
	return nil
}

func groundAllows(terrain *worldterrain.Document, x, y, reach float64, minSoil int) bool {
	for _, offset := range [][2]float64{{0, 0}, {reach, 0}, {-reach, 0}, {0, reach}, {0, -reach}} {
		px, py := x+offset[0], y+offset[1]
		height, solid := worldterrain.HeightAt(terrain, px, py)
		if !solid || height < .7 {
			return false
		}
		x0, ok0 := worldterrain.HeightAt(terrain, px-.3, py)
		x1, ok1 := worldterrain.HeightAt(terrain, px+.3, py)
		y0, ok2 := worldterrain.HeightAt(terrain, px, py-.3)
		y1, ok3 := worldterrain.HeightAt(terrain, px, py+.3)
		if !ok0 || !ok1 || !ok2 || !ok3 || math.Hypot(x1-x0, y1-y0)/.6 > .7 {
			return false
		}
		soil := 0
		for _, patch := range terrain.Patches {
			gx, gy := (px-float64(terrain.Origin[0]))/float64(terrain.CellSize), (py-float64(terrain.Origin[1]))/float64(terrain.CellSize)
			if gx < float64(patch.X) || gy < float64(patch.Y) || gx > float64(patch.X+patch.Columns) || gy > float64(patch.Y+patch.Rows) {
				continue
			}
			u := int(math.Round((gx - float64(patch.X)) / float64(patch.Columns) * float64(patch.Weights.Width-1)))
			v := int(math.Round((gy - float64(patch.Y)) / float64(patch.Rows) * float64(patch.Weights.Height-1)))
			for channel, material := range patch.Palette {
				if material == 2 {
					soil = int(patch.Weights.Pixels[(v*int(patch.Weights.Width)+u)*4+channel])
				}
			}
			break
		}
		if soil < minSoil {
			return false
		}
	}
	return true
}

func coverHash(value uint32) uint32 {
	value ^= value >> 16
	value *= 2246822519
	value ^= value >> 13
	value *= 3266489917
	return value ^ (value >> 16)
}

func coverPosition(seed uint32, ordinal, x, y int) (float64, float64) {
	identity := coverHash(seed ^ coverHash(uint32(ordinal)+977))
	random := func(salt uint32) float32 { return float32(coverHash(identity^salt)>>8) / 16777216 }
	// Match shader float precision and its existing 32 m sector bounds.
	return float64(float32(-64+(x-1)*32) + 32*random(1)), float64(float32(-64+(y-1)*32) + 32*random(2))
}

func prepareCover(master, name string) error {
	input, err := os.Open(filepath.Join(coverAssets, master))
	if err != nil {
		return err
	}
	source, err := png.Decode(input)
	input.Close()
	if err != nil {
		return err
	}
	bounds := source.Bounds()
	trim := image.Rectangle{Min: bounds.Max, Max: bounds.Min}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := source.At(x, y).RGBA()
			if alpha >= 128*257 {
				trim.Min.X, trim.Min.Y = min(trim.Min.X, x), min(trim.Min.Y, y)
				trim.Max.X, trim.Max.Y = max(trim.Max.X, x+1), max(trim.Max.Y, y+1)
			}
		}
	}
	if trim.Empty() {
		return fmt.Errorf("grass source has no opaque blades")
	}
	albedo := image.NewNRGBA(image.Rect(0, 0, trim.Dx(), trim.Dy()))
	for y := range trim.Dy() {
		for x := range trim.Dx() {
			pixel := color.NRGBAModel.Convert(source.At(x+trim.Min.X, y+trim.Min.Y)).(color.NRGBA)
			if pixel.A < 64 {
				pixel = color.NRGBA{}
			}
			albedo.SetNRGBA(x, y, pixel)
		}
	}
	normals := image.NewNRGBA(albedo.Bounds())
	luma := func(x, y int) float64 {
		pixel := albedo.NRGBAAt(max(0, min(albedo.Bounds().Dx()-1, x)), max(0, min(albedo.Bounds().Dy()-1, y)))
		if pixel.A < 128 {
			return .35
		}
		return (.2126*float64(pixel.R) + .7152*float64(pixel.G) + .0722*float64(pixel.B)) / 255
	}
	step := max(1, albedo.Bounds().Dx()/256)
	encode := func(value float64) uint8 { return uint8(math.Round((value*.5 + .5) * 255)) }
	for y := range albedo.Bounds().Dy() {
		for x := range albedo.Bounds().Dx() {
			normal := color.NRGBA{128, 128, 255, 255}
			if albedo.NRGBAAt(x, y).A >= 128 {
				nx, ny := (luma(x-step, y)-luma(x+step, y))*.6, (luma(x, y-step)-luma(x, y+step))*.6
				inverse := 1 / math.Sqrt(1+nx*nx+ny*ny)
				normal = color.NRGBA{encode(nx * inverse), encode(ny * inverse), encode(inverse), 255}
			}
			normals.SetNRGBA(x, y, normal)
		}
	}
	for name, data := range map[string]image.Image{name + ".png": albedo, name + "-normal.png": normals} {
		file, err := os.Create(filepath.Join(coverRoot, "levels/ground", name))
		if err != nil {
			return err
		}
		if err := png.Encode(file, data); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	fmt.Printf("Prepared %s albedo/normals %dx%d; master unchanged\n", name, trim.Dx(), trim.Dy())
	return nil
}
