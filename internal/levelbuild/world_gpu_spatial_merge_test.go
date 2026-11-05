package levelbuild

import (
	"math"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func gpuSpatialMergeFixture(
	t *testing.T,
	speeds ...float64,
) (world.Document, []worldchunk.PlannedChunk, spatialLoadingMergePolicy, worldchunk.Options) {
	t.Helper()
	document, _, _ := incrementalBakeFixture(t)
	document.StaticSolids = &world.StaticSolids{
		Version: world.StaticSolidsVersion,
		Items: []world.Solid{
			{
				ID:             "water",
				Footprint:      []world.Vec2{{X: 0, Y: 0}, {X: 4, Y: 0}, {X: 4, Y: 2}, {X: 0, Y: 2}},
				Bottom:         world.Plane{C: .1},
				Top:            world.Plane{C: .25},
				TopMaterial:    2,
				BottomMaterial: 2,
				SideMaterial:   2,
			},
		},
	}

	document.Animations = &world.Animations{
		Version:   world.AnimationVersion,
		Presets:   []world.AnimationPreset{{Name: "waves", Kind: "liquid", SurfaceAmplitude: .1, Frequency: 1}},
		Materials: []world.MaterialAnimation{{Material: 2, Preset: "waves"}},
	}
	if len(speeds) > 0 {
		document.Animations.Presets[0].Speed = speeds[0]
	}

	surfaces, err := worldlightmap.Surfaces(&document)
	if err != nil {
		t.Fatal(err)
	}

	var (
		space  gpuCellSpace
		policy spatialLoadingMergePolicy
	)

	for _, surface := range surfaces {
		if surface.Binding.Kind == "sector-floor" && surface.Binding.Index < 2 ||
			surface.Binding.Kind != "sector-floor" && surface.Binding.Kind != "sector-wall" && surface.Binding.Kind != "sector-ceiling" {
			space.surfaces = append(space.surfaces, gpuCellSurface{surface: surface})
			binding := surface.Binding
			binding.Chart = -1
			policy.sourceBindings = append(policy.sourceBindings, binding)
		}
	}

	lighting := worldlightmap.Options{PageSize: 512, MaxPages: 1, MaxTexels: 512 * 512, TexelsPerUnit: 1, Padding: 2, AllStatic: true}

	chunks := make([]worldchunk.PlannedChunk, 0, 2)

	for index, bounds := range [][4]float64{{0, 0, 2, 2}, {2, 0, 4, 2}} {
		selection := worldlightmap.SurfaceSelection{Clip: &bounds, LiquidEdgeStep: 1}
		for _, surface := range space.surfaces {
			if spatialSurfaceInTile(surface.surface, bounds, [4]float64{0, 0, 4, 2}) {
				binding := surface.surface.Binding
				binding.Chart = -1
				selection.Bindings = append(selection.Bindings, binding)
			}
		}

		chunk, err := compileSpatialLeaf(
			&document,
			worldchunk.PlannedChunk{Sectors: []int{index}, Solids: []int{0}},
			lighting,
			selection,
			spatialBuildTile{bounds: bounds, density: 1},
		)
		if err != nil {
			t.Fatal(err)
		}

		chunks = append(chunks, chunk)
	}

	return document, chunks, policy, worldchunk.Options{Lightmap: lighting, MaxSpan: 4, MaxMeshBytes: 16 << 20, MaxEncodedBytes: 16 << 20}
}

func TestGPUSpatialMergePreservesOwnersCoverageAndWaterVertices(t *testing.T) {
	t.Parallel()
	document, chunks, policy, options := gpuSpatialMergeFixture(t)
	settings := &spatialChunkSettings{TileSize: 2, LoadTileSize: 4}

	strict, _, err := groupSpatialLoadingUnits(&document, slices.Clone(chunks), []int{0, 0}, settings, options, nil)
	if err != nil || len(strict) != 2 {
		t.Fatal("semantic selections were combined", len(strict), err)
	}

	surfaces, err := worldchunk.ReadySurfaces([]worldchunk.Mesh{chunks[0].Mesh, chunks[1].Mesh}, &document)
	if err != nil {
		t.Fatal(err)
	}

	ready := worldchunk.ReadyOptions{Surfaces: surfaces}

	merged, mergedParents, err := groupSpatialLoadingUnits(&document, slices.Clone(chunks), []int{0, 0}, settings, options, &ready, policy)
	if err != nil || len(merged) != 1 || !slices.Equal(mergedParents, []int{0}) {
		t.Fatal("GPU selections did not combine with stable parent ownership", len(merged), mergedParents, err)
	}

	if !slices.Equal(merged[0].Sectors, []int{0, 1}) || !slices.Equal(merged[0].Solids, []int{0}) ||
		len(merged[0].Layout.Selection.LiquidTiles) != 2 {
		t.Fatal("owner or original water tile lost")
	}

	before, waterBefore := gpuSpatialCoverage(chunks)

	after, waterAfter := gpuSpatialCoverage(merged)
	if len(before) != len(after) || len(waterBefore) != len(waterAfter) {
		t.Fatal("coverage owners or water vertex count changed", len(before), len(after), len(waterBefore), len(waterAfter))
	}

	for owner, area := range before {
		if math.Abs(after[owner]-area) > 1e-6 {
			t.Fatal("geometry coverage changed", owner, area, after[owner])
		}
	}

	for vertex := range waterBefore {
		if !waterAfter[vertex] {
			t.Fatal("animated water position/phase changed", vertex)
		}
	}
}

func TestGPUSpatialMergeKeepsParentAndCapacityBounds(t *testing.T) {
	t.Parallel()
	document, chunks, policy, options := gpuSpatialMergeFixture(t)
	settings := &spatialChunkSettings{TileSize: 2, LoadTileSize: 4}

	for _, parents := range [][]int{{0, 1}, {0, 0}} {
		limited := options
		if parents[0] == parents[1] {
			limited.MaxMeshBytes = 1
		}

		merged, mergedParents, err := groupSpatialLoadingUnits(
			&document,
			slices.Clone(chunks),
			slices.Clone(parents),
			settings,
			limited,
			nil,
			policy,
		)
		if err != nil || len(merged) != 2 || !slices.Equal(mergedParents, parents) {
			t.Fatal("parent/capacity restriction lost", parents, len(merged), mergedParents, err)
		}
	}
}

func TestSpatialMergeRejectsIntermediateEntryOverflowBeforeEncoding(t *testing.T) {
	t.Parallel()
	document, chunks, _, options := gpuSpatialMergeFixture(t)
	chunk := chunks[0]
	// The stream remains within the scalar vertex-count bound but exceeds the
	// semantic entry's byte bound. It must be a non-fit, not an encoding error.
	chunk.Mesh.Vertices = make([]worldchunk.Vertex, worldchunk.MaxVertices)

	fits, err := spatialLoadingFits(&chunk, &document, options, nil)
	if err != nil || fits {
		t.Fatal("oversized intermediate entry reached the encoder", fits, err)
	}
}

func gpuSpatialCoverage(chunks []worldchunk.PlannedChunk) (map[worldlightmap.Binding]float64, map[[5]float32]bool) {
	area := map[worldlightmap.Binding]float64{}
	water := map[[5]float32]bool{}

	for _, chunk := range chunks {
		for index, triangle := range chunk.Mesh.Triangles {
			owner := triangle.Sector
			if triangle.Solid >= 0 {
				owner = triangle.Solid
			}

			binding := worldlightmap.Binding{
				Kind:  triangle.Surface,
				Index: owner,
				Edge:  triangle.SurfaceEdge,
				Chart: int(triangle.Material),
			}

			polygon := make([]world.Vec3, 0, 3)

			for corner := range 3 {
				v := chunk.Mesh.Vertices[chunk.Mesh.Indices[index*3+corner]]
				p := v.Position

				polygon = append(polygon, world.Vec3{X: float64(p[0]), Y: float64(p[1]), Z: float64(p[2])})
				if triangle.Material == 2 {
					water[[5]float32{p[0], p[1], p[2], v.WaveUV[0], v.WaveUV[1]}] = true
				}
			}

			area[binding] += spatialPolygonArea(polygon)
		}
	}

	return area, water
}
