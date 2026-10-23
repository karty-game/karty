package levelbuild

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

func TestReadyChunksSelectedCapabilityDeterminismAndMaskedSurfaces(t *testing.T) {
	t.Parallel()

	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "ready"}[enabled], func(t *testing.T) {
			t.Parallel()
			first, second := readySelectedArtifacts(t, enabled)
			document, catalog := readySelectedRoot(t, first, enabled)

			var surfaces []worldchunk.ReadySurface

			for index, chunk := range first.Chunks {
				if !bytes.Equal(chunk.Bytes, second.Chunks[index].Bytes) {
					t.Fatal("cached child bytes changed")
				}

				current := readySelectedChild(t, chunk, catalog.Chunks[index], &document, enabled)
				if surfaces == nil {
					surfaces = current
				} else if !slices.Equal(surfaces, current) {
					t.Fatal("children did not share a canonical surface table")
				}
			}
		})
	}
}
func readySelectedArtifacts(t *testing.T, enabled bool) (Artifact, Artifact) {
	t.Helper()
	root := lightingLevelFixture(t, "version: 4\n"+chunkRoomsYAML)
	selected := directLightmapSDK(t)

	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime,
		asset.Capability(worldchunk.Feature), asset.CapabilityWorldMaterialAtlasV2, asset.CapabilityWorldMaterialLayersV1)
	if enabled {
		selected.Assets.Capabilities.Runtime = append(
			selected.Assets.Capabilities.Runtime,
			asset.Capability(worldchunk.ReadyFeature),
		)
	}

	assets := &assetBuild{projectRoot: root, manifest: selected, materials: func(_ context.Context, _ string, _ sdk.Manifest,
		document world.Document, _ map[uint32][]byte, _ ...map[uint32]assetpipeline.MaterializeOptions,
	) (worldmaterial.Pair, assetpipeline.Artifact, error) {
		pair := fixtureWorldMaterialPair(t, assetpipeline.WorldMaterialIDs(document))

		pair.Layout.Schema = worldmaterial.SchemaV2
		for index := range pair.Layout.Materials {
			pair.Layout.Materials[index].Coverage = worldmaterial.CoverageOpaque
		}

		pair.Layout.Materials[len(pair.Layout.Materials)-1].Coverage = worldmaterial.CoverageMasked

		return pair, assetpipeline.Artifact{CacheKey: "fixture"}, nil
	}}
	enableFixtureChunks(t, root, "max_sectors = 1\n")

	first, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(err)
	}

	second, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil || len(first) != 1 || len(first[0].Chunks) != 2 || !bytes.Equal(first[0].Bytes, second[0].Bytes) {
		t.Fatal("selected ready output was not deterministic", err)
	}

	return first[0], second[0]
}
func readySelectedRoot(t *testing.T, first Artifact, enabled bool) (world.Document, worldchunk.Catalog) {
	t.Helper()

	if slices.Contains(first.Features, worldchunk.ReadyFeature) != enabled {
		t.Fatal("selected SDK did not control ready feature")
	}

	parent, err := level.DecodePassiveModule(first.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := level.DecodeMetadata(parent.Metadata)
	if err != nil || chunkMetadataPresent(metadata, worldchunk.ReadyMetadataKey) != enabled {
		t.Fatal("root ready marker mismatch", err)
	}

	worldBytes, _, _ := parent.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(worldBytes)
	if err != nil {
		t.Fatal(err)
	}

	encodedCatalog, _, _ := parent.Read(worldchunk.CatalogEntryName, 0, worldchunk.MaxEncodedSize)

	catalog, err := worldchunk.DecodeCatalog(encodedCatalog, &document)
	if err != nil {
		t.Fatal(err)
	}

	return document, catalog
}
func readySelectedChild(t *testing.T, chunk ChunkArtifact, descriptor worldchunk.Chunk,
	document *world.Document, enabled bool,
) []worldchunk.ReadySurface {
	t.Helper()

	child, err := level.DecodePassiveModule(chunk.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	encoded, _, found := child.Read(worldchunk.ReadyEntryName, 0, worldchunk.MaxReadyBytes)
	if found != enabled {
		t.Fatal("selected SDK did not control ready entry")
	}

	_, _, semanticFound := child.Read(worldchunk.MeshEntryName, 0, worldchunk.MaxEncodedSize)
	if semanticFound == enabled || len(child.Entries) != 1 {
		t.Fatal("selected SDK did not choose exactly one shipped geometry representation")
	}

	if !enabled {
		return nil
	}

	geometry, err := rebuildCachedWorldChunkGeometry(child, descriptor, document)
	if err != nil {
		t.Fatal(err)
	}

	masked := make([]bool, len(world.MaterialIDs(document)))
	masked[len(masked)-1] = true

	ready, err := worldchunk.DecodeReady(encoded, &geometry.mesh, geometry.encoded, document, nil, worldchunk.ReadyOptions{Masked: masked})
	if err != nil {
		t.Fatal(err)
	}

	for _, batch := range ready.Batches {
		if !batch.Masked {
			t.Fatal("material coverage was not carried to ready batches")
		}
	}

	return ready.Surfaces
}
func TestReadyChunkBakedCacheValidationAndCapabilityIdentity(t *testing.T) {
	t.Parallel()

	for _, lighting := range []bool{false, true} {
		t.Run(map[bool]string{false: "mesh", true: "baked"}[lighting], func(t *testing.T) {
			t.Parallel()
			fixture := chunkCacheSemanticFixture(t, lighting, true)

			options := &worldchunk.ReadyOptions{}
			if _, found := readWorldChunkCache(t.Context(), fixture.root, fixture.directory, fixture.key,
				&fixture.document, fixture.definition, options); !found {
				t.Fatal("valid ready cache rejected")
			}

			if _, found := readWorldChunkCache(t.Context(), fixture.root, fixture.directory, fixture.key,
				&fixture.document, fixture.definition); found {
				t.Fatal("legacy selection accepted ready cache")
			}

			for index, child := range fixture.children {
				validateReadyBuiltChild(t, child, fixture.catalog.Chunks[index], &fixture.document, lighting)
			}
		})
	}
}

func validateReadyBuiltChild(t *testing.T, child level.Envelope, descriptor worldchunk.Chunk,
	document *world.Document, lighting bool,
) {
	t.Helper()

	metadata, err := level.DecodeMetadata(child.Metadata)
	if err != nil || !chunkMetadataMatches(metadata, worldchunk.ReadyMetadataKey, worldchunk.ReadySchema) {
		t.Fatal("child ready declaration missing", err)
	}

	entries, err := readWorldChunkCacheEntries(child)
	if err != nil {
		t.Fatal(err)
	}

	geometry, err := rebuildCachedWorldChunkGeometry(child, descriptor, document)
	if err != nil {
		t.Fatal(err)
	}

	if (geometry.layout != nil) != lighting || entries[worldchunk.MeshEntryName] != nil {
		t.Fatal("ready-only child shipped semantic geometry or wrong lightmap presence")
	}

	if _, err := worldchunk.DecodeReady(entries[worldchunk.ReadyEntryName], &geometry.mesh, geometry.encoded,
		document, geometry.layout, worldchunk.ReadyOptions{}); err != nil {
		t.Fatal("child ready validation:", err)
	}
}
