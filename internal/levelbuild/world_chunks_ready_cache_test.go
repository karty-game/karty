package levelbuild

import (
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
)

//nolint:paralleltest,tparallel // Each mutation rewrites the same fixture cache; subtests must remain sequential.
func TestReadyChunkCacheRejectsHashValidPreparedCorruption(t *testing.T) {
	t.Parallel()

	fixture := chunkCacheSemanticFixture(t, false, true)
	for _, name := range []string{"missing entry", "missing marker", "wrong marker", "bad provenance", "packed coordinate", "noncanonical table", "semantic duplicate"} {
		t.Run(name, func(t *testing.T) {
			child := fixture.children[0]

			metadata, err := level.DecodeMetadata(child.Metadata)
			if err != nil {
				t.Fatal(err)
			}

			entries := chunkCacheSourceEntries(t, child)

			switch name {
			case "missing entry":
				entries = slices.DeleteFunc(entries, func(entry level.SourceEntry) bool { return entry.Name == worldchunk.ReadyEntryName })
			case "missing marker":
				delete(metadata, worldchunk.ReadyMetadataKey)
			case "wrong marker":
				metadata[worldchunk.ReadyMetadataKey] = level.StringValue("unknown@1")
			default:
				entries = mutateReadyChunkCacheEntries(t, fixture, name, entries)
			}

			catalog := fixture.catalog
			catalog.Chunks = slices.Clone(catalog.Chunks)
			writeCorruptSemanticChunkCache(t, fixture, metadata, entries, catalog)

			if _, found := readWorldChunkCache(t.Context(), fixture.root, fixture.directory, fixture.key,
				&fixture.document, fixture.definition, &worldchunk.ReadyOptions{}); found {
				t.Fatal("hash/size-valid corrupted ready stream reached publication")
			}
		})
	}
}

func mutateReadyChunkCacheEntries(t *testing.T, fixture chunkCacheValidationFixture,
	name string, entries []level.SourceEntry,
) []level.SourceEntry {
	t.Helper()

	child := fixture.children[0]

	values, err := readWorldChunkCacheEntries(child)
	if err != nil {
		t.Fatal(err)
	}

	geometry, err := rebuildCachedWorldChunkGeometry(child, fixture.catalog.Chunks[0], &fixture.document)
	if err != nil {
		t.Fatal(err)
	}

	ready, err := worldchunk.DecodeReady(values[worldchunk.ReadyEntryName], &geometry.mesh, geometry.encoded,
		&fixture.document, nil, worldchunk.ReadyOptions{})
	if err != nil {
		t.Fatal(err)
	}

	switch name {
	case "bad provenance":
		ready.MeshSHA256 = strings.Repeat("0", 64)
	case "packed coordinate":
		ready.Batches[0].Vertices[0] += 1
	case "noncanonical table":
		surface := ready.Surfaces[0]

		surface.SecondaryStrength, surface.BandCoverage = .125, 2
		if slices.Contains(ready.Surfaces, surface) {
			t.Fatal("noncanonical fixture must add a unique unused surface")
		}

		ready.Surfaces = append(ready.Surfaces, surface)
	case "semantic duplicate":
		entries = append(
			entries,
			level.SourceEntry{Name: worldchunk.MeshEntryName, Kind: level.EntryData, Data: geometry.encoded},
		)
	}

	encoded, err := worldchunk.EncodeReady(ready)
	if err != nil {
		t.Fatal(err)
	}
	// These mutations remain structurally safe for a checksum-approved
	// runtime payload. The offline cache must still reject them against
	// complete source geometry before publishing any new checksum.
	admission := worldchunk.ReadyAdmission{MaterialCount: len(world.MaterialIDs(&fixture.document)),
		Sectors: geometry.mesh.Sectors, Solids: geometry.mesh.Solids, Triangles: len(geometry.mesh.Triangles)}
	if _, err := worldchunk.DecodeReadyArtifact(encoded, admission); err != nil {
		t.Fatal("mutation fixture is not structurally admissible", err)
	}

	replaceChunkCacheEntry(entries, worldchunk.ReadyEntryName, encoded)

	return entries
}

func TestReadySelectionRejectsSemanticOnlyCache(t *testing.T) {
	t.Parallel()

	fixture := chunkCacheSemanticFixture(t, false)
	if _, found := readWorldChunkCache(t.Context(), fixture.root, fixture.directory, fixture.key,
		&fixture.document, fixture.definition, &worldchunk.ReadyOptions{}); found {
		t.Fatal("ready selection reused semantic-only cached children")
	}
}

