package levelbuild

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	"github.com/pelletier/go-toml/v2"
)

type chunkCacheValidationFixture struct {
	root       string
	directory  string
	key        string
	definition manifest
	document   world.Document
	catalog    worldchunk.Catalog
	container  level.Envelope
	children   []level.Envelope
}

func chunkCacheSemanticFixture(t *testing.T, lighting bool) chunkCacheValidationFixture {
	t.Helper()
	root := lightingLevelFixture(t, "version: 4\n"+chunkRoomsYAML+directLightmapYAML)
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.Capability(worldchunk.Feature))

	if lighting {
		enableFixtureLightmap(
			t,
			root,
			"enabled = true\noffline = true\nlights = ['red','blue']\npage_size = 512\ndensity = 1\nbake_samples = 1\nbake_bounces = 0\nbake_denoise = 'off'\n",
		)
	}

	enableFixtureChunks(t, root, "max_sectors = 1\n")

	built, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}

	fixture := chunkCacheValidationFixture{root: root, directory: filepath.Join(root, "levels", "lighting")}

	definition, err := os.ReadFile(filepath.Join(fixture.directory, "level.toml"))
	if err != nil {
		t.Fatal(err)
	}

	if err := toml.Unmarshal(definition, &fixture.definition); err != nil {
		t.Fatal(err)
	}

	parent, err := level.DecodePassiveModule(built[0].Bytes)
	if err != nil {
		t.Fatal(err)
	}

	encoded, _, _ := parent.Read(world.EntryName, 0, world.MaxEncodedSize)

	fixture.document, err = world.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, automaticBakeDirectory, "lighting")

	files, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		if suffix, found := strings.CutPrefix(file.Name(), "chunks-"); found {
			fixture.key = strings.TrimSuffix(suffix, ".bin")

			encoded, err = os.ReadFile(filepath.Join(path, file.Name()))
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	fixture.container, err = level.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	encoded, _, _ = fixture.container.Read(worldchunk.CatalogEntryName, 0, worldchunk.MaxEncodedSize)

	fixture.catalog, err = worldchunk.DecodeCatalog(encoded, &fixture.document)
	if err != nil {
		t.Fatal(err)
	}

	for _, chunk := range built[0].Chunks {
		child, err := level.DecodePassiveModule(chunk.Bytes)
		if err != nil {
			t.Fatal(err)
		}

		fixture.children = append(fixture.children, child)
	}

	return fixture
}

func chunkCacheSourceEntries(t *testing.T, envelope level.Envelope) []level.SourceEntry {
	t.Helper()

	entries := make([]level.SourceEntry, 0, len(envelope.Entries))
	for _, entry := range envelope.Entries {
		data, _, found := envelope.Read(entry.Name, 0, entry.Length)
		if !found {
			t.Fatal("fixture entry missing")
		}

		entries = append(entries, level.SourceEntry{Name: entry.Name, Kind: entry.Kind, Data: data})
	}

	return entries
}

