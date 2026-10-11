package levelbuild

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

const chunkRoomsYAML = `rooms:
  - id: first
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 2, y: 0}, material: wall}
      - {id: east, start: {x: 2, y: 0}, end: {x: 2, y: 2}, material: wall}
      - {id: north, start: {x: 2, y: 2}, end: {x: 0, y: 2}, material: wall}
      - {id: west, start: {x: 0, y: 2}, end: {x: 0, y: 0}, material: wall}
    floor: {c: 0}
    ceiling: {c: 4}
    floor_material: wall
    ceiling_material: wall
    contents:
      - {id: spawn, kind: spawn, position: {x: 1, y: 1, z: 1}}
  - id: second
    boundary:
      - {id: south, start: {x: 2, y: 0}, end: {x: 4, y: 0}, material: wall}
      - {id: east, start: {x: 4, y: 0}, end: {x: 4, y: 2}, material: wall}
      - {id: north, start: {x: 4, y: 2}, end: {x: 2, y: 2}, material: wall}
      - {id: west, start: {x: 2, y: 2}, end: {x: 2, y: 0}, material: wall}
    floor: {c: 0}
    ceiling: {c: 4}
    floor_material: wall
    ceiling_material: wall
connections:
  - {id: door, a: {room: first, edge: east}, b: {room: second, edge: west}}
`

func enableFixtureChunks(t *testing.T, root, settings string) {
	t.Helper()

	path := filepath.Join(root, "levels", "lighting", "level.toml")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G703 -- path is constructed inside the isolated t.TempDir level fixture.
	if err := os.WriteFile(path, append(data, []byte("\n[chunks]\nenabled = true\n"+settings)...), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestChunkLevelPackagesIndependentPassiveWASM(t *testing.T) {
	t.Parallel()

	root := lightingLevelFixture(t, "version: 4\n"+chunkRoomsYAML)
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(
		selected.Assets.Capabilities.Runtime,
		asset.Capability(worldchunk.Feature),
		asset.CapabilityWorldMaterialAtlasV1,
	)
	assets := &assetBuild{projectRoot: root, manifest: selected, materials: func(_ context.Context, _ string, _ sdk.Manifest,
		document world.Document, _ map[uint32][]byte, _ ...map[uint32]assetpipeline.MaterializeOptions) (worldmaterial.Pair, assetpipeline.Artifact, error) {
		return fixtureWorldMaterialPair(t, assetpipeline.WorldMaterialIDs(document)), assetpipeline.Artifact{CacheKey: "fixture"}, nil
	}}
	enableFixtureChunks(t, root, "max_sectors = 1\n")

	first, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(err)
	}

	second, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil || len(first) != 1 || len(first[0].Chunks) != 2 || !bytes.Equal(first[0].Bytes, second[0].Bytes) {
		t.Fatalf("deterministic independent chunks: %v", err)
	}

	envelope := executeLightmapEnvelope(t, first[0].Bytes)
	if _, _, found := envelope.Read(worldmaterial.AlbedoEntry, 0, level.MaxEntrySize); !found {
		t.Fatal("root lost shared albedo atlas")
	}

	worldData, _, found := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)
	if !found {
		t.Fatal("root lost full world")
	}

	document, err := world.Decode(worldData)
	foundPortal := false

	if len(document.Sectors) == 2 {
		for _, wall := range document.Sectors[0].Walls {
			foundPortal = foundPortal || wall.Portal == 1
		}
	}

	if err != nil || len(document.Sectors) != 2 || !foundPortal || len(document.Contents) != 1 || len(first[0].AuthoredActors) != 1 {
		t.Fatalf("root topology or authored actor identity lost: %v", err)
	}

	catalogBytes, _, found := envelope.Read(worldchunk.CatalogEntryName, 0, worldchunk.MaxEncodedSize)
	if !found {
		t.Fatal("root missing catalog")
	}

	catalog, err := worldchunk.DecodeCatalog(catalogBytes, &document)
	if err != nil || len(catalog.Chunks) != 2 || !slices.Equal(catalog.Candidates[0], []int{0, 1}) {
		t.Fatalf("catalog: %+v, %v", catalog, err)
	}

	shadowBytes, _, found := envelope.Read(worldchunk.ShadowEntryName, 0, worldchunk.MaxShadowEncodedSize)
	if !found {
		t.Fatal("root missing independent opaque caster proxy")
	}

	shadow, err := worldchunk.DecodeShadowProxy(shadowBytes, &document)
	if err != nil || len(shadow.Triangles) != 20 {
		t.Fatalf("root caster proxy coverage: %d triangles, %v", len(shadow.Triangles), err)
	}

	checkBuiltChunks(t, document, catalog, first[0], second[0])
}

