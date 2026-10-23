package packages

import (
	"bytes"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

// Exercise the shipped kit through an archive-only consumer, including every
// material, static collision shape and runtime renderer selected by its example.
func TestFPSKitArchive(t *testing.T) {
	t.Parallel()

	kit := fpsKitSource(t)
	archive := fpsKitArchive(t)
	game := t.TempDir()
	write(t, filepath.Join(game, "karty.toml"), []byte("[project]\nname = 'fps-kit-check'\n"))

	if _, err := Import(t.Context(), game, archive, false); err != nil {
		t.Fatal(err)
	}

	set := restore(t, game)
	if set.Items["fps-kit"].Manifest.GoModule != "" {
		t.Fatal("asset pack unexpectedly requires a Go module")
	}

	prefabs, _, err := collectWorldExports(t.Context(), set)
	if err != nil {
		t.Fatal(err)
	}

	if _, found := prefabs["fps-kit.ramp"]; found {
		t.Fatal("slopes belong in authored level geometry, not the package prefab library")
	}

	merged, textures, err := ResolveWorld(t.Context(), set, read(t, filepath.Join(kit, "examples/room/world.yaml")), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(textures) != 49 {
		t.Fatalf("example must exercise all 49 materials, got %d", len(textures))
	}

	materials := make(map[string]uint32, len(textures))
	for index, texture := range textures {
		img, err := png.Decode(bytes.NewReader(texture.Data))
		if err != nil {
			t.Fatal(err)
		}

		if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
			t.Fatalf("%s is not 256x256", texture.Name)
		}

		opaque, ok := img.(interface{ Opaque() bool })
		if !ok || !opaque.Opaque() {
			t.Fatalf("%s has transparency", texture.Name)
		}

		assertFPSKitMaterialPolicy(t, texture)

		materials[texture.Name] = uint32(index + 1)
	}

	expanded, err := source.Decode(merged)
	if err != nil {
		t.Fatal(err)
	}

	assertFPSKitGallerySpawnClearance(t, expanded)

	compiled, err := geometry.Compile(expanded, materials)
	if err != nil {
		t.Fatal(err)
	}

	if compiled.StaticSolids == nil || len(compiled.StaticSolids.Items) != 94 {
		t.Fatal("example lost its static crates, bumpers, stack, cover or step")
	}

	for _, solid := range compiled.StaticSolids.Items {
		if solid.Collision == strings.HasPrefix(solid.ID, "sample-") {
			t.Fatalf("static solid %s has incorrect collision policy", solid.ID)
		}
	}

	assertFPSKitBumpers(t, compiled.StaticSolids, materials)
	assertFPSKitDynamic(t, compiled.SolidRenderers)

	encoded, err := world.Encode(compiled)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := world.Decode(encoded); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(game, "go.mod")); !os.IsNotExist(err) {
		t.Fatal("resource-only import changed the consumer Go module")
	}
}

func assertFPSKitMaterialPolicy(t *testing.T, texture WorldTexture) {
	t.Helper()

	if texture.Materialize == nil {
		t.Fatalf("material %s lost its explicit generation policy", texture.Name)
	}

	if strings.HasPrefix(texture.Name, "fps-kit.bumper-") {
		if texture.Materialize.Preset != "metal" {
			t.Fatalf("bumper material %s lost its automatic metal preset", texture.Name)
		}

		return
	}

	if texture.Materialize.MetallicScale == nil || *texture.Materialize.MetallicScale != 0 {
		t.Fatalf("nonmetal material %s must not infer metalness from its color", texture.Name)
	}
}

func assertFPSKitDynamic(t *testing.T, renderers *world.SolidRenderers) {
	t.Helper()

	if renderers == nil {
		t.Fatal("example lost its dynamic renderer prefabs")
	}

	for _, shape := range []struct {
		name    string
		x, y, z float64
	}{
		{"small", .25, .25, .25}, {"medium", .5, .5, .5},
		{"large", 1, 1, 1}, {"long", 1, .5, .5}, {"tall", .5, .5, 1},
	} {
		found := false

		for _, prefab := range renderers.Prefabs {
			if prefab.ID != "crate-"+shape.name+"-dynamic" {
				continue
			}

			found = true

			assertFPSCrateBounds(t, prefab, shape.x, shape.y, shape.z)
		}

		if !found {
			t.Fatalf("missing dynamic %s renderer", shape.name)
		}
	}
}

func assertFPSCrateBounds(t *testing.T, prefab world.SolidPrefab, halfX, halfY, halfZ float64) {
	t.Helper()

	if len(prefab.Solids) != 7 {
		t.Fatal("crate must have one inset panel, two caps and four corner posts")
	}

	minX, minY, minZ := math.Inf(1), math.Inf(1), math.Inf(1)
	maxX, maxY, maxZ := math.Inf(-1), math.Inf(-1), math.Inf(-1)
	posts := 0

	for _, solid := range prefab.Solids {
		if solid.Collision {
			t.Fatal("dynamic renderer must not bake static collision")
		}

		if strings.Contains(solid.ID, "post-") {
			posts++
		}

		minZ, maxZ = math.Min(minZ, solid.Bottom.C), math.Max(maxZ, solid.Top.C)
		for _, point := range solid.Footprint {
			minX, minY = math.Min(minX, point.X), math.Min(minY, point.Y)
			maxX, maxY = math.Max(maxX, point.X), math.Max(maxY, point.Y)
		}
	}

	if posts != 4 || minX != -halfX || maxX != halfX || minY != -halfY || maxY != halfY || minZ != -halfZ || maxZ != halfZ {
		t.Fatalf("crate %s frame does not match its centered physics box", prefab.ID)
	}
}

func assertFPSKitBumpers(t *testing.T, solids *world.StaticSolids, materials map[string]uint32) {
	t.Helper()

	red, capID := materials["fps-kit.bumper-red-metal"], materials["fps-kit.bumper-metal-cap"]
	if red == 0 || capID == 0 || red == capID {
		t.Fatal("bumper side and cap materials must be selected separately")
	}

	heights := map[float64]bool{0.75: false, 1.25: false, 2: false}
	count := 0

	for _, solid := range solids.Items {
		if solid.SideMaterial != red {
			continue
		}

		count++

		_, expected := heights[solid.Top.C]
		if !expected || heights[solid.Top.C] || !solid.Collision || len(solid.Footprint) != 8 ||
			solid.Bottom.A != 0 || solid.Bottom.B != 0 || solid.Bottom.C != 0 ||
			solid.Top.A != 0 || solid.Top.B != 0 || solid.TopMaterial != capID || solid.BottomMaterial != capID {
			t.Fatalf("bumper must be one colliding octagonal solid with metal caps: %+v", solid)
		}

		heights[solid.Top.C] = true

		minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, point := range solid.Footprint {
			minX, minY = math.Min(minX, point.X), math.Min(minY, point.Y)
			maxX, maxY = math.Max(maxX, point.X), math.Max(maxY, point.Y)
		}

		if math.Abs(maxX-minX-1.7) > 1e-9 || math.Abs(maxY-minY-1.7) > 1e-9 {
			t.Fatalf("bumper %s must retain its 1.7-meter diameter", solid.ID)
		}
	}

	if count != 3 {
		t.Fatalf("gallery must select one solid per bumper height, got %d", count)
	}
}
