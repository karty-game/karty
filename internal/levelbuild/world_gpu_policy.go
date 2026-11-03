package levelbuild

import (
	"math"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
)

type gpuCellPolicy struct {
	spatial     *spatialChunkSettings
	options     worldchunk.Options
	minimumPage int
	roomGroups  [][]string
}
type gpuSourceTile struct {
	bounds    [4]float64
	grid      [2]int32
	span      uint32
	density   float64
	liquidCap int
}

func gpuSourceTiles(space gpuCellSpace, cellSize, density float64, settings *spatialChunkSettings) ([]gpuSourceTile, error) {
	if space.group == 1 {
		tiles, err := spatialSurfaceTiles(space.bounds, settings, density)
		if err != nil {
			return nil, err
		}

		result := make([]gpuSourceTile, 0, len(tiles))
		for _, tile := range tiles {
			grid, span, err := gpuGridBounds(tile.bounds, cellSize)
			if err != nil {
				return nil, err
			}

			entry := gpuSourceTile{bounds: tile.bounds, grid: grid, span: span, density: tile.density}
			if settings.BackgroundLiquidSubdivisions != 0 && !spatialRectanglesOverlap(tile.bounds, settings.GeometryDetailBounds) {
				entry.liquidCap = settings.BackgroundLiquidSubdivisions
			}

			result = append(result, entry)
		}

		return result, nil
	}

	left, right := int(math.Floor(space.bounds[0]/cellSize)), int(math.Ceil(space.bounds[2]/cellSize))

	bottom, top := int(math.Floor(space.bounds[1]/cellSize)), int(math.Ceil(space.bounds[3]/cellSize))
	if uint64(right-left)*uint64(top-bottom) > worldgpuchunk.MaxCells {
		return nil, worldgpuchunk.ErrChunk
	}

	result := make([]gpuSourceTile, 0, (right-left)*(top-bottom))
	for row := bottom; row < top; row++ {
		for column := left; column < right; column++ {
			result = append(
				result,
				gpuSourceTile{
					bounds: [4]float64{
						float64(column) * cellSize,
						float64(row) * cellSize,
						float64(column+1) * cellSize,
						float64(row+1) * cellSize,
					},
					grid:    [2]int32{int32(column), int32(row)},
					span:    1,
					density: density,
				},
			)
		}
	}

	return result, nil
}

func gpuGridBounds(bounds [4]float64, cellSize float64) ([2]int32, uint32, error) {
	column, row := math.Floor(bounds[0]/cellSize), math.Floor(bounds[1]/cellSize)
	xSpan, ySpan := math.Ceil(bounds[2]/cellSize)-column, math.Ceil(bounds[3]/cellSize)-row

	span := uint32(1)
	for float64(span) < max(xSpan, ySpan) && span < 4096 {
		span *= 2
	}

	if float64(span) < max(xSpan, ySpan) || math.Abs(column) > math.MaxInt32 || math.Abs(row) > math.MaxInt32 {
		return [2]int32{}, 0, worldgpuchunk.ErrChunk
	}

	return [2]int32{int32(column), int32(row)}, span, nil
}

func mergeGPUCellPlans(
	cells []gpuCellPlan,
	space gpuCellSpace,
	physical worldchunk.PlanResult,
	document *world.Document,
	cellSize float64,
	policy gpuCellPolicy,
) ([]gpuCellPlan, error) {
	if space.group != 1 || policy.spatial.LoadTileSize == 0 {
		return cells, nil
	}

	chunks := make([]worldchunk.PlannedChunk, len(cells))

	parents := make([]int, len(cells))
	for index, cell := range cells {
		chunks[index] = cell.chunk
	}

	merged, _, err := groupSpatialLoadingUnits(document, chunks, parents, policy.spatial, policy.options, nil)
	if err != nil {
		return nil, err
	}

	result := make([]gpuCellPlan, 0, len(merged))
	for _, chunk := range merged {
		bounds := *chunk.Layout.Selection.Clip
		_, ids, _, _ := selectGPUCell(physical, space, bounds, policy.spatial.LiquidEdgeStep)

		grid, span, err := gpuGridBounds(bounds, cellSize)
		if err != nil {
			return nil, err
		}

		result = append(
			result,
			gpuCellPlan{
				space:    space.id,
				group:    space.group,
				grid:     grid,
				span:     span,
				physical: ids,
				sectors:  gpuIDs(chunk.Sectors),
				chunk:    chunk,
			},
		)
	}

	return result, nil
}
