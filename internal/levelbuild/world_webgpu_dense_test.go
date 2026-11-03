package levelbuild

import (
	"fmt"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
)

func TestWebGPUChartDenseGridKeepsIndependentFootprints(t *testing.T) {
	t.Parallel()

	for _, grid := range []int{4, 8} {
		t.Run(fmt.Sprintf("grid%d", grid), func(t *testing.T) {
			t.Parallel()

			checkDenseChartGrid(t, grid)
		})
	}
}

func setDenseChartTestColor(baker *webGPUChartBaker, source worldmaterial.Rect, value byte) {
	for y := range source.Height {
		for x := range source.Width {
			p := baker.source.Albedo().NRGBAAt(source.X+x, source.Y+y)
			p.R = value
			baker.source.Albedo().SetNRGBA(source.X+x, source.Y+y, p)
		}
	}
}

func checkDenseChartGrid(t *testing.T, grid int) {
	t.Helper()

	document := world.Document{}

	baker, err := newWebGPUChartBaker(&document, webGPUChartTestPair(t), 4)
	if err != nil {
		t.Fatal(err)
	}

	baker.grid = grid
	chart := webGPUChart{
		primary: 1,
		mapping: webGPUChartTestMapping(),
		normal:  chartVector{0, 0, 1},
		points:  []chartVector{{0, 0, 0}, {2, 0, 0}, {2, 2, 0}, {0, 2, 0}},
	}
	capacity, cellSize := grid*grid, 256/grid

	source := baker.materials[1]
	for cell := range capacity + 1 {
		// Distinct constant source colors exercise every cell and the transition
		// to the next tile without relying on material dedup rounding.
		setDenseChartTestColor(baker, source, byte(cell+1))

		id, mapping, err := baker.bake(t.Context(), chart)
		if err != nil || id != uint32(4+cell/capacity) {
			t.Fatalf("chart %d: material %d: %v", cell, id, err)
		}

		point := chartVector{1, 1, 0}

		column, row := int(chartPlane(mapping.Projections[0].U, point)*256), int(chartPlane(mapping.Projections[0].V, point)*256)
		if column/cellSize != cell%grid || row/cellSize != cell%capacity/grid {
			t.Fatal("mapping escaped own dense cell", cell, column, row)
		}

		pixels := baker.tiles[cell/capacity].albedo
		if pixels.NRGBAAt(column, row) != pixels.NRGBAAt(cell%grid*cellSize, cell%capacity/grid*cellSize) {
			t.Fatal("dense gutter lost own chart", cell)
		}
	}

	if len(baker.tiles) != 2 || baker.charts != capacity+1 {
		t.Fatal("dense packing lost independent charts")
	}
}
