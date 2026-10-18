package geometry_test

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestCompileVegetationResolvesRoomOwnershipAndCeiling(t *testing.T) {
	t.Parallel()

	expanded := lightingExpanded(0)
	expanded.Vegetation = &worldsource.Vegetation{
		Version: 1,
		Presets: []worldsource.VegetationPreset{
			{
				Name:          "grass",
				Texture:       "grass",
				Width:         worldvegetation.Range{Min: .2, Max: .4},
				Height:        worldvegetation.Range{Min: .2, Max: .5},
				WindAmplitude: .08,
				WindFrequency: .7,
				DrawDistance:  32,
			},
		},
	}
	expanded.Rooms[0].VegetationLayers = []worldsource.VegetationLayer{
		{Preset: "grass", Density: 4, Seed: 42, Surface: "floor"},
		{Preset: "grass", Density: 0, Seed: 43, Surface: "ceiling"},
	}

	compiled, err := geometry.Compile(expanded, map[string]uint32{"wall": 1})
	if err != nil {
		t.Fatal(err)
	}

	document, err := geometry.CompileVegetation(expanded, &compiled, map[string]uint32{"grass": 2})
	if err != nil {
		t.Fatal(err)
	}

	if len(document.Layers) != 2 || document.Layers[0].Seed != 42 || document.Presets[0].Texture != 2 {
		t.Fatalf("lost layers: %+v", document)
	}

	if _, err := geometry.CompileVegetation(expanded, &compiled, map[string]uint32{}); err == nil {
		t.Fatal("undeclared texture accepted")
	}

	compiled.Sectors[0].Sky = true
	if _, err := geometry.CompileVegetation(expanded, &compiled, map[string]uint32{"grass": 2}); err == nil {
		t.Fatal("sky ceiling accepted")
	}
}

