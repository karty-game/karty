package levelbuild

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func incrementalBakeFixture(t *testing.T) (world.Document, []worldlightmapbake.Material, *chunkPixelCache) {
	t.Helper()
	root := t.TempDir()

	directory := filepath.Join(root, "levels", "test")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}

	cache, err := prepareChunkPixelCache(root, directory)
	if err != nil {
		t.Fatal(err)
	}

	document := world.Document{Version: world.Version,
		Lighting: &world.Lighting{Version: world.LightingVersion, Lights: []world.PointLight{
			{ID: "near", Position: world.Vec3{X: 1, Y: 1, Z: 1}, Color: world.Vec3{X: 1, Y: .4, Z: .2}, Radius: 20},
			{ID: "far", Position: world.Vec3{X: 31, Y: 1, Z: 1}, Color: world.Vec3{X: .2, Y: .4, Z: 1}, Radius: 20},
		}}}
	for index, interval := range [][2]float64{{0, 2}, {2, 30}, {30, 32}} {
		points := []world.Vec2{{X: interval[0]}, {X: interval[1]}, {X: interval[1], Y: 2}, {X: interval[0], Y: 2}}

		walls := make([]world.Wall, 4)
		for edge, point := range points {
			walls[edge] = world.Wall{Start: point, End: points[(edge+1)%4], Material: 1, Portal: -1, SourceEdge: "wall"}
		}

		if index < 2 {
			walls[1].Portal = int32(index + 1)
			walls[1].PortalWall = 4
		}

		if index > 0 {
			walls[3].Portal = int32(index - 1)
			walls[3].PortalWall = 2
		}

		id := []string{"near-room", "corridor", "far-room"}[index]
		document.Sectors = append(document.Sectors, world.Sector{ID: id, SourceRoom: id, Walls: walls,
			Floor: world.Plane{}, Ceiling: world.Plane{C: 3}, FloorMaterial: 1, CeilingMaterial: 1})
	}

	albedo := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	albedo.SetNRGBA(0, 0, color.NRGBA{R: 180, G: 180, B: 180, A: 255})

	return document, []worldlightmapbake.Material{{ID: 1, Albedo: albedo}}, cache
}

func incrementalChunkLayouts(t *testing.T, document world.Document) []worldlightmap.Layout {
	t.Helper()

	plan, err := worldchunk.Plan(document, worldchunk.Options{MaxSectors: 1, Lightmap: worldlightmap.Options{
		AllStatic: true, TexelsPerUnit: 1, PageSize: 512, Padding: 2, ShadowSize: 32,
	}})
	if err != nil || len(plan.Chunks) != 3 {
		t.Fatalf("incremental plan: %d chunks, %v", len(plan.Chunks), err)
	}

	layouts := make([]worldlightmap.Layout, len(plan.Chunks))
	for index, chunk := range plan.Chunks {
		layouts[index] = chunk.Layout
	}

	return layouts
}

func cloneIncrementalWorld(t *testing.T, document world.Document) world.Document {
	t.Helper()

	encoded, err := world.Encode(document)
	if err != nil {
		t.Fatal(err)
	}

	cloned, err := world.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	return cloned
}

func bakeIncrementalChunks(t *testing.T, document world.Document, materials []worldlightmapbake.Material,
	options worldlightmapbake.Options, cache *chunkPixelCache) [][]level.SourceEntry {
	t.Helper()
	layouts := incrementalChunkLayouts(t, document)

	result := make([][]level.SourceEntry, len(layouts))
	for index, layout := range layouts {
		entries, _, err := bakeWorldChunkLightmap(t.Context(), document, layout, materials, options, cache)
		if err != nil {
			t.Fatalf("bake chunk %d: %v", index, err)
		}

		result[index] = entries
	}

	return result
}

