package levelbuild

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

//nolint:paralleltest,tparallel // Each missing-resource case rewrites one shared cache publication.
func TestReadyChunkCacheRejectsPartialRequiredResourceUnit(t *testing.T) {
	t.Parallel()

	fixture := chunkCacheSemanticFixture(t, true, true)
	descriptor := fixture.catalog.Chunks[0]

	module, err := os.ReadFile(filepath.Join(fixture.root, automaticBakeDirectory, "lighting", descriptor.ContentSHA256+".chunk.kld"))
	if err != nil {
		t.Fatal(err)
	}

	digest := sha256.Sum256(module)
	if hex.EncodeToString(digest[:]) != descriptor.ContentSHA256 || uint64(len(module)) != descriptor.Size {
		t.Fatal("catalog does not bind the complete geometry and lighting module")
	}

	if _, found := readWorldChunkCache(t.Context(), fixture.root, fixture.directory, fixture.key,
		&fixture.document, fixture.definition, &worldchunk.ReadyOptions{}); !found {
		t.Fatal("complete geometry and default RNM unit was not accepted")
	}

	child := fixture.children[0]

	metadata, err := level.DecodeMetadata(child.Metadata)
	if err != nil {
		t.Fatal(err)
	}

	for _, missing := range []string{worldchunk.ReadyEntryName, worldlightmap.EntryName,
		worldlightmap.PrebakeEntryName, worldlightmap.PrebakeImageEntryName} {
		t.Run(missing, func(t *testing.T) {
			entries := chunkCacheSourceEntries(t, child)

			entries = slices.DeleteFunc(entries, func(entry level.SourceEntry) bool { return entry.Name == missing })
			if len(entries) != 3 {
				t.Fatal("required chunk resource was not present in the complete unit")
			}

			catalog := fixture.catalog
			catalog.Chunks = slices.Clone(catalog.Chunks)
			// Recompute the child hash and size to exercise complete-resource
			// admission rather than stopping at an integrity mismatch.
			writeCorruptSemanticChunkCache(t, fixture, metadata, entries, catalog)

			if _, found := readWorldChunkCache(t.Context(), fixture.root, fixture.directory, fixture.key,
				&fixture.document, fixture.definition, &worldchunk.ReadyOptions{}); found {
				t.Fatal("partial geometry/default lighting unit reached publication")
			}
		})
	}
}
