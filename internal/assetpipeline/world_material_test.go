package assetpipeline

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/sdk"
)

func fixtureMaterialManifest() sdk.Manifest {
	manifest := sdk.Manifest{Version: "0.0.8"}
	manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityWorldMaterialAtlasV1}
	manifest.Tools.Materialize = "2.0.0"
	manifest.Tools.MaterializeRevision = strings.Repeat("b", 40)
	manifest.Artifacts.Materialize = make(map[string]sdk.MaterialToolArtifact)

	for _, platform := range []string{"darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64"} {
		executable := "bin/materialize-cli"
		if platform == "windows-amd64" {
			executable += ".exe"
		}

		manifest.Artifacts.Materialize[platform] = sdk.MaterialToolArtifact{
			URL:    "https://github.com/karty-game/karty-tools/releases/download/materialize-v2.0.0/materialize-2.0.0-" + platform + ".zip",
			SHA256: strings.Repeat("a", 64), Format: "zip", Executable: executable,
		}
	}

	return manifest
}

func fixtureMaterialWorld() world.Document {
	return world.Document{Sectors: []world.Sector{
		{FloorMaterial: 9, CeilingMaterial: 3, Walls: []world.Wall{{Material: 9}, {Material: 0}, {Material: 7}}},
		{FloorMaterial: 7, CeilingMaterial: 3, Walls: []world.Wall{{Material: 0}}},
	}}
}

func fixtureQOI(t *testing.T, width, height int, pixel color.NRGBA) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetNRGBA(x, y, pixel)
		}
	}

	encoded, _, err := qoi.Encode(img, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		t.Fatal(err)
	}

	return encoded
}

func fixtureTextures(t *testing.T) map[uint32][]byte {
	t.Helper()

	return map[uint32][]byte{
		9: fixtureQOI(t, 2, 2, color.NRGBA{R: 200, A: 255}),
		3: fixtureQOI(t, 2, 2, color.NRGBA{G: 150, A: 255}),
		7: fixtureQOI(t, 2, 2, color.NRGBA{B: 100, A: 255}),
	}
}

func writeMaterialPNG(path string, img image.Image) error {
	var encoded bytes.Buffer

	if err := png.Encode(&encoded, img); err != nil {
		return err
	}

	return os.WriteFile(path, encoded.Bytes(), 0o600)
}

func fixtureMaterialMaps(directory string, bounds image.Rectangle) error {
	for _, name := range []string{"normal", "height", "ao"} {
		var img image.Image

		if name == "normal" {
			normal := image.NewRGBA(bounds)
			draw.Draw(normal, bounds, image.NewUniform(color.RGBA{R: 127, G: 220, B: 19, A: 255}), image.Point{}, draw.Src)
			img = normal
		} else {
			// The pinned native encoder emits RGB8 normal and Luma8 height/AO.
			value := uint8(61)
			if name == "ao" {
				value = 83
			}

			gray := image.NewGray(bounds)
			draw.Draw(gray, bounds, image.NewUniform(color.Gray{Y: value}), image.Point{}, draw.Src)
			img = gray
		}

		if err := writeMaterialPNG(filepath.Join(directory, "atlas_"+name+".png"), img); err != nil {
			return err
		}
	}

	return nil
}

func executePackedMaterialFixture(directory string, args []string, _ worldmaterial.Layout) error {
	wantArgs := []string{filepath.Join(directory, "atlas.png"), "--output", directory, "--only", "height,normal,ao",
		"--normal-format", "opengl", "--format", "png", "--preset", "default"}
	if !reflect.DeepEqual(args, wantArgs) {
		return ErrCacheOutput
	}

	encoded, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	input, err := png.Decode(bytes.NewReader(encoded))
	if err != nil {
		return err
	}

	if input.Bounds() != image.Rect(0, 0, 256, 256) {
		return ErrCacheOutput
	}

	want := []color.NRGBA{{R: 200, A: 255}, {G: 150, A: 255}, {R: 192, G: 192, B: 192, A: 255}, {B: 100, A: 255}}

	pixel, ok := color.NRGBAModel.Convert(input.At(0, 0)).(color.NRGBA)
	if !ok {
		return ErrCacheOutput
	}

	if !slices.Contains(want, pixel) {
		return ErrCacheOutput
	}

	for _, point := range []image.Point{{0, 0}, {255, 255}, {128, 128}} {
		if color.NRGBAModel.Convert(input.At(point.X, point.Y)) != pixel {
			return ErrCacheOutput
		}
	}

	return fixtureMaterialMaps(directory, input.Bounds())
}

