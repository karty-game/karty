package levelbuild

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldterrain"
	worldgeometry "github.com/karty-game/karty/internal/worldbuild/geometry"
	worldsource "github.com/karty-game/karty/internal/worldbuild/source"
)

func terrainFixture(t *testing.T) (string, manifest, world.Document, *assetBuild, []byte) {
	t.Helper()

	image := func(size int, normal bool) worldterrain.Image {
		pixels := make([]byte, size*size*4)
		for i := 0; i < len(pixels); i += 4 {
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = 80, 120, 60, 255
			if normal {
				pixels[i], pixels[i+1], pixels[i+2] = 128, 128, 255
			}
		}

		return worldterrain.Image{Width: uint16(size), Height: uint16(size), Pixels: pixels}
	}

	weights := image(2, false)
	for i := range weights.Pixels {
		weights.Pixels[i] = 0
	}

	for i := 0; i < len(weights.Pixels); i += 4 {
		weights.Pixels[i] = 255
	}

	terrain := worldterrain.Document{
		Version: worldterrain.Version, CellSize: 1, Columns: 2, Rows: 2,
		Heights: make([]float32, 4), Normals: [][3]float32{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}},
		ExteriorSectors: []uint16{0}, Holes: []byte{0}, FarColour: image(2, false),
		Materials: []worldterrain.Material{{Albedo: image(256, false), Normal: image(256, true), Scale: 2, NormalStrength: 1}},
		Patches:   []worldterrain.Patch{{Columns: 1, Rows: 1, Weights: weights}},
	}

	encoded, err := worldterrain.Encode(terrain)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "terrain.bin"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}

	definition := manifest{World: worldSettings{Source: "world.yaml"}, Terrain: &terrainBuildSettings{Source: "terrain.bin"}}
	compiled := world.Document{Sectors: []world.Sector{{Sky: true, Ceiling: world.Plane{C: 4}, Walls: []world.Wall{
		{Start: world.Vec2{X: -1, Y: -1}, End: world.Vec2{X: 2, Y: -1}, Portal: -1},
		{Start: world.Vec2{X: 2, Y: -1}, End: world.Vec2{X: 2, Y: 2}, Portal: -1},
		{Start: world.Vec2{X: 2, Y: 2}, End: world.Vec2{X: -1, Y: 2}, Portal: -1},
		{Start: world.Vec2{X: -1, Y: 2}, End: world.Vec2{X: -1, Y: -1}, Portal: -1},
	}}}}
	assets := &assetBuild{}
	assets.manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.Capability(worldterrain.Feature)}

	return root, definition, compiled, assets, encoded
}

func TestTerrainImportPreservesBytesAndCapability(t *testing.T) {
	t.Parallel()
	root, definition, compiled, assets, encoded := terrainFixture(t)

	entries, features, err := compileWorldTerrain(root, definition, &compiled, assets)
	if err != nil || len(entries) != 1 || entries[0].Name != worldterrain.EntryName || entries[0].Kind != level.EntryData ||
		!bytes.Equal(entries[0].Data, encoded) ||
		!slices.Equal(features, []string{worldterrain.Feature}) {
		t.Fatalf("terrain import: %v, %v", features, err)
	}

	metadataBytes, err := loadMetadata(root, definition, nil)
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := level.DecodeMetadata(metadataBytes)
	if err != nil || metadata[worldterrain.MetadataKey].String != worldterrain.Schema {
		t.Fatalf("terrain marker: %v %v", metadata, err)
	}

	assets.manifest.Assets.Capabilities.Runtime = nil
	if _, _, err := compileWorldTerrain(root, definition, &compiled, assets); !errors.Is(err, ErrManifest) {
		t.Fatalf("capability gate: %v", err)
	}
}

