package levelbuild

import (
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func TestReadyCleanupFinalMeshBudgetRepartitionsIncreasedTriangleCount(t *testing.T) {
	t.Parallel()

	fixture := chunkCacheSemanticFixture(t, false)
	document := fixture.document
	material := document.Sectors[0].FloorMaterial
	document.StaticSolids = &world.StaticSolids{Version: 1, Items: []world.Solid{{
		ID: "floor-occluder", Footprint: []world.Vec2{{X: .85, Y: .1}, {X: 1.15, Y: .1}, {X: 1.15, Y: .4}, {X: .85, Y: .4}},
		Bottom: world.Plane{}, Top: world.Plane{C: 1}, SideMaterial: material, TopMaterial: material, BottomMaterial: material,
		Collision: true,
	}}}
	definition := fixture.definition
	definition.Lightmap = nil
	settings := *definition.Chunks
	definition.Chunks = &settings
	limit := 16
	definition.Chunks.MaxSectors = &limit

	metadata, err := level.EncodeMetadata(level.Metadata{})
	if err != nil {
		t.Fatal(err)
	}

	input := worldMaterialBuild{metadata: metadata, ready: &worldchunk.ReadyOptions{CleanupStatic: true}}

	merged, err := buildWorldChunks(t.Context(), "", definition, document, input, nil)
	if err != nil || len(merged.chunks) != 1 {
		t.Fatal("cleanup fixture did not group adjacent sectors", err)
	}

	descriptor, canonicalBytes := validateCleanedChunkCount(t, merged, &document)
	if descriptor.MeshBytes <= canonicalBytes {
		t.Fatal("floor clipping fixture must increase triangles after removing covered geometry")
	}

	t.Logf("merged canonical triangles %d, cleaned triangles %d", canonicalBytes/484, descriptor.MeshBytes/484)

	budget := canonicalBytes
	definition.Chunks.MaxMeshBytes = &budget

	split, err := buildWorldChunks(t.Context(), "", definition, document, input, nil)
	if err != nil || len(split.chunks) != 2 {
		t.Fatal("actual cleanup triangle increase did not trigger mesh-budget repartition", err)
	}

	for _, artifact := range split.chunks {
		child := decodePassiveEnvelope(t, artifact.Bytes)

		entries, err := readWorldChunkCacheEntries(child)
		if err != nil {
			t.Fatal(err)
		}

		if len(entries[worldchunk.ReadyEntryName]) == 0 || entries[worldchunk.MeshEntryName] != nil {
			t.Fatal("cleanup repartition lost ready-only shipping")
		}
	}

	if err := writeWorldChunkCache(t.Context(), fixture.root, fixture.directory, "cleanup", split); err != nil {
		t.Fatal(err)
	}

	if _, found := readWorldChunkCache(t.Context(), fixture.root, fixture.directory, "cleanup", &document,
		definition, input.ready); !found {
		t.Fatal("full offline cache oracle rejected deterministic cleaned geometry")
	}

	if _, found := readWorldChunkCache(t.Context(), fixture.root, fixture.directory, "cleanup", &document,
		definition, &worldchunk.ReadyOptions{}); found {
		t.Fatal("cache accepted cleaned streams without the selected cleanup proof")
	}

	limit = 1
	definition.Chunks.MaxSectors = &limit

	plan, err := worldchunk.Plan(document, worldchunk.Options{NoLightmap: true, MaxSectors: 1})
	if err != nil {
		t.Fatal(err)
	}

	budget = plan.Chunks[0].MeshBytes

	if _, err := buildWorldChunks(
		t.Context(),
		"",
		definition,
		document,
		input,
		nil,
	); err == nil ||
		!strings.Contains(err.Error(), "mesh bytes") {
		t.Fatal("single-sector final mesh overflow was not rejected", err)
	}
}

func validateCleanedChunkCount(t *testing.T, result worldMaterialBuild, document *world.Document) (worldchunk.Chunk, uint64) {
	t.Helper()

	var encodedCatalog []byte

	for _, entry := range result.data {
		if entry.Name == worldchunk.CatalogEntryName {
			encodedCatalog = entry.Data
		}
	}

	catalog, err := worldchunk.DecodeCatalog(encodedCatalog, document)
	if err != nil || len(catalog.Chunks) != 1 {
		t.Fatal("cleaned catalog was not complete", err)
	}

	descriptor := catalog.Chunks[0]

	mesh, err := worldchunk.CompileMesh(*document, descriptor.Sectors, descriptor.Solids, worldlightmap.Layout{})
	if err != nil {
		t.Fatal(err)
	}

	encodedMesh, err := worldchunk.EncodeMesh(mesh, document)
	if err != nil {
		t.Fatal(err)
	}

	child := decodePassiveEnvelope(t, result.chunks[0].Bytes)
	encoded, _, _ := child.Read(worldchunk.ReadyEntryName, 0, worldchunk.MaxReadyBytes)

	ready, err := worldchunk.DecodeReady(encoded, &mesh, encodedMesh, document, nil, *result.ready)
	if err != nil || readyChunkMeshBytes(&ready) != descriptor.MeshBytes || !slices.Equal(ready.Surfaces, result.ready.Surfaces) {
		t.Fatal("descriptor count does not match fully proved cleanup streams", err)
	}

	return descriptor, worldchunk.DecodedMeshBytes(mesh)
}
