package levelbuild

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func gpuRoomTestLighting() worldlightmap.Options {
	return worldlightmap.Options{
		PageSize:      512,
		MaxPages:      1,
		MaxTexels:     512 * 512,
		TexelsPerUnit: 1,
		Padding:       2,
		Lights:        []string{"near", "far"},
	}
}

func gpuRoomTestPhysical(t *testing.T, document world.Document) worldchunk.PlanResult {
	t.Helper()

	result, err := worldchunk.Plan(document, worldchunk.Options{NoLightmap: true, MaxSectors: 1})
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func TestGPUWholeRoomDefaultKeepsConvexComponentsTogether(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	document.Sectors[0].SourceRoom = "joined-room"
	document.Sectors[1].SourceRoom = "joined-room"
	document.Sectors[0].Walls[1].SourceEdge = ""
	document.Sectors[1].Walls[3].SourceEdge = ""
	physical := gpuRoomTestPhysical(t, document)

	cells, err := planGPUCells(t.Context(), &document, physical, nil, 2, gpuRoomTestLighting(), 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(cells) != 2 || !slices.Equal(cells[0].sectors, []uint32{0, 1}) || !slices.Equal(cells[1].sectors, []uint32{2}) {
		t.Fatal("convex decomposition became a loading boundary", cells)
	}

	if cells[0].chunk.Bounds[0] != 0 || cells[0].chunk.Bounds[3] != 30 {
		t.Fatal("joined room geometry clipped by the grid", cells[0].chunk.Bounds)
	}
}

func TestGPUWholeRoomExplicitGroupsRejectInvalidOwnership(t *testing.T) {
	t.Parallel()

	cases := map[string][][]string{
		"unknown":                            {{"missing-room"}},
		"empty":                              {{}},
		"duplicate within group":             {{"near-room", "near-room"}},
		"duplicate across groups":            {{"near-room"}, {"near-room"}},
		"disconnected through another group": {{"near-room", "far-room"}},
	}
	for name, groups := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			document, _, _ := incrementalBakeFixture(t)
			physical := gpuRoomTestPhysical(t, document)

			cells, err := planGPUCells(
				t.Context(),
				&document,
				physical,
				nil,
				2,
				gpuRoomTestLighting(),
				0,
				gpuCellPolicy{roomGroups: groups},
			)
			if err == nil || len(cells) != 0 {
				t.Fatal("invalid room ownership published chunks", name, cells, err)
			}
		})
	}
}

func TestGPUWholeRoomGroupsPreserveCompleteSolidAndSurfaceGeometry(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	document.StaticSolids = &world.StaticSolids{Version: world.StaticSolidsVersion, Items: []world.Solid{{
		ID: "cross-boundary-solid", Footprint: []world.Vec2{{X: 1.75, Y: .5}, {X: 2.25, Y: .5}, {X: 2.25, Y: 1.5}, {X: 1.75, Y: 1.5}},
		Bottom: world.Plane{}, Top: world.Plane{C: 1}, TopMaterial: 1, SideMaterial: 1, BottomMaterial: 1,
	}}}
	physical := gpuRoomTestPhysical(t, document)

	cells, err := planGPUCells(t.Context(), &document, physical, nil, 2, gpuRoomTestLighting(), 0,
		gpuCellPolicy{roomGroups: [][]string{{"near-room", "corridor"}}})
	if err != nil {
		t.Fatal(err)
	}

	if len(cells) != 2 {
		t.Fatal("explicit loading room split by grid", len(cells))
	}

	owners := 0

	var actual, expected float64
	for _, cell := range cells {
		actual += gpuTestMeshArea(cell.chunk.Mesh)
		for _, solid := range cell.chunk.Solids {
			if solid == 0 {
				owners++
			}
		}
	}

	complete, err := worldchunk.CompileMesh(document, []int{0, 1, 2}, []int{0}, worldlightmap.Layout{})
	if err != nil {
		t.Fatal(err)
	}

	expected = gpuTestMeshArea(complete)
	if owners != 1 || math.Abs(actual-expected) > 1e-5 {
		t.Fatal("solid or surface geometry omitted/duplicated", owners, actual, expected)
	}
}

func TestGPUWholeRoomCapacityFailsWithoutGridFallbackOrDensityReduction(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	physical := gpuRoomTestPhysical(t, document)
	lighting := gpuRoomTestLighting()
	lighting.PageSize, lighting.MaxTexels, lighting.TexelsPerUnit = 32, 32*32, 8

	cells, err := planGPUCells(t.Context(), &document, physical, nil, 2, lighting, 0)
	if !errors.Is(err, worldlightmap.ErrPageCapacity) || !strings.Contains(err.Error(), "room") ||
		!strings.Contains(err.Error(), "capacity") ||
		len(cells) != 0 {
		t.Fatal("oversized whole room was silently tiled or quality reduced", len(cells), err)
	}
}

func TestGPUWholeRoomIdentityIncludesInstanceAndPortalSpace(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	document.Sectors[0].SourceRoom = "reused-room"
	document.Sectors[1].SourceRoom = "reused-room"
	document.Sectors[0].Instance = "instance-a"
	document.Sectors[1].Instance = "instance-b"
	physical := gpuRoomTestPhysical(t, document)

	cells, err := planGPUCells(t.Context(), &document, physical, nil, 2, gpuRoomTestLighting(), 0)
	if err != nil || len(cells) != 3 {
		t.Fatal("independent prefab room instances merged", len(cells), err)
	}

	for edge := range document.Sectors[2].Walls {
		document.Sectors[2].Walls[edge].Start.X -= 30
		document.Sectors[2].Walls[edge].End.X -= 30
	}

	document.Lighting.Lights[1].Position.X -= 30
	document.Sectors[2].SourceRoom = "reused-room"
	document.Sectors[2].Instance = "instance-a"
	physical = gpuRoomTestPhysical(t, document)

	cells, err = planGPUCells(t.Context(), &document, physical, nil, 2, gpuRoomTestLighting(), 0)
	if err != nil || len(cells) != 3 {
		t.Fatal("overlapping non-Euclidean room identity merged", len(cells), err)
	}

	seen := map[uint32]uint32{}

	for _, cell := range cells {
		for _, sector := range cell.sectors {
			seen[sector] = cell.space
		}
	}

	if seen[0] == seen[2] {
		t.Fatal("overlapping rooms share a portal space", seen)
	}
}

func TestGPUWholeRoomCatalogOrderKeepsDistinctRoomsSharingGridBounds(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	shrink := func(coordinate float64) float64 {
		switch coordinate {
		case 30:
			return 4
		case 32:
			return 6
		default:
			return coordinate
		}
	}

	for index := range document.Sectors {
		for edge := range document.Sectors[index].Walls {
			wall := &document.Sectors[index].Walls[edge]
			wall.Start.X, wall.End.X = shrink(wall.Start.X), shrink(wall.End.X)
		}
	}

	document.Lighting.Lights[1].Position.X = 5
	physical := gpuRoomTestPhysical(t, document)

	cells, err := planGPUCells(t.Context(), &document, physical, nil, 8, gpuRoomTestLighting(), 0)
	if err != nil || len(cells) != 3 {
		t.Fatal("shared grid room catalog lost entries", len(cells), err)
	}

	for index, cell := range cells {
		if cell.grid != [2]int32{} || cell.span != 1 || len(cell.sectors) != 1 || int(cell.sectors[0]) != index {
			t.Fatal("room identities collapsed or reordered by matching grid/span", cells)
		}
	}
}