func assertIncrementalEqualsFresh(t *testing.T, document world.Document, materials []worldlightmapbake.Material,
	options worldlightmapbake.Options, cached [][]level.SourceEntry) {
	t.Helper()
	fresh := bakeIncrementalChunks(t, document, materials, options, nil)

	for index := range cached {
		for entry := range cached[index] {
			if cached[index][entry].Name != fresh[index][entry].Name || !bytes.Equal(cached[index][entry].Data, fresh[index][entry].Data) {
				t.Fatalf("cached chunk %d %s differs from fresh bounded bake", index, cached[index][entry].Name)
			}
		}
	}
}

//nolint:paralleltest,tparallel // Sequential controls compare cache state after the shared baseline bake.
func TestChunkPixelCacheLocalEditsAndHistoricalReuse(t *testing.T) {
	t.Parallel()
	document, materials, cache := incrementalBakeFixture(t)
	options := worldlightmapbake.Options{Samples: 1, Bounces: 1, Workers: 1, Seed: 1, Denoise: "off", MaxRayDistance: 4}

	baseline := bakeIncrementalChunks(t, document, materials, options, cache)
	if cache.hits != 0 || cache.misses != 3 {
		t.Fatalf("cold bake: reused=%d baked=%d", cache.hits, cache.misses)
	}

	layouts := incrementalChunkLayouts(t, document)

	nearKey, err := worldlightmapbake.BakeInputDigest(document, layouts[0], materials, options)
	if err != nil {
		t.Fatal(err)
	}

	nearPath := filepath.Join(cache.directory, nearKey+".bin")

	oldTime := time.Unix(100, 0)
	if err := os.Chtimes(nearPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Several edits can be compared with an arbitrarily old cached input; no
	// previous checkout, commit ID, modification time or change log is needed.
	changed := cloneIncrementalWorld(t, document)
	changed.Sectors[2].Ceiling.C = 2.8
	changed.Lighting.Lights[1].Color.Z = .7
	changed.Lighting.Lights[1].Radius = 19
	cache.hits, cache.misses = 0, 0

	incremental := bakeIncrementalChunks(t, changed, materials, options, cache)
	if cache.hits != 1 || cache.misses != 2 {
		t.Fatalf("far edits: reused=%d baked=%d, want 1/2", cache.hits, cache.misses)
	}

	info, err := os.Stat(nearPath)
	if err != nil || !info.ModTime().Equal(oldTime) {
		t.Fatalf("unaffected pixel/image entry was rewritten: %v", err)
	}

	if !bytes.Equal(baseline[0][2].Data, incremental[0][2].Data) || bytes.Equal(baseline[0][1].Data, incremental[0][1].Data) {
		t.Fatal("far edit must preserve pixels and regenerate the complete-document manifest")
	}

	assertIncrementalEqualsFresh(t, changed, materials, options, incremental)

	cache.hits, cache.misses = 0, 0

	reverted := bakeIncrementalChunks(t, document, materials, options, cache)
	if cache.hits != 3 || cache.misses != 0 {
		t.Fatalf("revert-to-old-input did not reuse historical cache: %d/%d", cache.hits, cache.misses)
	}

	assertIncrementalEqualsFresh(t, document, materials, options, reverted)

	for _, control := range []struct {
		name string
		edit func(*worldlightmapbake.Options)
	}{
		{"samples", func(o *worldlightmapbake.Options) { o.Samples = 2 }},
		{"distance", func(o *worldlightmapbake.Options) { o.MaxRayDistance = 5 }},
		{"producer", func(o *worldlightmapbake.Options) { o.Denoise = "low" }},
	} {
		t.Run(control.name, func(t *testing.T) {
			changedOptions := options
			control.edit(&changedOptions)

			cache.hits, cache.misses = 0, 0

			output := bakeIncrementalChunks(t, document, materials, changedOptions, cache)
			if cache.hits != 0 || cache.misses != 3 {
				t.Fatalf("changed controls reused stale pixels: %d/%d", cache.hits, cache.misses)
			}

			assertIncrementalEqualsFresh(t, document, materials, changedOptions, output)
		})
	}
}

func TestChunkPixelCacheWallsAndMovingLightInfluence(t *testing.T) {
	t.Parallel()

	for _, edit := range []struct {
		name   string
		change func(*world.Document)
		hits   int
	}{
		{"near-wall", func(d *world.Document) { d.Sectors[0].Ceiling.C = 2.7 }, 1},
		{"add-wall", func(d *world.Document) {
			d.StaticSolids = &world.StaticSolids{Version: world.StaticSolidsVersion, Items: []world.Solid{{
				ID: "wall", Footprint: []world.Vec2{{X: 3, Y: .5}, {X: 3.2, Y: .5}, {X: 3.2, Y: 1.5}, {X: 3, Y: 1.5}},
				Top: world.Plane{C: 2}, SideMaterial: 1, TopMaterial: 1, BottomMaterial: 1,
			}}}
		}, 1},
		{"move-light", func(d *world.Document) { d.Lighting.Lights[1].Position.X = 1.5 }, 0},
	} {
		t.Run(edit.name, func(t *testing.T) {
			t.Parallel()
			document, materials, cache := incrementalBakeFixture(t)
			options := worldlightmapbake.Options{Samples: 1, Bounces: 1, Workers: 1, Seed: 1, Denoise: "off", MaxRayDistance: 4}
			bakeIncrementalChunks(t, document, materials, options, cache)
			changed := cloneIncrementalWorld(t, document)
			edit.change(&changed)

			cache.hits, cache.misses = 0, 0

			output := bakeIncrementalChunks(t, changed, materials, options, cache)
			if cache.hits != edit.hits || cache.misses != 3-edit.hits {
				t.Fatalf("edit influence reused=%d baked=%d want %d/%d", cache.hits, cache.misses, edit.hits, 3-edit.hits)
			}

			assertIncrementalEqualsFresh(t, changed, materials, options, output)

			if edit.name == "add-wall" {
				cache.hits, cache.misses = 0, 0
				bakeIncrementalChunks(t, document, materials, options, cache)

				if cache.hits != 3 {
					t.Fatal("removing a wall did not recover exact previous input cache")
				}
			}
		})
	}
}

func TestUnboundedChunkPixelCacheKeepsBroadDependencies(t *testing.T) {
	t.Parallel()
	document, materials, cache := incrementalBakeFixture(t)
	options := worldlightmapbake.Options{Samples: 1, Bounces: 0, Workers: 1, Seed: 1, Denoise: "off"}
	bakeIncrementalChunks(t, document, materials, options, cache)
	document.Sectors[2].Ceiling.C = 2.8
	cache.hits, cache.misses = 0, 0

	output := bakeIncrementalChunks(t, document, materials, options, cache)
	if cache.hits != 0 || cache.misses != 3 {
		t.Fatalf("unbounded far geometry changed without broad invalidation: %d/%d", cache.hits, cache.misses)
	}

	assertIncrementalEqualsFresh(t, document, materials, options, output)
}

func TestBakeMaxRayDistanceSettings(t *testing.T) {
	t.Parallel()

	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1), world.MaxCoordinate + 1} {
		settings := lightmapBuildSettings{BakeMaxRayDistance: &value}
		if _, err := settings.options(); err == nil {
			t.Fatalf("invalid ray distance accepted: %g", value)
		}
	}

	for _, value := range []float64{.1, 16, world.MaxCoordinate} {
		settings := lightmapBuildSettings{BakeMaxRayDistance: &value}
		if _, err := settings.options(); err != nil || chunkBakeOptions(&settings).MaxRayDistance != value {
			t.Fatalf("valid ray distance rejected: %g, %v", value, err)
		}
	}

	if chunkBakeOptions(nil).MaxRayDistance != 0 {
		t.Fatal("absent distance changed unbounded defaults")
	}
}