func checkBuiltChunks(t *testing.T, document world.Document, catalog worldchunk.Catalog, first, second Artifact) {
	t.Helper()

	for index, artifact := range first.Chunks {
		digest := sha256.Sum256(artifact.Bytes)
		if !bytes.Equal(artifact.Bytes, second.Chunks[index].Bytes) || artifact.ContentSHA256 != hex.EncodeToString(digest[:]) ||
			catalog.Chunks[index].ContentSHA256 != artifact.ContentSHA256 || catalog.Chunks[index].Size != uint64(len(artifact.Bytes)) {
			t.Fatal("child identity or deterministic bytes changed")
		}

		child := executeLightmapEnvelope(t, artifact.Bytes)

		meshBytes, _, found := child.Read(worldchunk.MeshEntryName, 0, worldchunk.MaxEncodedSize)
		if !found {
			t.Fatal("child missing prebuilt geometry")
		}

		mesh, err := worldchunk.DecodeMesh(meshBytes, &document)

		if err != nil || !slices.Equal(mesh.Sectors, catalog.Chunks[index].Sectors) || len(mesh.Vertices) == 0 {
			t.Fatalf("prebuilt mesh: %v", err)
		}

		metadata, err := level.DecodeMetadata(child.Metadata)
		marker, _ := metadata.String(worldchunk.ChunkMetadataKey)

		chunkID, _ := metadata.Uint("chunkID")
		if err != nil || marker != worldchunk.Schema || chunkID != uint64(index) {
			t.Fatalf("child metadata: %v", err)
		}

		if _, _, found := child.Read(world.EntryName, 0, world.MaxEncodedSize); found {
			t.Fatal("child duplicates logical world and actor data")
		}

		if _, _, found := child.Read(worldmaterial.AlbedoEntry, 0, level.MaxEntrySize); found {
			t.Fatal("child duplicates shared albedo atlas")
		}

		if _, _, found := child.Read(worldchunk.ShadowEntryName, 0, worldchunk.MaxShadowEncodedSize); found {
			t.Fatal("child duplicates root shadow proxy")
		}

		if _, _, found := child.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize); found {
			t.Fatal("mesh-only level acquired an artificial lightmap")
		}
	}
}

func TestChunkLevelTransformedPortalSeparatesAuthoredSpaces(t *testing.T) {
	t.Parallel()

	parts := strings.SplitN(chunkRoomsYAML, "  - id: second", 2)
	destination := strings.ReplaceAll(strings.ReplaceAll(parts[1], "x: 2", "x: 20"), "x: 4", "x: 22")
	source := parts[0] + "  - id: second" + destination
	source = strings.Replace(source, "b: {room: second, edge: west}}", "b: {room: second, edge: west}, non_euclidean: true}", 1)
	root := lightingLevelFixture(t, "version: 4\n"+source)
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.Capability(worldchunk.Feature))

	enableFixtureChunks(t, root, "max_sectors = 16\n")

	artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || len(artifacts) != 1 || len(artifacts[0].Chunks) != 2 {
		t.Fatalf("transformed portals must preserve independently authored chunks: %v", err)
	}

	envelope := executeLightmapEnvelope(t, artifacts[0].Bytes)
	encoded, _, _ := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	catalogBytes, _, _ := envelope.Read(worldchunk.CatalogEntryName, 0, worldchunk.MaxEncodedSize)

	catalog, err := worldchunk.DecodeCatalog(catalogBytes, &document)
	if err != nil || !slices.Equal(catalog.Candidates[0], []int{0, 1}) || catalog.Chunks[1].Bounds[0] < 20 {
		t.Fatalf("portal reachability or authored coordinates were lost: %+v, %v", catalog, err)
	}
}

func TestChunkLevelBakesSeparateReceiverAtlases(t *testing.T) {
	t.Parallel()

	root := lightingLevelFixture(t, "version: 4\n"+chunkRoomsYAML+directLightmapYAML)
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.Capability(worldchunk.Feature))

	enableFixtureLightmap(
		t,
		root,
		"enabled = true\noffline = true\nlights = ['red','blue']\npage_size = 512\ndensity = 1\nbake_samples = 1\nbake_bounces = 0\nbake_denoise = 'off'\n",
	)
	enableFixtureChunks(t, root, "max_sectors = 1\nmax_lightmap_texels = 262144\n")

	artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}

	rootEnvelope := executeLightmapEnvelope(t, artifacts[0].Bytes)
	encodedWorld, _, _ := rootEnvelope.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(encodedWorld)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, found := rootEnvelope.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize); found {
		t.Fatal("chunk root still packages one whole-world lightmap")
	}

	if len(artifacts[0].Chunks) != 2 {
		t.Fatal("missing independent lightmapped children")
	}

	for _, artifact := range artifacts[0].Chunks {
		child := executeLightmapEnvelope(t, artifact.Bytes)
		layoutBytes, _, _ := child.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)

		layout, err := worldlightmap.Decode(layoutBytes, &document)
		if err != nil || layout.Receivers == nil || len(layout.Receivers.Sectors) != 1 || layout.TexelsPerUnit != 1 {
			t.Fatalf("receiver-scoped layout: %v", err)
		}

		manifest, _, _ := child.Read(worldlightmap.PrebakeEntryName, 0, worldlightmap.MaxPrebakeManifestSize)

		image, _, _ := child.Read(worldlightmap.PrebakeImageEntryName, 0, worldlightmap.MaxPrebakeImageSize)
		if _, err := worldlightmap.DecodePrebake(manifest, image, layout, &document); err != nil {
			t.Fatalf("child incomplete offline bake: %v", err)
		}
	}
}

