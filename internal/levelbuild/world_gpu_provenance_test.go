package levelbuild

import (
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func TestGPUCellOmitsCPUProvenanceAndPhysicalChunkRetainsLighting(t *testing.T) {
	t.Parallel()
	document, materials, cache := incrementalBakeFixture(t)
	definition := manifest{Lightmap: &lightmapBuildSettings{Enabled: true, Fixed: true, Offline: true,
		Density: new(float64(1)), PageSize: new(int(512)), BakeSamples: new(int(1)), BakeBounces: new(int(0)), BakeDenoise: new("off")}}

	lighting, bake, err := gpuLightingOptions(definition, &document, nil)
	if err != nil {
		t.Fatal(err)
	}

	layout, err := worldlightmap.CompileReceivers(document, lighting, []int{0}, nil)
	if err != nil {
		t.Fatal(err)
	}

	build := gpuWorldBuild{assets: &assetBuild{}, document: document, definition: definition,
		lighting: lighting, bake: bake, bakeMaterials: materials, cache: cache}

	cell, err := build.cellLighting(t.Context(), gpuCellPlan{chunk: worldchunk.PlannedChunk{Layout: layout}})
	if err != nil {
		t.Fatal(err)
	}

	if len(cell.entries) != 0 || len(cell.pixels) == 0 || cell.rangeValue <= 0 {
		t.Fatal("fixed GPU render lighting retained CPU provenance or lost GPU pixels/range")
	}

	metadata, err := level.EncodeMetadata(level.Metadata{})
	if err != nil {
		t.Fatal(err)
	}

	module, err := encodeLevelModule("render-cell", metadata, cell.entries)
	if err != nil {
		t.Fatal(err)
	}

	envelope := decodePassiveEnvelope(t, module)
	for _, name := range []string{worldlightmap.EntryName, worldlightmap.FixedEntryName} {
		if _, _, found := envelope.Read(name, 0, level.MaxEntrySize); found {
			t.Fatal("render cell retained", name)
		}
	}

	physical, err := worldchunk.Plan(document, worldchunk.Options{NoLightmap: true, MaxSectors: 1})
	if err != nil {
		t.Fatal(err)
	}

	artifact, _, err := encodeGPUPhysicalChunk(
		t.Context(),
		document,
		"physical",
		physical.Chunks[0],
		materials,
		lighting,
		bake,
		cache,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}

	envelope = decodePassiveEnvelope(t, artifact.Bytes)
	for _, name := range []string{worldchunk.CollisionEntryName, worldlightmap.EntryName, worldlightmap.FixedEntryName} {
		if _, _, found := envelope.Read(name, 0, level.MaxEntrySize); !found {
			t.Fatal("physical chunk lost", name)
		}
	}
}