func TestEnvironmentVegetationLivesOnlyOnGroundWithIndependentDensities(t *testing.T) {
	t.Parallel()

	directory := filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight")

	expanded, err := source.Load(directory, "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	textures := environmentMaterials()
	textures["grass-tuft"] = uint32(len(textures) + 1)
	textures["small-bush"] = uint32(len(textures) + 1)
	textures["grass-tuft-normal"] = uint32(len(textures) + 1)
	textures["small-bush-normal"] = uint32(len(textures) + 1)

	compiled, err := geometry.Compile(expanded, textures)
	if err != nil {
		t.Fatal(err)
	}

	document, err := geometry.CompileVegetation(expanded, &compiled, textures)
	if err != nil {
		t.Fatal(err)
	}

	if len(document.Presets) != 3 || len(document.Layers) != 70 {
		t.Fatalf("unexpected field vegetation: %+v", document)
	}

	if document.Version != worldvegetation.TreeVersion ||
		document.Presets[0].Geometry != worldvegetation.GeometryCrossedCards ||
		document.Presets[1].Geometry != worldvegetation.GeometryShrub ||
		document.Presets[2].Geometry != worldvegetation.GeometryTree {
		t.Fatalf("field lost its independent grass-card and shrub-mesh geometry: %+v", document.Presets)
	}

	if document.Presets[0].NormalTexture != textures["grass-tuft-normal"] ||
		document.Presets[1].NormalTexture != textures["small-bush-normal"] ||
		document.Presets[2].NormalTexture != textures["small-bush-normal"] {
		t.Fatalf("field normal-map resource references changed: %+v", document.Presets)
	}

	densities := map[string]map[float64]bool{}

	for _, layer := range document.Layers {
		if !strings.HasPrefix(compiled.Sectors[layer.Sector].SourceRoom, "test-ground-") || layer.Surface != "floor" {
			t.Fatalf("vegetation outside field: %+v", layer)
		}

		if densities[layer.Preset] == nil {
			densities[layer.Preset] = map[float64]bool{}
		}

		densities[layer.Preset][layer.Density] = true
	}

	for _, name := range []string{"field-grass", "field-bush"} {
		if len(densities[name]) < 4 {
			t.Fatalf("authored independent densities lost: %+v", densities)
		}
	}
}

func TestEnvironmentTrailStaysClearAndShouldersThinTheVegetation(t *testing.T) {
	t.Parallel()

	directory := filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight")

	expanded, err := source.Load(directory, "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	rooms := map[string]source.Room{}
	for _, room := range expanded.Rooms {
		rooms[room.ID] = room
	}

	for row := range 4 {
		trail := rooms[fmt.Sprintf("test-ground-4-%d", row)]
		if trail.FloorMaterial != "path" || len(trail.VegetationLayers) != 0 ||
			trail.FloorUV == nil || trail.FloorUV.Scale == nil || trail.FloorUV.Scale.X != 3 || trail.FloorUV.Scale.Y != 3 {
			t.Fatalf("trail lost its clear dirt floor: %+v", trail)
		}

		for _, column := range []int{3, 5} {
			shoulder := rooms[fmt.Sprintf("test-ground-%d-%d", column, row)]

			outerColumn := column - 1
			if column == 5 {
				outerColumn = column + 1
			}

			outer := rooms[fmt.Sprintf("test-ground-%d-%d", outerColumn, row)]
			assertEnvironmentShoulderDensity(t, shoulder, outer)
		}
	}
}

func assertEnvironmentShoulderDensity(t *testing.T, shoulder, outer source.Room) {
	t.Helper()

	if shoulder.FloorMaterial != "ground" || shoulder.FloorUV == nil || shoulder.FloorUV.Scale == nil || shoulder.FloorUV.Scale.X != 12 {
		t.Fatalf("trail shoulder lost its ground material: %+v", shoulder)
	}

	for _, preset := range []string{"field-grass", "field-bush"} {
		var shoulderDensity, outerDensity float64

		for _, layer := range shoulder.VegetationLayers {
			if layer.Preset == preset {
				shoulderDensity = layer.Density
			}
		}

		for _, layer := range outer.VegetationLayers {
			if layer.Preset == preset {
				outerDensity = layer.Density
			}
		}

		if shoulderDensity <= 0 || shoulderDensity >= outerDensity*.3 {
			t.Fatalf("trail shoulder is not thinner than adjoining meadow: %s %g >= %g", preset, shoulderDensity, outerDensity)
		}
	}
}

func TestEnvironmentTreesHaveTwoSeededCandidatesInEachSelectedSector(t *testing.T) {
	t.Parallel()

	directory := filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight")

	expanded, err := source.Load(directory, "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	seeds := map[uint32]bool{}

	for _, room := range expanded.Rooms {
		for _, layer := range room.VegetationLayers {
			if layer.Preset != "field-tree" {
				continue
			}

			assertEnvironmentTreeCandidate(t, room, layer)

			if seeds[layer.Seed] {
				t.Fatal("tree cohorts must have distinct seeds")
			}

			seeds[layer.Seed] = true
		}
	}

	if len(seeds) != 6 {
		t.Fatalf("expected six tree cohorts beside the trail, got %d", len(seeds))
	}
}

func assertEnvironmentTreeCandidate(t *testing.T, room source.Room, layer worldsource.VegetationLayer) {
	t.Helper()

	var column, row int
	if _, err := fmt.Sscanf(room.ID, "test-ground-%d-%d", &column, &row); err != nil ||
		(column != 2 && column != 6) || row < 1 || row > 3 || layer.Surface != "floor" {
		t.Fatalf("tree cohort outside the authored trail margins: %s %+v", room.ID, layer)
	}

	var area float64

	for index, edge := range room.Boundary {
		next := room.Boundary[(index+1)%len(room.Boundary)]
		area += edge.Start.X*next.Start.Y - edge.Start.Y*next.Start.X
	}

	mean := float64(float32(math.Abs(area)/2)) * float64(float32(layer.Density))
	if math.Abs(mean-2) > 1e-6 || math.Ceil(mean) != 2 {
		t.Fatalf("tree sector must generate two candidates: %s mean=%g", room.ID, mean)
	}
}

func TestEnvironmentDirtTrailTextureContrastsWithGrassSoil(t *testing.T) {
	t.Parallel()

	dirt := environmentVegetationTexture(t, "path")
	ground := environmentVegetationTexture(t, "ground")
	dirtColor := environmentMeanRGB(dirt)

	groundColor := environmentMeanRGB(ground)
	if dirtColor[0]-dirtColor[1] < 15 || dirtColor[0]/dirtColor[1] < groundColor[0]/groundColor[1]+.15 {
		t.Fatalf("dirt trail must remain visibly earth-brown against the grass soil: dirt=%v grass=%v", dirtColor, groundColor)
	}
}

func environmentMeanRGB(pixels image.Image) [3]float64 {
	var sums [3]float64

	for row := pixels.Bounds().Min.Y; row < pixels.Bounds().Max.Y; row++ {
		for column := pixels.Bounds().Min.X; column < pixels.Bounds().Max.X; column++ {
			red, green, blue, _ := pixels.At(column, row).RGBA()
			sums[0] += float64(red) / 257
			sums[1] += float64(green) / 257
			sums[2] += float64(blue) / 257
		}
	}

	for index := range sums {
		sums[index] /= float64(pixels.Bounds().Dx() * pixels.Bounds().Dy())
	}

	return sums
}

func TestEnvironmentGrassTextureIsCutout(t *testing.T) {
	t.Parallel()

	pixels := environmentVegetationTexture(t, "grass-tuft")
	if pixels.Bounds() != image.Rect(0, 0, 512, 512) {
		t.Fatalf("unexpected grass cutout dimensions: %v", pixels.Bounds())
	}

	opaque, transparent := 0, 0

	for y := pixels.Bounds().Min.Y; y < pixels.Bounds().Max.Y; y++ {
		for x := pixels.Bounds().Min.X; x < pixels.Bounds().Max.X; x++ {
			_, _, _, a := pixels.At(x, y).RGBA()
			if a == 0 {
				transparent++
			} else {
				opaque++
			}
		}
	}

	if opaque == 0 || transparent == 0 {
		t.Fatal("grass texture is not a cutout")
	}
}

func TestEnvironmentShrubTextureCombinesLeafCutoutAndOpaqueBark(t *testing.T) {
	t.Parallel()

	pixels := environmentVegetationTexture(t, "small-bush")
	if pixels.Bounds() != image.Rect(0, 0, 512, 1024) {
		t.Fatalf("unexpected shrub leaf/bark atlas dimensions: %v", pixels.Bounds())
	}

	leafPixels, leafGaps := 0, 0

	for row := pixels.Bounds().Min.Y; row < pixels.Bounds().Max.Y; row++ {
		for column := pixels.Bounds().Min.X; column < pixels.Bounds().Max.X; column++ {
			red, green, _, alpha := pixels.At(column, row).RGBA()
			if row < pixels.Bounds().Dy()/2 {
				if alpha == 0 {
					leafGaps++
				} else {
					leafPixels++
				}

				continue
			}

			if alpha != 0xffff || red <= green {
				t.Fatalf("shrub bark must be opaque brown at %d,%d", column, row)
			}
		}
	}

	if leafPixels == 0 || leafGaps == 0 {
		t.Fatal("shrub leaf spray must contain leaves and transparent gaps")
	}
}

func TestEnvironmentVegetationNormalsHaveReliefAndNeutralBackground(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"grass-tuft", "small-bush"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			checkEnvironmentVegetationNormals(t, name)
		})
	}
}