func TestChunkCacheRejectsHashValidSemanticCorruption(t *testing.T) {
	t.Parallel()

	for _, lighting := range []bool{false, true} {
		name := "mesh"
		if lighting {
			name = "prebaked"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fixture := chunkCacheSemanticFixture(t, lighting)
			if _, found := readWorldChunkCache(
				t.Context(),
				fixture.root,
				fixture.directory,
				fixture.key,
				&fixture.document,
				fixture.definition,
			); !found {
				t.Fatal("valid fixture cache rejected")
			}

			mutations := map[string]func(level.Metadata, *[]level.SourceEntry, *worldchunk.Chunk){
				"wrong chunk identity": func(metadata level.Metadata, _ *[]level.SourceEntry, _ *worldchunk.Chunk) {
					metadata["chunkID"] = level.UintValue(999)
				},
				"null chunk identity": func(metadata level.Metadata, _ *[]level.SourceEntry, _ *worldchunk.Chunk) {
					metadata["chunkID"] = level.NullValue()
				},
				"wrong geometry identity": func(metadata level.Metadata, _ *[]level.SourceEntry, _ *worldchunk.Chunk) {
					metadata["geometrySha256"] = level.StringValue(strings.Repeat("a", 64))
				},
				"broken mesh": func(_ level.Metadata, entries *[]level.SourceEntry, _ *worldchunk.Chunk) {
					replaceChunkCacheEntry(*entries, worldchunk.MeshEntryName, []byte("broken"))
				},
				"wrong mesh ownership": func(_ level.Metadata, entries *[]level.SourceEntry, _ *worldchunk.Chunk) {
					other, _, _ := fixture.children[1].Read(worldchunk.MeshEntryName, 0, worldchunk.MaxEncodedSize)
					replaceChunkCacheEntry(*entries, worldchunk.MeshEntryName, other)
				},
				"wrong bounds": func(_ level.Metadata, _ *[]level.SourceEntry, descriptor *worldchunk.Chunk) {
					descriptor.Bounds[0] -= 1
				},
				"wrong decoded size": func(_ level.Metadata, _ *[]level.SourceEntry, descriptor *worldchunk.Chunk) {
					descriptor.MeshBytes += 484
				},
				"extra owned entry": func(_ level.Metadata, entries *[]level.SourceEntry, _ *worldchunk.Chunk) {
					*entries = append(
						*entries,
						level.SourceEntry{Name: "@world/chunks/extra", Kind: level.EntryData, Data: []byte("unexpected")},
					)
				},
			}
			if lighting {
				mutations["partial prebake"] = func(_ level.Metadata, entries *[]level.SourceEntry, _ *worldchunk.Chunk) {
					*entries = slices.DeleteFunc(
						*entries,
						func(entry level.SourceEntry) bool { return entry.Name == worldlightmap.PrebakeImageEntryName },
					)
				}
				mutations["invalid prebake image"] = func(_ level.Metadata, entries *[]level.SourceEntry, _ *worldchunk.Chunk) {
					replaceChunkCacheEntry(*entries, worldlightmap.PrebakeImageEntryName, []byte("invalid QOI"))
				}
				mutations["wrong receiver layout"] = func(_ level.Metadata, entries *[]level.SourceEntry, _ *worldchunk.Chunk) {
					other, _, _ := fixture.children[1].Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)
					replaceChunkCacheEntry(*entries, worldlightmap.EntryName, other)
				}
				mutations["wrong decoded lightmap size"] = func(_ level.Metadata, _ *[]level.SourceEntry, descriptor *worldchunk.Chunk) {
					descriptor.LightmapBytes = 1024 * 1024 * 12
				}
			}

			for name, mutate := range mutations {
				t.Run(name, func(t *testing.T) {
					metadata, err := level.DecodeMetadata(fixture.children[0].Metadata)
					if err != nil {
						t.Fatal(err)
					}

					entries := chunkCacheSourceEntries(t, fixture.children[0])
					catalog := fixture.catalog
					catalog.Chunks = slices.Clone(catalog.Chunks)
					mutate(metadata, &entries, &catalog.Chunks[0])
					writeCorruptSemanticChunkCache(t, fixture, metadata, entries, catalog)

					if _, found := readWorldChunkCache(
						t.Context(),
						fixture.root,
						fixture.directory,
						fixture.key,
						&fixture.document,
						fixture.definition,
					); found {
						t.Fatal("hash/size/passive-valid semantic corruption accepted")
					}
				})
			}
		})
	}
}

func replaceChunkCacheEntry(entries []level.SourceEntry, name string, data []byte) {
	for index := range entries {
		if entries[index].Name == name {
			entries[index].Data = data
		}
	}
}