func TestTerrainImportRejectsUnsafeAndInconsistentSources(t *testing.T) {
	t.Parallel()

	root, definition, compiled, assets, _ := terrainFixture(t)
	for _, source := range []string{"../terrain.bin", filepath.Join(root, "terrain.bin"), "missing.bin"} {
		definition.Terrain.Source = source
		if _, _, err := compileWorldTerrain(root, definition, &compiled, assets); err == nil {
			t.Fatalf("accepted source %q", source)
		}
	}

	definition.Terrain.Source = "terrain.bin"

	outdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outdir, "outside.bin"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(filepath.Join(outdir, "outside.bin"), filepath.Join(root, "escaped.bin")); err != nil {
		t.Fatal(err)
	}

	definition.Terrain.Source = "escaped.bin"
	if _, _, err := compileWorldTerrain(root, definition, &compiled, assets); !errors.Is(err, ErrSource) {
		t.Fatalf("symlink escape: %v", err)
	}

	definition.Terrain.Source = "terrain.bin"

	compiled.Sectors[0].Sky = false
	if _, _, err := compileWorldTerrain(root, definition, &compiled, assets); err == nil {
		t.Fatal("accepted non-exterior floor")
	}

	compiled.Sectors[0].Sky = true

	definition.Chunks = &chunkBuildSettings{Enabled: true}
	if _, _, err := compileWorldTerrain(root, definition, &compiled, assets); !errors.Is(err, ErrManifest) {
		t.Fatalf("chunk combination: %v", err)
	}

	definition.Chunks = nil

	definition.World.Source = ""
	if err := validateTerrainSettings(definition, assets); !errors.Is(err, ErrManifest) {
		t.Fatalf("missing world: %v", err)
	}

	if _, err := loadData(root, []dataEntry{{Name: worldterrain.EntryName, Source: "terrain.bin"}}); !errors.Is(err, ErrSource) {
		t.Fatalf("reserved entry: %v", err)
	}

	definition.World.Source = "world.yaml"

	if err := os.WriteFile(filepath.Join(root, "terrain.bin"), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := compileWorldTerrain(root, definition, &compiled, assets); err == nil {
		t.Fatal("accepted corrupt terrain")
	}
}

func TestTerrainSampleCoveAndExteriorOwnership(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "samples", "terrain", "levels", "ground")

	expanded, err := worldsource.Load(root, "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := worldgeometry.Compile(
		expanded,
		map[string]uint32{
			"ocean":                      1,
			"ocean-boundary":             2,
			"seabed":                     3,
			"bunker-concrete":            4,
			"pier-wood":                  5,
			"coastal-rock":               6,
			"lighthouse-white":           7,
			"lighthouse-red":             8,
			"lighthouse-metal":           9,
			"lighthouse-lamp":            10,
			"coastal-palm":               11,
			"coastal-palm-normal":        12,
			"coastal-broadleaf":          13,
			"coastal-broadleaf-normal":   14,
			"coastal-grass":              15,
			"coastal-grass-normal":       16,
			"coastal-tall-grass":         17,
			"coastal-tall-grass-normal":  18,
			"coastal-red-fruit":          19,
			"coastal-red-fruit-normal":   20,
			"coastal-seagrass":           21,
			"coastal-seagrass-normal":    22,
			"coastal-ribbon-kelp":        23,
			"coastal-ribbon-kelp-normal": 24,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(compiled.Sectors) != 36 {
		t.Fatal("compact cove must use its connected six-by-six outdoor authoring grid")
	}

	for index, sector := range compiled.Sectors {
		if !sector.Sky || sector.SourceRoom != fmt.Sprintf("cove-%d-%d", index%6, index/6) ||
			sector.Floor.C != -32 || sector.Ceiling.C != 128 {
			t.Fatal("cove grid must preserve source order, sky and open outdoor planes")
		}
	}

	assertTerrainSampleOcean(t, &compiled)

	encoded, err := os.ReadFile(filepath.Join(root, "terrain.bin"))
	if err != nil {
		t.Fatal(err)
	}

	terrain, err := worldterrain.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if err := worldterrain.ValidateForWorld(&terrain, &compiled); err != nil {
		t.Fatal(err)
	}

	if terrain.Origin != [2]float32{-64, -64} || terrain.CellSize != 1 || terrain.Columns != 129 || terrain.Rows != 129 ||
		!slices.Equal(
			terrain.ExteriorSectors,
			[]uint16{7, 8, 9, 10, 13, 14, 15, 16, 19, 20, 21, 22, 25, 26, 27, 28},
		) || len(terrain.Patches) != 16 {
		t.Fatal("cove must use the compact one-meter terrain lattice and its sixteen central exterior owners")
	}

	hollow := false

	for index, hole := range terrain.Holes {
		if hole == 0 {
			continue
		}

		hollow = true

		x := terrain.Origin[0] + (float32(index%128)+.5)*terrain.CellSize

		y := terrain.Origin[1] + (float32(index/128)+.5)*terrain.CellSize
		if !terrainSampleSolidCovers(compiled.StaticSolids.Items, "bunker-floor", float64(x), float64(y)) ||
			!terrainSampleSolidCovers(compiled.StaticSolids.Items, "bunker-roof", float64(x), float64(y)) {
			t.Fatal("terrain opening must be covered by the bunker shell")
		}
	}

	assertTerrainSamplePierApproach(t, &terrain, &compiled)

	if !hollow {
		t.Fatal("bunker interior must retain its covered terrain opening")
	}

	for _, patch := range terrain.Patches {
		if patch.Columns != 32 || patch.Rows != 32 {
			t.Fatal("cove patch dimensions changed")
		}
	}
}

func assertTerrainSampleOcean(t *testing.T, compiled *world.Document) {
	t.Helper()

	if compiled.Fluids == nil || len(compiled.Fluids.Items) != 1 || compiled.Fluids.Items[0].ID != "sea" ||
		!slices.Equal(compiled.Fluids.Items[0].Solids, []string{"cove-ocean"}) || compiled.StaticSolids == nil ||
		len(compiled.StaticSolids.Items) < 3 {
		t.Fatal("cove must retain one ocean volume and its distant seabed")
	}

	for _, solid := range compiled.StaticSolids.Items {
		if solid.ID == "cove-ocean" || solid.ID == "cove-seabed" {
			assertTerrainSampleOceanSolid(t, solid)
		}
	}
}

func assertTerrainSampleOceanSolid(t *testing.T, solid world.Solid) {
	t.Helper()

	switch solid.ID {
	case "cove-ocean":
		if solid.Collision || solid.Top.C != 0 || solid.Bottom.C != -32 || solid.TopMaterial != 1 {
			t.Fatal("ocean must retain its fixed, non-colliding sea level")
		}
	case "cove-seabed":
		if !solid.Collision || solid.Top.C != -30 || solid.Bottom.C != -32 || solid.TopMaterial != 3 {
			t.Fatal("distant ocean requires an opaque colliding sand seabed")
		}
	default:
		t.Fatal("unexpected solid in terrain-only cove")
	}

	expected := []world.Vec2{{X: -1024, Y: -1024}, {X: 1024, Y: -1024}, {X: 1024, Y: 1024}, {X: -1024, Y: 1024}}
	if !slices.Equal(solid.Footprint, expected) {
		t.Fatal("ocean backdrop must cover the complete outdoor grid")
	}
}

// Check physical coverage, rather than coupling the test to detail/plank counts.
func terrainSampleSolidCovers(solids []world.Solid, id string, worldX, worldY float64) bool {
	for _, solid := range solids {
		if solid.ID != id {
			continue
		}

		if !solid.Collision {
			return false
		}

		for i, a := range solid.Footprint {
			b := solid.Footprint[(i+1)%len(solid.Footprint)]
			if (b.X-a.X)*(worldY-a.Y)-(b.Y-a.Y)*(worldX-a.X) < 0 {
				return false
			}
		}

		return true
	}

	return false
}