// Ensure cancellation remains authoritative even when pixels are already cached.
func TestChunkPixelCacheCanceledHitDoesNotPublish(t *testing.T) {
	t.Parallel()
	document, materials, cache := incrementalBakeFixture(t)
	options := worldlightmapbake.Options{Samples: 1, Workers: 1, Denoise: "off", MaxRayDistance: 4}

	layout := incrementalChunkLayouts(t, document)[0]
	if _, _, err := bakeWorldChunkLightmap(t.Context(), document, layout, materials, options, cache); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, _, err := bakeWorldChunkLightmap(ctx, document, layout, materials, options, cache); err == nil {
		t.Fatal("canceled cached bake returned artifact")
	}
}

func TestChunkPixelCacheIncompleteCorruptAndEscapedEntries(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"missing", "truncated", "checksum", "producer", "dependency", "partial", "dimensions", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			document, materials, cache := incrementalBakeFixture(t)
			options := worldlightmapbake.Options{Samples: 1, Workers: 1, Seed: 1, Denoise: "off", MaxRayDistance: 4}
			layout := incrementalChunkLayouts(t, document)[0]

			original, _, err := bakeWorldChunkLightmap(t.Context(), document, layout, materials, options, cache)
			if err != nil {
				t.Fatal(err)
			}

			key, err := worldlightmapbake.BakeInputDigest(document, layout, materials, options)
			if err != nil {
				t.Fatal(err)
			}

			path := filepath.Join(cache.directory, key+".bin")

			encoded, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			outside := corruptChunkPixelFixture(t, mode, path, encoded)

			cache.hits, cache.misses = 0, 0

			repaired, _, err := bakeWorldChunkLightmap(t.Context(), document, layout, materials, options, cache)
			if err != nil || cache.hits != 0 || cache.misses != 1 {
				t.Fatalf("incomplete/corrupt cache did not cold bake: %d/%d, %v", cache.hits, cache.misses, err)
			}

			for index := range original {
				if !bytes.Equal(original[index].Data, repaired[index].Data) {
					t.Fatal("repaired cache differs from fresh output")
				}
			}

			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				t.Fatal("cache replacement is not a regular confined file")
			}

			assertEscapedChunkPixelFileUntouched(t, outside, encoded)
		})
	}
}