func TestChunkCacheRejectsCatalogBakeClassificationDifferentFromChildren(t *testing.T) {
	t.Parallel()
	fixture := chunkCacheSemanticFixture(t, true)

	metadata, err := level.DecodeMetadata(fixture.children[0].Metadata)
	if err != nil {
		t.Fatal(err)
	}

	entries := chunkCacheSourceEntries(t, fixture.children[0])
	catalog := fixture.catalog

	catalog.Chunks = slices.Clone(catalog.Chunks)
	if len(catalog.BakedLights) != 2 {
		t.Fatal("fixture lost its completed source union")
	}
	// This catalog remains structurally valid and each child's hash still
	// matches, but omitting one baked source would admit it as a live light.
	catalog.BakedLights = slices.Clone(catalog.BakedLights[:1])
	writeCorruptSemanticChunkCache(t, fixture, metadata, entries, catalog)

	if _, found := readWorldChunkCache(
		t.Context(),
		fixture.root,
		fixture.directory,
		fixture.key,
		&fixture.document,
		fixture.definition,
	); found {
		t.Fatal("catalog classification did not bind completed child source sets")
	}
}

func writeCorruptSemanticChunkCache(t *testing.T, fixture chunkCacheValidationFixture, metadata level.Metadata,
	entries []level.SourceEntry, catalog worldchunk.Catalog,
) {
	t.Helper()

	encodedMetadata, err := level.EncodeMetadata(metadata)
	if err != nil {
		t.Fatal(err)
	}

	envelope, err := level.Encode(encodedMetadata, entries)
	if err != nil {
		t.Fatal(err)
	}

	module, err := level.WrapModule(envelope)
	if err != nil {
		t.Fatal(err)
	}

	if err := level.ValidatePassiveModule(module); err != nil {
		t.Fatal(err)
	}

	digest := sha256.Sum256(module)
	catalog.Chunks[0].ContentSHA256 = hex.EncodeToString(digest[:])
	catalog.Chunks[0].Size = uint64(len(module))

	encodedCatalog, err := worldchunk.EncodeCatalog(catalog, &fixture.document)
	if err != nil {
		t.Fatal(err)
	}

	cacheEntries := chunkCacheSourceEntries(t, fixture.container)
	for index := range cacheEntries {
		if cacheEntries[index].Name == worldchunk.CatalogEntryName {
			cacheEntries[index].Data = encodedCatalog
		}
	}

	encoded, err := level.Encode(fixture.container.Metadata, cacheEntries)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(fixture.root, automaticBakeDirectory, "lighting")
	if err := atomicBakeWrite(path, catalog.Chunks[0].ContentSHA256+".chunk.kld", module); err != nil {
		t.Fatal(err)
	}

	if err := atomicBakeWrite(path, "chunks-"+fixture.key+".bin", encoded); err != nil {
		t.Fatal(err)
	}
}

func TestChunkRootRejectsAuthoredWholeWorldLightingMarkers(t *testing.T) {
	t.Parallel()

	for _, marker := range []string{worldlightmap.MetadataKey, worldlightmap.PrebakeMetadataKey} {
		for _, value := range []string{`"owned"`, "null"} {
			t.Run(marker+"/"+value, func(t *testing.T) {
				t.Parallel()

				definition := manifest{Chunks: &chunkBuildSettings{Enabled: true}}

				for _, cached := range []bool{false, true} {
					entries := []level.SourceEntry(nil)
					if cached {
						entries = []level.SourceEntry{{Name: worldchunk.CatalogEntryName, Kind: level.EntryData, Data: []byte("cached")}}
					}

					_, err := applyWorldChunks(
						worldMaterialBuild{metadata: binaryMetadata(t, level.Metadata{marker: func() level.Value {
							if value == "null" {
								return level.NullValue()
							}

							return level.StringValue("owned")
						}()})},
						entries,
						nil,
						definition,
					)
					if !errors.Is(err, ErrManifest) {
						t.Fatalf("cached=%v whole-world marker accepted: %v", cached, err)
					}
				}
			})
		}
	}

	for _, name := range []string{worldchunk.CatalogEntryName, "@world/chunks/unknown", worldlightmap.EntryName, "@world/lightmaps/unknown"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := applyWorldChunks(
				worldMaterialBuild{
					metadata: []byte(`{}`),
					data:     []level.SourceEntry{{Name: name, Kind: level.EntryData, Data: []byte("authored")}},
				},
				nil,
				nil,
				manifest{},
			)
			if !errors.Is(err, ErrSource) {
				t.Fatalf("authored reserved namespace %q accepted: %v", name, err)
			}
		})
	}
}