func TestChunkLevelBudgetIncludesCompletePassiveModule(t *testing.T) {
	t.Parallel()

	root := lightingLevelFixture(t, "version: 4\n"+chunkRoomsYAML)
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.Capability(worldchunk.Feature))

	enableFixtureChunks(t, root, "max_sectors = 16\n")

	first, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || len(first) != 1 || len(first[0].Chunks) != 1 {
		t.Fatalf("ordinary adjacency did not group: %v", err)
	}

	budget := len(first[0].Chunks[0].Bytes) - 1
	path := filepath.Join(root, "levels", "lighting", "level.toml")

	definition, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G703 -- path is constructed inside the isolated t.TempDir level fixture.
	if err := os.WriteFile(path, append(definition, fmt.Appendf(nil, "max_encoded_bytes = %d\n", budget)...), 0o600); err != nil {
		t.Fatal(err)
	}

	split, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || len(split) != 1 || len(split[0].Chunks) != 2 {
		t.Fatalf("module budget did not repartition: %v", err)
	}

	for _, child := range split[0].Chunks {
		if len(child.Bytes) > budget {
			t.Fatal("final child exceeds complete artifact budget")
		}
	}
}

func TestChunkLevelRejectsUnsupportedCapabilitiesAndBudgets(t *testing.T) {
	t.Parallel()

	for _, settings := range []string{"", "max_sectors = 0\n", "max_mesh_bytes = 1\n", "max_encoded_bytes = 1\n", "max_lightmap_texels = 1\n"} {
		t.Run(settings, func(t *testing.T) {
			t.Parallel()

			root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML)
			selected := directLightmapSDK(t)

			if settings == "" {
				selected = withoutRuntimeCapabilities(selected, asset.Capability(worldchunk.Feature))
			} else {
				selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.Capability(worldchunk.Feature))
			}

			enableFixtureChunks(t, root, settings)

			artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
			if err == nil || artifacts != nil || settings == "" && !errors.Is(err, ErrManifest) {
				t.Fatalf("invalid chunk setup published artifacts: %v", err)
			}
		})
	}
}

func TestChunkLevelExplicitBakeDirectsToBuild(t *testing.T) {
	t.Parallel()

	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML)
	enableFixtureChunks(t, root, "")

	_, err := BakeAll(t.Context(), root, BakeOptions{Level: "levels.lighting"}, nil)
	if !errors.Is(err, ErrManifest) || !strings.Contains(err.Error(), "run karty build") {
		t.Fatalf("whole-world bake attempted on chunked level: %v", err)
	}
}

func TestChunkShadowBudgetDoesNotConstrainRenderGeometry(t *testing.T) {
	t.Parallel()

	document := world.Document{Version: world.Version, Sectors: make([]world.Sector, 32)}
	for index := range document.Sectors {
		sector := world.Sector{
			ID:         fmt.Sprintf("room-%d", index),
			SourceRoom: "room",
			Ceiling:    world.Plane{C: 4},
			Walls:      make([]world.Wall, 72),
		}

		points := make([]world.Vec2, len(sector.Walls))
		for edge := range points {
			angle := float64(edge) * 2 * math.Pi / float64(len(points))
			points[edge] = world.Vec2{X: float64(index)*4 + math.Cos(angle), Y: math.Sin(angle)}
		}

		for edge, point := range points {
			sector.Walls[edge] = world.Wall{Start: point, End: points[(edge+1)%len(points)], Portal: -1}
		}

		document.Sectors[index] = sector
	}

	if _, err := worldchunk.CompileShadowProxy(document); !errors.Is(err, worldchunk.ErrShadowBudget) {
		t.Fatalf("fixture must exceed only the caster bound: %v", err)
	}

	encoded, err := encodeWorldChunkShadow(document)
	if err != nil || encoded != nil {
		t.Fatalf("render-only world rejected by optional caster budget: %v", err)
	}

	document.Sectors[0].Floor.C = math.NaN()
	if _, err := encodeWorldChunkShadow(document); err == nil {
		t.Fatal("malformed source was mistaken for an optional caster budget overflow")
	}
}