func TestChunkPixelCacheRejectsDirectorySymlink(t *testing.T) {
	t.Parallel()

	_, _, cache := incrementalBakeFixture(t)
	if err := os.Remove(cache.directory); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(t.TempDir(), cache.directory); err != nil {
		t.Skip("symlinks unavailable:", err)
	}

	if _, err := prepareChunkPixelCache(cache.root, filepath.Join(cache.root, "levels", "test")); err == nil {
		t.Fatal("escaped pixel cache directory accepted")
	}
}

func TestBoundedBakeAutomaticAndImportedDistanceExpectations(t *testing.T) {
	t.Parallel()
	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(t, root, automaticBakeSettings+"lights = ['red','blue']\nbake_max_ray_distance = 4\n")

	reports, err := BakeAll(t.Context(), root, BakeOptions{Workers: 1}, nil)
	if err != nil || len(reports) != 1 {
		t.Fatalf("bounded standalone bake: %v", err)
	}

	encoded, err := os.ReadFile(reports[0].Manifest)
	if err != nil {
		t.Fatal(err)
	}

	header, err := worldlightmap.DecodePrebakeManifest(encoded)
	if err != nil || header.MaxRayDistance != 4 || header.Producer != worldlightmap.BoundedOfflinePrebakeProducer {
		t.Fatalf("bounded manifest did not bind distance/producer: %+v, %v", header, err)
	}

	selected := directLightmapSDK(t)
	build := func(want bool) {
		t.Helper()

		artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
		if err != nil || len(artifacts) != 1 || slices.Contains(artifacts[0].Features, worldlightmap.PrebakeFeature) != want {
			t.Fatalf("bounded artifact availability want=%v: %v", want, err)
		}
	}
	build(true)

	definitionPath := filepath.Join(root, "levels", "lighting", "level.toml")

	definition, err := os.ReadFile(definitionPath)
	if err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(
		t,
		definitionPath,
		[]byte(strings.Replace(string(definition), "bake_max_ray_distance = 4", "bake_max_ray_distance = 5", 1)),
	)
	build(false)
	writePrebakeFixture(t, definitionPath, []byte(strings.Replace(string(definition), "bake_max_ray_distance = 4\n", "", 1)))
	build(false)
	writePrebakeFixture(t, definitionPath, definition)
	build(true)

	directory := filepath.Dir(definitionPath)
	writePrebakeFixture(t, filepath.Join(directory, "import.bin"), encoded)

	pixels, err := os.ReadFile(reports[0].Image)
	if err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(t, filepath.Join(directory, "import.qoi"), pixels)

	imported := strings.Replace(string(definition), "offline = true", "prebake = 'import.bin'\nprebake_image = 'import.qoi'", 1)
	writePrebakeFixture(t, definitionPath, []byte(imported))
	build(true)
	writePrebakeFixture(t, definitionPath, []byte(strings.Replace(imported, "bake_max_ray_distance = 4", "bake_max_ray_distance = 5", 1)))

	if _, err := BuildAllWithAssets(t.Context(), root, 4, "", selected); err == nil {
		t.Fatal("explicit import accepted distance mismatching level settings")
	}
}

