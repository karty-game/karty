package levelbuild

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty/internal/testfixture"
	"github.com/pelletier/go-toml/v2"
)

func TestChunkAreasPreserveDescendantsAndQualifiedRooms(t *testing.T) {
	t.Parallel()

	document := world.Document{Sectors: []world.Sector{
		{ID: "hall#0", SourceRoom: "hall"},
		{ID: "gallery/hall#0", Instance: "gallery", SourceRoom: "hall"},
		{ID: "court/bay#0", Instance: "court", SourceRoom: "bay"},
		{ID: "court/inner/bay#0", Instance: "court/inner", SourceRoom: "bay"},
		{ID: "courtyard/bay#0", Instance: "courtyard", SourceRoom: "bay"},
		{ID: "other#0", SourceRoom: "other"},
	}}
	areas := []chunkAreaSettings{
		{Name: "courtyard", Instances: []string{"court"}},
		{Name: "gallery", Rooms: []string{"gallery/hall"}},
	}

	groups, err := resolveChunkAreas(areas, &document)
	if err != nil || !slices.Equal(groups, []int{0, 2, 1, 1, 0, 0}) {
		t.Fatalf("authoring identities or unselected default group lost: %v, %v", groups, err)
	}
}

func TestChunkAreasRejectCompleteInvalidSelection(t *testing.T) {
	t.Parallel()

	document := world.Document{Sectors: []world.Sector{
		{ID: "court/bay#0", Instance: "court", SourceRoom: "bay"},
		{ID: "hall#0", SourceRoom: "hall"},
	}}
	valid := chunkAreaSettings{Name: "court", Instances: []string{"court"}}
	cases := map[string][]chunkAreaSettings{
		"empty name":       {{Instances: []string{"court"}}},
		"empty area":       {{Name: "court"}},
		"empty selector":   {{Name: "court", Instances: []string{""}}},
		"unknown instance": {{Name: "court", Instances: []string{"missing"}}},
		"unknown room":     {{Name: "court", Rooms: []string{"bay"}}},
		"duplicate names":  {valid, {Name: "court", Rooms: []string{"hall"}}},
		"overlap":          {valid, {Name: "other", Rooms: []string{"court/bay"}}},
		"invalid last":     {valid, {Name: "hall", Rooms: []string{"hall", "missing"}}},
		"too many":         make([]chunkAreaSettings, world.MaxSectors),
	}

	for name, areas := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			groups, err := resolveChunkAreas(areas, &document)
			if groups != nil || !errors.Is(err, ErrManifest) {
				t.Fatalf("invalid complete selection escaped: %v, %v", groups, err)
			}

			if name == "too many" && !strings.Contains(err.Error(), "too many chunk areas") {
				t.Fatalf("oversized batch was not rejected before selector processing: %v", err)
			}
		})
	}
}

func TestRomanCourtFixtureChunkAreasKeepWholeCourtyard(t *testing.T) {
	t.Parallel()

	sample := filepath.Join("..", "..", "testdata", "roman-court")
	root := testfixture.RomanCourtGeometry(t, sample)
	path := filepath.Join(root, "levels", "showcase", "level.toml")

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var definition manifest
	if err := toml.Unmarshal(contents, &definition); err != nil {
		t.Fatal(err)
	}

	sampleContents, err := os.ReadFile(filepath.Join(sample, "levels", "showcase", "level.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var sampleDefinition manifest
	if err := toml.Unmarshal(sampleContents, &sampleDefinition); err != nil {
		t.Fatal(err)
	}

	definition.Chunks = sampleDefinition.Chunks

	contents, err = toml.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}

	// #nosec G703 -- isolated geometry fixture beneath t.TempDir.
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", directLightmapSDK(t))
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("sample authored areas: %v", err)
	}

	if len(artifacts[0].Chunks) != 6 {
		t.Fatalf("sample authored areas produced %d chunks, want six", len(artifacts[0].Chunks))
	}

	envelope := executeLightmapEnvelope(t, artifacts[0].Bytes)
	worldBytes, _, _ := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(worldBytes)
	if err != nil {
		t.Fatal(err)
	}

	catalogBytes, _, _ := envelope.Read(worldchunk.CatalogEntryName, 0, worldchunk.MaxEncodedSize)

	catalog, err := worldchunk.DecodeCatalog(catalogBytes, &document)
	if err != nil {
		t.Fatal(err)
	}

	counts := make([]int, len(catalog.Chunks))
	courtyardChunk := -1

	for _, chunk := range catalog.Chunks {
		counts[chunk.ID] = len(chunk.Sectors)
	}

	for sector, authored := range document.Sectors {
		if authored.Instance != "roman-court" {
			continue
		}

		if courtyardChunk < 0 {
			courtyardChunk = catalog.SectorChunks[sector]
		}

		if courtyardChunk != catalog.SectorChunks[sector] {
			t.Fatal("courtyard split across chunks")
		}
	}

	slices.Sort(counts)

	if !slices.Equal(counts, []int{1, 1, 1, 2, 3, 16}) || courtyardChunk < 0 || len(catalog.Chunks[courtyardChunk].Solids) != 271 {
		t.Fatalf("authored areas lost courtyard sectors or columns: counts=%v courtyard=%d", counts, courtyardChunk)
	}

	checkRomanCourtChunkLightmapPages(t, sampleDefinition, document)
}

func checkRomanCourtChunkLightmapPages(t *testing.T, definition manifest, document world.Document) {
	t.Helper()

	options, err := chunkOptions(definition)
	if err != nil {
		t.Fatal(err)
	}

	options.SectorGroups, err = resolveChunkAreas(definition.Chunks.Areas, &document)
	if err != nil {
		t.Fatal(err)
	}

	// Geometry fixtures omit lighting; recipe selection does not affect packing.
	options.Lightmap.Lights = nil

	plan, err := worldchunk.Plan(document, options)
	if err != nil || len(plan.Chunks) != 6 {
		t.Fatalf("sample lightmap budgets split authored areas: %v", err)
	}

	for _, chunk := range plan.Chunks {
		page := 512
		if len(chunk.Sectors) == 16 {
			page = 1024
		}

		if chunk.Layout.TexelsPerUnit != 16 || len(chunk.Layout.Pages) != 1 ||
			chunk.Layout.Pages[0].Width != page || chunk.Layout.Pages[0].Height != page ||
			chunk.LightmapBytes != uint64(page*page*12) {
			t.Fatalf("chunk %d density/page reservation: %+v bytes=%d", chunk.ID, chunk.Layout.Pages, chunk.LightmapBytes)
		}
	}
}