func TestWorldMaterialPerAssetAndWarmAtlasCache(t *testing.T) {
	t.Parallel()

	document, textures, manifest := fixtureMaterialWorld(), fixtureTextures(t), fixtureMaterialManifest()

	if got := WorldMaterialIDs(document); !reflect.DeepEqual(got, []uint32{9, 3, 0, 7}) {
		t.Fatalf("surface order: %v", got)
	}

	layout, err := worldmaterial.NewLayout(WorldMaterialIDs(document))
	if err != nil {
		t.Fatal(err)
	}

	layout, err = worldmaterial.NewMipLayout(layout)
	if err != nil {
		t.Fatal(err)
	}

	calls := 0
	execute := func(_ context.Context, _ sdk.Manifest, directory string, args []string) error {
		calls++

		return executePackedMaterialFixture(directory, args, layout)
	}
	root := t.TempDir()

	first, cold, err := processWorldMaterials(context.Background(), root, manifest, document, textures, execute)
	if err != nil {
		t.Fatal(err)
	}

	second, warm, err := processWorldMaterials(context.Background(), root, manifest, document, textures, execute)
	if err != nil {
		t.Fatal(err)
	}

	if calls != 4 || cold.CacheHit || !warm.CacheHit || !reflect.DeepEqual(first, second) {
		t.Fatalf("calls=%d cold=%v warm=%v", calls, cold.CacheHit, warm.CacheHit)
	}

	_, data, err := qoi.Decode(first.Data)
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range layout.Materials {
		if got := data.NRGBAAt(r.X-r.Gutter, r.Y-r.Gutter); got != (color.NRGBA{R: 128, G: 220, B: 61, A: 83}) {
			t.Fatalf("merged straight channels/gutter = %v", got)
		}
	}
	// Layout is the canonical JSON mapping the same IDs to fixed pair entries.
	encoded, err := worldmaterial.EncodeLayout(first.Layout)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := worldmaterial.DecodeLayout(encoded)
	if err != nil || !reflect.DeepEqual(decoded, layout) {
		t.Fatalf("layout round trip: %v", err)
	}
	// Ordinary cache corruption must rebuild, not publish stale map bytes.
	if err := os.WriteFile(cold.PayloadPath, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, rebuilt, err := processWorldMaterials(context.Background(), root, manifest, document, textures, execute)
	if err != nil || rebuilt.CacheHit || calls != 4 {
		t.Fatalf("corrupt cache: calls=%d hit=%v err=%v", calls, rebuilt.CacheHit, err)
	}
	// Identical packed pixels do not hide a changed source QOI from the cache.
	textures[9] = fixtureQOI(t, 3, 3, color.NRGBA{R: 200, A: 255})

	_, changed, err := processWorldMaterials(context.Background(), root, manifest, document, textures, execute)
	if err != nil || changed.CacheHit || changed.CacheKey == rebuilt.CacheKey || calls != 5 {
		t.Fatalf("source invalidation: calls=%d err=%v", calls, err)
	}
}

func TestWorldMaterialPerAssetOptionsAndLayoutIndependence(t *testing.T) {
	t.Parallel()
	manifest, textures, document := fixtureMaterialManifest(), fixtureTextures(t), fixtureMaterialWorld()
	root := t.TempDir()
	calls := 0
	execute := func(_ context.Context, _ sdk.Manifest, directory string, args []string) error {
		calls++

		if slices.Contains(args, "--seamless") || slices.Contains(args, "--no-seamless") {
			t.Fatal("seamless detection was overridden")
		}

		encoded, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}

		input, err := png.Decode(bytes.NewReader(encoded))
		if err != nil {
			return err
		}

		if input.Bounds() != image.Rect(0, 0, 256, 256) {
			return ErrCacheOutput
		}

		pixel, ok := color.NRGBAModel.Convert(input.At(0, 0)).(color.NRGBA)
		if !ok {
			return ErrCacheOutput
		}

		if pixel.R == 200 {
			want := []string{
				"--preset",
				"metal",
				"--normal-strength",
				"0",
				"--height-contrast",
				"1.5",
				"--metallic-scale",
				"1",
				"--ao-depth-scale",
				"2",
			}
			if !reflect.DeepEqual(args[len(args)-len(want):], want) {
				t.Fatalf("asset options: %v", args)
			}
		}

		return fixtureMaterialMaps(directory, input.Bounds())
	}
	zero, height, metal, ao := 0.0, 1.5, 1.0, 2.0
	options := map[uint32]MaterializeOptions{
		9: {Preset: "metal", NormalStrength: &zero, HeightContrast: &height, MetallicScale: &metal, AODepthScale: &ao},
	}

	first, _, err := processWorldMaterials(t.Context(), root, manifest, document, textures, execute, options)
	if err != nil || calls != 4 {
		t.Fatalf("cold perasset build: calls=%d err=%v", calls, err)
	}
	// Changing one preset regenerates just that asset.
	options[3] = MaterializeOptions{Preset: "brick"}

	_, changed, err := processWorldMaterials(t.Context(), root, manifest, document, textures, execute, options)
	if err != nil || calls != 5 || changed.CacheHit {
		t.Fatalf("preset change: calls=%d err=%v", calls, err)
	}
	// Moving materials within the atlas must only rebuild assembly.
	document.Sectors[0].FloorMaterial, document.Sectors[0].CeilingMaterial = 3, 9

	reordered, moved, err := processWorldMaterials(t.Context(), root, manifest, document, textures, execute, options)
	if err != nil || calls != 5 || moved.CacheHit || reflect.DeepEqual(first.Layout, reordered.Layout) {
		t.Fatalf("layout change regenerated source maps: calls=%d err=%v", calls, err)
	}
}

