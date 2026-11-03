package levelbuild

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
	"github.com/pelletier/go-toml/v2"
)

// This compiles maintained authoring inputs only: no texture processing, rays,
// bake caches, child packaging or GPU device is needed for the ownership proof.
func TestGPUWholeRoomHangarPreservesCompleteSourceGeometry(t *testing.T) {
	t.Parallel()
	document, definition := gpuHangarRoomTestSource(t)

	build := gpuRoomTestBuildInputs(t, document, definition)
	settings := gpuWorldSettings(definition)

	cells, err := planGPUCells(t.Context(), &document, build.physical, nil, settings.CellSize, build.lighting, settings.LiquidEdgeStep,
		gpuCellPolicy{roomGroups: settings.RoomGroups})
	if err != nil {
		t.Fatal(err)
	}

	if len(cells) != 9 || len(build.physical.Chunks) != 9 {
		t.Fatal("Hangar room groups fragmented or merged", len(cells), len(build.physical.Chunks))
	}

	assignments, _, err := gpuRoomSectorGroups(&document, settings.RoomGroups)
	if err != nil {
		t.Fatal(err)
	}

	checkGPUHangarRoomSourceCoverage(t, document, cells, build.physical, assignments)
}

func gpuRoomTestBuildInputs(t *testing.T, document world.Document, definition manifest) gpuWorldBuild {
	t.Helper()

	layout, err := worldmaterial.NewLayout(world.MaterialIDs(&document))
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := worldmaterial.EncodeLayout(layout)
	if err != nil {
		t.Fatal(err)
	}

	assets := &assetBuild{}
	assets.manifest.Assets.Capabilities.Runtime = append(assets.manifest.Assets.Capabilities.Runtime, worldmaterialgpu.Feature)
	build := gpuWorldBuild{document: document, definition: definition, assets: assets}

	_, err = build.prepareInputs(
		&worldMaterialBuild{data: []level.SourceEntry{{Name: worldmaterial.LayoutEntry, Kind: level.EntryData, Data: encoded}}},
	)
	if err != nil {
		t.Fatal(err)
	}

	return build
}

func TestGPUWholeRoomPhysicalGroupingMatchesAuthoredLoadingRooms(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	document.Sectors[0].SourceRoom = "joined-room"
	document.Sectors[1].SourceRoom = "joined-room"
	document.Sectors[0].Walls[1].SourceEdge = ""
	document.Sectors[1].Walls[3].SourceEdge = ""
	maxSectors := 1
	definition := manifest{Chunks: &chunkBuildSettings{Enabled: true, MaxSectors: &maxSectors, GPU: &gpuChunkSettings{CellSize: 2}}}

	build := gpuRoomTestBuildInputs(t, document, definition)
	if len(build.physical.Chunks) != 2 || build.physical.SectorChunks[0] != build.physical.SectorChunks[1] ||
		build.physical.SectorChunks[0] == build.physical.SectorChunks[2] {
		t.Fatal("physical transport split convex room or crossed semantic boundary", build.physical.SectorChunks)
	}

	definition.Chunks.GPU.RoomGroups = [][]string{{"joined-room", "far-room"}}

	build = gpuRoomTestBuildInputs(t, document, definition)
	if len(build.physical.Chunks) != 1 {
		t.Fatal("explicit loading room fragmented by old physical sector limit")
	}
}

func gpuHangarRoomTestSource(t *testing.T) (world.Document, manifest) {
	t.Helper()

	directory := filepath.Join("..", "..", "samples", "hangar", "levels", "hangar")

	encoded, err := os.ReadFile(filepath.Join(directory, "level.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var definition manifest
	if err := toml.Unmarshal(encoded, &definition); err != nil {
		t.Fatal(err)
	}

	expanded, err := source.Load(directory, definition.World.Source)
	if err != nil {
		t.Fatal(err)
	}

	materials := make(map[string]uint32, len(definition.Textures))
	for index, texture := range definition.Textures {
		materials[texture.Name] = uint32(index + 1)
	}

	document, err := geometry.Compile(expanded, materials)
	if err != nil {
		t.Fatal(err)
	}

	if document.StaticSolids == nil || len(document.StaticSolids.Items) != 413 {
		t.Fatal("Hangar source-solid fixture changed")
	}

	return document, definition
}

func checkGPUHangarRoomSourceCoverage(t *testing.T, document world.Document, cells []gpuCellPlan,
	physical worldchunk.PlanResult, assignments []int,
) {
	t.Helper()

	sourceSectors := make([]int, len(document.Sectors))
	sourceSolids := make([]int, len(document.StaticSolids.Items))

	for index := range sourceSectors {
		sourceSectors[index] = index
	}

	for index := range sourceSolids {
		sourceSolids[index] = index
	}

	complete, err := worldchunk.CompileMesh(document, sourceSectors, sourceSolids, worldlightmap.Layout{})
	if err != nil {
		t.Fatal(err)
	}

	var actual float64

	sectorUses := make([]int, len(sourceSectors))
	solidUses := make([]int, len(sourceSolids))

	for _, cell := range cells {
		actual += gpuTestMeshArea(cell.chunk.Mesh)

		group := assignments[cell.sectors[0]]
		for _, id := range cell.sectors {
			sectorUses[id]++
			if assignments[id] != group {
				t.Fatal("render chunk crossed semantic room group")
			}
		}

		for _, id := range cell.chunk.Solids {
			solidUses[id]++
		}
	}

	for _, uses := range sectorUses {
		if uses != 1 {
			t.Fatal("source sector omitted or duplicated")
		}
	}

	for id, uses := range solidUses {
		if uses != 1 {
			t.Fatal("source solid omitted or duplicated", id, uses)
		}
	}

	for _, chunk := range physical.Chunks {
		group := assignments[chunk.Sectors[0]]
		for _, id := range chunk.Sectors {
			if assignments[id] != group {
				t.Fatal("physical chunk crossed semantic room group")
			}
		}
	}

	expected := gpuTestMeshArea(complete)
	if math.Abs(actual-expected) > 1e-5 {
		t.Fatal("whole-room chunk meshes changed full source surface area", actual, expected)
	}
}