func checkEnvironmentVegetationNormals(t *testing.T, name string) {
	t.Helper()

	albedo := environmentVegetationTexture(t, name)

	normals := environmentVegetationTexture(t, name+"-normal")
	if normals.Bounds() != albedo.Bounds() {
		t.Fatalf("normal/albedo dimensions differ: %s", name)
	}

	var varied, neutralBackground int

	for row := normals.Bounds().Min.Y; row < normals.Bounds().Max.Y; row++ {
		for column := normals.Bounds().Min.X; column < normals.Bounds().Max.X; column++ {
			red, green, blue, alpha := normals.At(column, row).RGBA()
			if alpha != 0xffff {
				t.Fatalf("normal data must be opaque: %s at %d,%d", name, column, row)
			}

			nx, ny, nz := float64(red)/65535*2-1, float64(green)/65535*2-1, float64(blue)/65535*2-1
			if math.Abs(math.Sqrt(nx*nx+ny*ny+nz*nz)-1) > .015 || nz <= 0 {
				t.Fatalf("invalid tangent normal: %s at %d,%d", name, column, row)
			}

			_, _, _, coverage := albedo.At(column, row).RGBA()
			bark := name == "small-bush" && row >= albedo.Bounds().Dy()/2

			neutral := red == 0x8080 && green == 0x8080 && blue == 0xffff
			if coverage == 0 || bark {
				if !neutral {
					t.Fatalf("background/bark normal must be flat: %s at %d,%d", name, column, row)
				}

				neutralBackground++
			} else if !neutral {
				varied++
			}
		}
	}

	if varied == 0 || neutralBackground == 0 {
		t.Fatalf("normal map lacks relief or flat background: %s", name)
	}
}

func environmentVegetationTexture(t *testing.T, name string) image.Image {
	t.Helper()

	directory := filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight")

	file, err := os.Open(filepath.Join(directory, name+".png"))
	if err != nil {
		t.Fatal(err)
	}

	pixels, err := png.Decode(file)

	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("vegetation texture %s: %v %v", name, err, closeErr)
	}

	return pixels
}

func TestCompilePalmRetainsGeometryNormalsAndCeilingLayer(t *testing.T) {
	t.Parallel()

	expanded := lightingExpanded(0)
	expanded.Vegetation = &worldsource.Vegetation{
		Version: worldvegetation.PalmVersion,
		Presets: []worldsource.VegetationPreset{
			{
				Name:          "palm",
				Geometry:      "palm",
				Texture:       "palm",
				NormalTexture: "palm-normal",
				Width:         worldvegetation.Range{Min: .2, Max: .4},
				Height:        worldvegetation.Range{Min: .2, Max: .5},
				WindAmplitude: .08,
				WindFrequency: .7,
				DrawDistance:  32,
			},
		},
	}
	expanded.Rooms[0].VegetationLayers = []worldsource.VegetationLayer{
		{Preset: "palm", Density: .01, Seed: 73, Surface: "floor"},
		{Preset: "palm", Density: 0, Seed: 74, Surface: "ceiling"},
	}

	compiled, err := geometry.Compile(expanded, map[string]uint32{"wall": 1})
	if err != nil {
		t.Fatal(err)
	}

	document, err := geometry.CompileVegetation(expanded, &compiled, map[string]uint32{"palm": 2, "palm-normal": 3})
	if err != nil {
		t.Fatal(err)
	}

	if document.Version != worldvegetation.PalmVersion || document.Presets[0].Geometry != worldvegetation.GeometryPalm ||
		document.Presets[0].NormalTexture != 3 ||
		len(document.Layers) != 2 ||
		document.Layers[1].Surface != "ceiling" ||
		document.Layers[0].Seed != 73 {
		t.Fatalf("palm sidecar lost authoring: %+v", document)
	}
}