func TestWorldMaterialRecipeInvalidation(t *testing.T) {
	t.Parallel()

	manifest, textures := fixtureMaterialManifest(), fixtureTextures(t)

	layout, err := worldmaterial.NewLayout([]uint32{9, 3, 0, 7})
	if err != nil {
		t.Fatal(err)
	}

	args := materializeArguments("atlas.png", ".")

	base, err := worldMaterialRecipe(manifest, layout, textures, "linux-amd64", strings.Repeat("a", 64), args)
	if err != nil {
		t.Fatal(err)
	}

	changedSource := fixtureTextures(t)
	changedSource[9] = fixtureQOI(t, 3, 3, color.NRGBA{R: 200, A: 255}) // Same completed atlas, different source.
	changedLayout, _ := worldmaterial.NewLayout([]uint32{3, 9, 0, 7})
	changedPin := fixtureMaterialManifest()
	changedPin.Tools.MaterializeRevision = strings.Repeat("c", 40)
	changedVersion := fixtureMaterialManifest()
	changedVersion.Tools.Materialize = "2.0.1"

	changedArgs := append([]string(nil), args...)
	changedArgs[len(changedArgs)-1] = "brick"

	for _, test := range []struct {
		name               string
		manifest           sdk.Manifest
		layout             worldmaterial.Layout
		textures           map[uint32][]byte
		platform, checksum string
		args               []string
	}{
		{"source", manifest, layout, changedSource, "linux-amd64", strings.Repeat("a", 64), args},
		{"layout", manifest, changedLayout, textures, "linux-amd64", strings.Repeat("a", 64), args},
		{"revision", changedPin, layout, textures, "linux-amd64", strings.Repeat("a", 64), args},
		{"version", changedVersion, layout, textures, "linux-amd64", strings.Repeat("a", 64), args},
		{"archive", manifest, layout, textures, "linux-amd64", strings.Repeat("d", 64), args},
		{"platform", manifest, layout, textures, "darwin-arm64", strings.Repeat("a", 64), args},
		{"options", manifest, layout, textures, "linux-amd64", strings.Repeat("a", 64), changedArgs},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := worldMaterialRecipe(test.manifest, test.layout, test.textures, test.platform, test.checksum, test.args)
			if err != nil || got.Digest() == base.Digest() {
				t.Fatalf("recipe not invalidated: %v", err)
			}
		})
	}
}

func TestWorldMaterialFailuresNeverPublish(t *testing.T) {
	t.Parallel()

	sentinel := os.ErrPermission

	for _, failure := range []string{"executor", "missing", "dimensions", "truncated", "16-bit", "transparent", "symlink"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			calls := 0

			execute := func(_ context.Context, _ sdk.Manifest, directory string, args []string) error {
				calls++

				return executeFailedMaterialFixture(failure, directory, args)
			}

			for range 2 {
				_, _, err := processWorldMaterials(
					t.Context(), root, fixtureMaterialManifest(), fixtureMaterialWorld(), fixtureTextures(t), execute)
				if err == nil {
					t.Fatal("failed material output accepted")
				}

				if failure == "executor" && !errors.Is(err, sentinel) {
					t.Fatal("executor error lost")
				}
			}

			if calls != 2 {
				t.Fatal("failed processor published a cache entry")
			}
		})
	}
}

