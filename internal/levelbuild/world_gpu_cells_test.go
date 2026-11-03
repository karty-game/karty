package levelbuild

import (
	"math"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func TestGPUCellsKeepWholeRoomsWithinPortalSpace(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)

	physical, err := worldchunk.Plan(document, worldchunk.Options{NoLightmap: true, MaxSectors: 1})
	if err != nil {
		t.Fatal(err)
	}

	lighting := worldlightmap.Options{
		PageSize:      512,
		MaxPages:      1,
		MaxTexels:     512 * 512,
		TexelsPerUnit: 1,
		Padding:       2,
		Lights:        []string{"near", "far"},
	}

	cells, err := planGPUCells(t.Context(), &document, physical, nil, 2, lighting, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(cells) != 3 {
		t.Fatal("whole source rooms were split by the XY grid", len(cells))
	}

	var area, original float64

	for _, cell := range cells {
		if cell.space != 0 || len(cell.sectors) != 1 || len(cell.physical) != 1 {
			t.Fatal("whole room lost portal-space/physical ownership", cell)
		}

		if cell.chunk.Layout.TexelsPerUnit != lighting.TexelsPerUnit {
			t.Fatal("whole-room planning lowered bake density")
		}

		area += gpuTestMeshArea(cell.chunk.Mesh)
	}

	for _, chunk := range physical.Chunks {
		original += gpuTestMeshArea(chunk.Mesh)
	}

	if math.Abs(area-original) > 1e-5 || cells[1].span <= 1 {
		t.Fatal("whole-room geometry was lost, duplicated or clipped to one grid tile", area, original)
	}

	// Unrelated geometry at identical coordinates must retain a separate
	// topological owner, even though a global XY grid would merge it.
	for edge := range document.Sectors[2].Walls {
		document.Sectors[2].Walls[edge].Start.X -= 30
		document.Sectors[2].Walls[edge].End.X -= 30
	}

	document.Lighting.Lights[1].Position.X -= 30

	physical, err = worldchunk.Plan(document, worldchunk.Options{NoLightmap: true, MaxSectors: 1})
	if err != nil {
		t.Fatal(err)
	}

	cells, err = planGPUCells(t.Context(), &document, physical, nil, 2, lighting, 0)
	if err != nil {
		t.Fatal(err)
	}

	last := cells[len(cells)-1]
	if last.space != 2 || last.grid != [2]int32{} || len(last.sectors) != 1 || last.sectors[0] != 2 {
		t.Fatal("nonEuclidean destination was merged by coordinates", last)
	}

	spaces, err := worldgpuchunk.PortalSpaces(&document)
	if err != nil || spaces[0] != 0 || spaces[2] != 2 {
		t.Fatal(spaces, err)
	}
}

func TestGPUCellsPreserveAuthoredSpatialDensityGroups(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	document.StaticSolids = &world.StaticSolids{
		Version: world.StaticSolidsVersion,
		Items: []world.Solid{
			{
				ID:             "pillar",
				Footprint:      []world.Vec2{{X: .25, Y: .25}, {X: .75, Y: .25}, {X: .75, Y: .75}, {X: .25, Y: .75}},
				Bottom:         world.Plane{},
				Top:            world.Plane{C: 1},
				TopMaterial:    1,
				SideMaterial:   1,
				BottomMaterial: 1,
			},
		},
	}

	physical, err := worldchunk.Plan(document, worldchunk.Options{NoLightmap: true, MaxSectors: 1})
	if err != nil {
		t.Fatal(err)
	}

	lighting := worldlightmap.Options{
		PageSize:      512,
		MaxPages:      1,
		MaxTexels:     512 * 512,
		TexelsPerUnit: 1,
		Padding:       2,
		Lights:        []string{"near", "far"},
	}
	settings := &spatialChunkSettings{
		TileSize:           4,
		BackgroundTileSize: 16,
		Density:            .5,
		BackgroundDensity:  .125,
		DetailBounds:       []float64{0, 0, 8, 4},
		Solids:             []string{"pillar"},
	}

	cells, err := planGPUCells(
		t.Context(),
		&document,
		physical,
		nil,
		4,
		lighting,
		0,
		gpuCellPolicy{
			spatial: settings,
			options: worldchunk.Options{Lightmap: lighting, MaxSpan: 16, MaxMeshBytes: 16 << 20, MaxEncodedBytes: 8 << 20},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	ordinary, fine, background := 0, 0, 0

	var total float64

	for _, cell := range cells {
		switch {
		case cell.group == 0:
			ordinary++

			if cell.chunk.Layout.TexelsPerUnit != 1 {
				t.Fatal("ordinary source quality changed")
			}
		case cell.span > 1:
			background++

			if cell.chunk.Layout.TexelsPerUnit != .125 {
				t.Fatal("background source quality changed")
			}
		default:
			fine++

			if cell.chunk.Layout.TexelsPerUnit != .5 {
				t.Fatal("fine spatial source quality changed")
			}
		}

		total += gpuTestMeshArea(cell.chunk.Mesh)
	}

	var original float64
	for _, chunk := range physical.Chunks {
		original += gpuTestMeshArea(chunk.Mesh)
	}

	if ordinary == 0 || fine == 0 || background == 0 || math.Abs(total-original) > 1e-5 {
		t.Fatal("spatial partition changed source coverage/groups", ordinary, fine, background, total, original)
	}
}

func gpuTestMeshArea(mesh worldchunk.Mesh) float64 {
	var total float64

	for index := range mesh.Triangles {
		points := make([]world.Vec3, 3)

		for corner := range 3 {
			position := mesh.Vertices[mesh.Indices[index*3+corner]].Position
			points[corner] = world.Vec3{X: float64(position[0]), Y: float64(position[1]), Z: float64(position[2])}
		}

		total += spatialPolygonArea(points)
	}

	return total
}