func TestReadyChunksShareWholeLevelSurfaceTableAndAccountEncodedBytes(t *testing.T) {
	t.Parallel()
	fixture := chunkCacheSemanticFixture(t, false)
	document := fixture.document
	document.Sectors[1].FloorMaterial = 2
	definition := fixture.definition
	definition.Lightmap = nil

	metadata, err := level.EncodeMetadata(level.Metadata{})
	if err != nil {
		t.Fatal(err)
	}

	legacy, err := buildWorldChunks(t.Context(), "", definition, document, worldMaterialBuild{metadata: metadata}, nil)
	if err != nil {
		t.Fatal(err)
	}

	ready, err := buildWorldChunks(t.Context(), "", definition, document,
		worldMaterialBuild{metadata: metadata, ready: &worldchunk.ReadyOptions{}}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var (
		surfaces          []worldchunk.ReadySurface
		minimumReadyBytes uint64
	)

	for index, artifact := range ready.chunks {
		child, err := level.DecodePassiveModule(artifact.Bytes)
		if err != nil {
			t.Fatal(err)
		}

		entries, err := readWorldChunkCacheEntries(child)
		if err != nil {
			t.Fatal(err)
		}

		legacyChild, err := level.DecodePassiveModule(legacy.chunks[index].Bytes)
		if err != nil {
			t.Fatal(err)
		}

		encodedMesh, _, _ := legacyChild.Read(worldchunk.MeshEntryName, 0, worldchunk.MaxEncodedSize)

		mesh, err := worldchunk.DecodeMesh(encodedMesh, &document)
		if err != nil {
			t.Fatal(err)
		}

		streams, err := worldchunk.DecodeReady(entries[worldchunk.ReadyEntryName], &mesh, encodedMesh,
			&document, nil, *ready.ready)
		if err != nil || len(streams.Surfaces) < 2 {
			t.Fatal("whole-level table missing disjoint material", err)
		}

		if index == 0 {
			surfaces = streams.Surfaces
		} else if !slices.Equal(surfaces, streams.Surfaces) {
			t.Fatal("canonical table changed between chunks")
		}

		if entries[worldchunk.MeshEntryName] != nil || len(entries) != 1 {
			t.Fatal("ready-only module shipped duplicate semantic geometry")
		}

		if minimumReadyBytes == 0 {
			minimumReadyBytes = uint64(len(artifact.Bytes))
		} else {
			minimumReadyBytes = min(minimumReadyBytes, uint64(len(artifact.Bytes)))
		}
	}

	budget := minimumReadyBytes - 1

	definition.Chunks.MaxEncodedBytes = &budget
	if _, err := buildWorldChunks(t.Context(), "", definition, document,
		worldMaterialBuild{metadata: metadata, ready: &worldchunk.ReadyOptions{}}, nil); err == nil {
		t.Fatal("ready stream escaped the complete encoded chunk budget")
	}
}

func TestReadyChunkCompleteModuleBudgetRepartitions(t *testing.T) {
	t.Parallel()
	fixture := chunkCacheSemanticFixture(t, false)
	definition := fixture.definition
	definition.Lightmap = nil
	limit := 16
	definition.Chunks.MaxSectors = &limit

	metadata, err := level.EncodeMetadata(level.Metadata{})
	if err != nil {
		t.Fatal(err)
	}

	input := worldMaterialBuild{metadata: metadata, ready: &worldchunk.ReadyOptions{}}

	merged, err := buildWorldChunks(t.Context(), "", definition, fixture.document, input, nil)
	if err != nil || len(merged.chunks) != 1 {
		t.Fatal("ready fixture did not initially group adjacent rooms", err)
	}

	budget := uint64(len(merged.chunks[0].Bytes) - 1)
	definition.Chunks.MaxEncodedBytes = &budget

	split, err := buildWorldChunks(t.Context(), "", definition, fixture.document, input, nil)
	if err != nil || len(split.chunks) != 2 {
		t.Fatal("ready module budget did not repartition", err)
	}

	for _, chunk := range split.chunks {
		if uint64(len(chunk.Bytes)) > budget {
			t.Fatal("ready child exceeded complete module budget")
		}
	}
}