func executeFailedMaterialFixture(failure, directory string, args []string) error {
	if failure == "executor" {
		return os.ErrPermission
	}

	if failure == "missing" {
		return nil
	}

	input, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	config, err := png.DecodeConfig(bytes.NewReader(input))
	if err != nil {
		return err
	}

	bounds := image.Rect(0, 0, config.Width, config.Height)
	if err := fixtureMaterialMaps(directory, bounds); err != nil {
		return err
	}

	path := filepath.Join(directory, "atlas_normal.png")

	switch failure {
	case "dimensions":
		return writeMaterialPNG(path, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	case "truncated":
		return os.WriteFile(path, []byte("invalid PNG"), 0o600)
	case "16-bit":
		return writeMaterialPNG(path, image.NewNRGBA64(bounds))
	case "transparent":
		return writeMaterialPNG(path, image.NewNRGBA(bounds))
	case "symlink":
		if err := os.Remove(path); err != nil {
			return err
		}

		return os.Symlink(args[0], path)
	default:
		return ErrCacheOutput
	}
}

func TestWorldMaterialPreflight(t *testing.T) {
	t.Parallel()

	layout, _ := worldmaterial.NewLayout([]uint32{1})
	if _, err := packWorldAlbedo(t.Context(), layout, nil); err == nil {
		t.Fatal("missing texture accepted")
	}

	if _, err := packWorldAlbedo(t.Context(), layout, map[uint32][]byte{1: fixtureQOI(t, 1, 1, color.NRGBA{A: 254})}); err == nil {
		t.Fatal("transparent albedo accepted")
	}

	invalid := layout
	invalid.Width = -1

	if _, err := packWorldAlbedo(t.Context(), invalid, nil); err == nil {
		t.Fatal("unbounded layout accepted")
	}
	// Header-only giant textures must fail aggregate preflight before decoding.
	large := fixtureQOI(t, 1, 1, color.NRGBA{A: 255})
	binary.BigEndian.PutUint32(large[4:8], 4096)
	binary.BigEndian.PutUint32(large[8:12], 4096)
	textures := map[uint32][]byte{1: large, 2: large, 3: large, 4: large}

	if _, err := packWorldAlbedo(t.Context(), layout, textures); !errors.Is(err, ErrAssetResourceLimits) {
		t.Fatalf("aggregate budget: %v", err)
	}

	execute := func(context.Context, sdk.Manifest, string, []string) error {
		t.Fatal("executor called before preflight")

		return nil
	}
	manifest := fixtureMaterialManifest()
	manifest.Assets.Capabilities.Runtime = nil

	if _, _, err := processWorldMaterials(
		t.Context(), t.TempDir(), manifest, fixtureMaterialWorld(), fixtureTextures(t), execute); err == nil {
		t.Fatal("capability absent")
	}

	manifest = fixtureMaterialManifest()
	delete(manifest.Artifacts.Materialize, runtime.GOOS+"-"+runtime.GOARCH)

	if _, _, err := processWorldMaterials(
		t.Context(), t.TempDir(), manifest, fixtureMaterialWorld(), fixtureTextures(t), execute); err == nil {
		t.Fatal("incomplete pin accepted")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := processWorldMaterials(ctx, t.TempDir(), fixtureMaterialManifest(), fixtureMaterialWorld(), fixtureTextures(t), execute)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestWorldMaterialConcurrentColdBuilds(t *testing.T) {
	t.Parallel()

	root, manifest, document, textures := t.TempDir(), fixtureMaterialManifest(), fixtureMaterialWorld(), fixtureTextures(t)

	var calls atomic.Int32

	execute := func(_ context.Context, _ sdk.Manifest, directory string, args []string) error {
		calls.Add(1)

		encoded, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}

		config, err := png.DecodeConfig(bytes.NewReader(encoded))
		if err != nil {
			return err
		}

		return fixtureMaterialMaps(directory, image.Rect(0, 0, config.Width, config.Height))
	}

	var group sync.WaitGroup

	for range 3 {
		group.Go(func() {
			if _, _, err := processWorldMaterials(context.Background(), root, manifest, document, textures, execute); err != nil {
				t.Error(err)
			}
		})
	}

	group.Wait()

	if calls.Load() != 4 {
		t.Fatalf("native invocations = %d", calls.Load())
	}
}

func TestMaterializeLogBounded(t *testing.T) {
	t.Parallel()

	var log materializeLog

	for range 3 {
		input := bytes.Repeat([]byte("x"), 64*1024)
		if count, err := log.Write(input); count != len(input) || err != nil {
			t.Fatal("log must drain child output")
		}
	}

	if log.buffer.Len() != 64*1024 {
		t.Fatal("unbounded child output")
	}
}