func TestLegacyDirectImportRejectsBoundedDistanceExpectation(t *testing.T) {
	t.Parallel()
	root, selected := prebakeLevelFixture(t)
	addPrebakeControls(t, root, "prebake = 'prebake.bin'\nprebake_image = 'prebake.qoi'\nbake_max_ray_distance = 4\n")

	if _, err := BuildAllWithAssets(t.Context(), root, 4, "", selected); err == nil {
		t.Fatal("unbounded direct import accepted bounded level expectation")
	}
}

func corruptChunkPixelFixture(t *testing.T, mode, path string, encoded []byte) string {
	t.Helper()

	var outside string

	switch mode {
	case "missing":
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	case "symlink":
		outside = filepath.Join(t.TempDir(), "outside.bin")
		// #nosec G703 -- outside is an isolated t.TempDir fixture used to test escaping symlinks.
		if err := os.WriteFile(outside, encoded, 0o600); err != nil {
			t.Fatal(err)
		}

		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}

		if err := os.Symlink(outside, path); err != nil {
			t.Skip("symlinks unavailable:", err)
		}
	case "truncated":
		// #nosec G703 -- path is the generated fixture cache entry beneath t.TempDir.
		if err := os.WriteFile(path, encoded[:len(encoded)-5], 0o600); err != nil {
			t.Fatal(err)
		}
	default:
		corruptChunkPixelEnvelope(t, mode, path, encoded)
	}

	return outside
}

func corruptChunkPixelEnvelope(t *testing.T, mode, path string, encoded []byte) {
	t.Helper()

	envelope, err := level.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := level.DecodeMetadata(envelope.Metadata)
	if err != nil {
		t.Fatal(err)
	}

	pixels, _, _ := envelope.Read(chunkPixelCacheEntry, 0, worldlightmap.MaxPrebakeImageSize)
	entries := []level.SourceEntry{{Name: chunkPixelCacheEntry, Kind: level.EntryData, Data: pixels}}

	switch mode {
	case "checksum":
		metadata["imageSha256"] = level.StringValue(string(bytes.Repeat([]byte{'a'}, 64)))
	case "producer":
		metadata["producer"] = level.StringValue("obsolete-producer")
	case "dependency":
		metadata["dependencySha256"] = level.StringValue(string(bytes.Repeat([]byte{'a'}, 64)))
	case "partial":
		entries = nil
	case "dimensions":
		wrong, _, err := qoi.Encode(
			image.NewNRGBA(image.Rect(0, 0, 1, 1)),
			qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear},
		)
		if err != nil {
			t.Fatal(err)
		}

		digest := sha256.Sum256(wrong)
		metadata["imageSha256"] = level.StringValue(hex.EncodeToString(digest[:]))
		entries[0].Data = wrong
	}

	meta, err := level.EncodeMetadata(metadata)
	if err != nil {
		t.Fatal(err)
	}

	bad, err := level.Encode(meta, entries)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, bad, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertEscapedChunkPixelFileUntouched(t *testing.T, outside string, encoded []byte) {
	t.Helper()

	if outside != "" {
		after, err := os.ReadFile(outside)
		if err != nil || !bytes.Equal(after, encoded) {
			t.Fatal("cache repair wrote through escaping symlink")
		}
	}
}
