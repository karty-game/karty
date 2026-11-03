package levelbuild

import (
	"bytes"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
)

func TestGPUCellV2ProducerUsesCompactSDKBytes(t *testing.T) {
	t.Parallel()

	document, _, _ := incrementalBakeFixture(t)

	mesh, err := worldchunk.CompileMesh(document, []int{0}, nil, worldlightmap.Layout{})
	if err != nil {
		t.Fatal(err)
	}

	materials, err := worldmaterial.NewLayoutV2(world.MaterialIDs(&document))
	if err != nil {
		t.Fatal(err)
	}

	build := gpuWorldBuild{document: document, materials: materials}

	data, err := build.compileCell(gpuCellPlan{sectors: []uint32{0}, chunk: worldchunk.PlannedChunk{Mesh: mesh}}, 1)
	if err != nil {
		t.Fatal(err)
	}

	if worldgpuchunk.VertexStride != 96 || len(data.Vertices) == 0 || len(data.Vertices)%96 != 0 {
		t.Fatal("CLI producer did not emit compact96 vertices", len(data.Vertices))
	}

	payload := worldgpuchunk.Payload{
		CellID: 0, Bounds: gpuDrawBounds(data.Draws), MaterialLayoutSHA256: [32]byte{1},
		RGBMRange: 1, GPUData: data,
	}

	encoded, err := worldgpuchunk.EncodePayload(payload)
	if err != nil {
		t.Fatal(err)
	}

	if string(encoded[:4]) != "KGP2" {
		t.Fatal("CLI producer retained old payload wire")
	}

	decoded, err := worldgpuchunk.DecodePayload(encoded)
	if err != nil || !bytes.Equal(data.Vertices, decoded.Vertices) {
		t.Fatalf("published compact GPU bytes changed: %v", err)
	}
}

func TestGPUCellV2RequiresExactSelectedSDKFeature(t *testing.T) {
	t.Parallel()

	if worldgpuchunk.Feature != "world/gpu-chunks@2" || worldgpuchunk.Schema != "karty.world-gpu-chunks@2" ||
		worldgpuchunk.ABI != "karty.webgpu-cell@2" {
		t.Fatal("CLI dependency did not select the versioned compact contract")
	}

	if worldGPUChunksEnabled(nil) {
		t.Fatal("missing SDK enabled GPU cells")
	}

	for _, scenario := range []struct {
		feature string
		want    bool
	}{
		{feature: "world/gpu-chunks@1"},
		{feature: worldgpuchunk.Feature, want: true},
	} {
		assets := &assetBuild{}
		assets.manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.Capability(scenario.feature)}

		if worldGPUChunksEnabled(assets) != scenario.want {
			t.Fatalf("feature %s selected wrong profile", scenario.feature)
		}
	}
}

func TestGPUCellV2OfflineCleanupDoesNotRequireRetiredReadyCapability(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	document.StaticSolids = &world.StaticSolids{Version: world.StaticSolidsVersion, Items: []world.Solid{{
		ID: "floor-occluder", Footprint: []world.Vec2{{X: .85, Y: .1}, {X: 1.15, Y: .1}, {X: 1.15, Y: .4}, {X: .85, Y: .4}},
		Bottom: world.Plane{}, Top: world.Plane{C: 1}, SideMaterial: 1, TopMaterial: 1, BottomMaterial: 1, Collision: true,
	}}}
	assets := &assetBuild{}
	assets.manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.Capability(worldgpuchunk.Feature)}

	prepared, err := buildWorldMaterials(t.Context(), assets, document, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if prepared.ready == nil || !prepared.ready.CleanupStatic {
		t.Fatal("GPU offline cleanup depended on retired renderer capability")
	}

	mesh, err := worldchunk.CompileMesh(document, []int{0}, []int{0}, worldlightmap.Layout{})
	if err != nil {
		t.Fatal(err)
	}

	layout, err := worldmaterial.NewLayoutV2(world.MaterialIDs(&document))
	if err != nil {
		t.Fatal(err)
	}

	build := gpuWorldBuild{document: document, materials: layout, ready: *prepared.ready, solidSectors: [][]uint32{{0}}}
	cell := gpuCellPlan{sectors: []uint32{0}, chunk: worldchunk.PlannedChunk{Mesh: mesh}}

	cleaned, err := build.compileCell(cell, 1)
	if err != nil {
		t.Fatal(err)
	}

	build.ready.CleanupStatic = false

	original, err := build.compileCell(cell, 1)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(cleaned.Indices, original.Indices) && bytes.Equal(cleaned.Vertices, original.Vertices) {
		t.Fatal("covered-floor cleanup did not change final GPU geometry")
	}
}
