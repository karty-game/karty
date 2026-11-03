package levelbuild

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldterrain"
)

const gpuMaxSourceReferences = 65536

type gpuCellPlan struct {
	space             uint32
	group, span       uint32
	grid              [2]int32
	physical, sectors []uint32
	chunk             worldchunk.PlannedChunk
}

type gpuCellSurface struct {
	surface  worldlightmap.Surface
	physical int
}

type gpuCellSpace struct {
	id, group uint32
	bounds    [4]float64
	room      string
	sectors   []int
	surfaces  []gpuCellSurface
}

// Indoor geometry keeps complete authored room groups inside identity-portal
// spaces. Explicit spatial and terrain sources retain their authored tiling.
// Sector references drive streaming; grid metadata never implies adjacency.
func planGPUCells(ctx context.Context, document *world.Document, physical worldchunk.PlanResult,
	terrain *worldterrain.Document, cellSize float64, lighting worldlightmap.Options, liquidEdgeStep float64, policies ...gpuCellPolicy,
) ([]gpuCellPlan, error) {
	if math.IsNaN(cellSize) || math.IsInf(cellSize, 0) || cellSize < 1 || cellSize > 1024 {
		return nil, ErrManifest
	}

	var policy gpuCellPolicy
	if len(policies) > 0 {
		policy = policies[0]
	}

	spaces, err := worldgpuchunk.PortalSpaces(document)
	if err != nil {
		return nil, err
	}

	groups, err := gpuCellSpaces(document, physical, terrain, spaces, policy.spatial)
	if err != nil {
		return nil, err
	}

	if terrain == nil {
		groups, err = gpuRoomSpaces(document, groups, policy.roomGroups)
		if err != nil {
			return nil, err
		}
	}

	var result []gpuCellPlan

	work, references := 0, 0

	for _, space := range groups {
		if space.sectors != nil {
			room, roomErr := planGPURoom(ctx, document, physical, space, cellSize, lighting, liquidEdgeStep, policy)
			if roomErr != nil {
				return nil, roomErr
			}

			work += len(space.surfaces)
			references += len(space.surfaces)

			if work > 8*1024*1024 || references > gpuMaxSourceReferences {
				return nil, fmt.Errorf("GPU room source work budget: %w", worldgpuchunk.ErrChunk)
			}

			if len(room.chunk.Mesh.Triangles) > 0 {
				result = append(result, room)
			}

			if len(result) > worldgpuchunk.MaxCells {
				return nil, worldgpuchunk.ErrChunk
			}

			continue
		}

		planned, err := planGPUCellSpace(ctx, document, physical, space, cellSize, lighting, liquidEdgeStep, policy, &work, &references)
		if err != nil {
			return nil, err
		}

		result = append(result, planned...)
		if len(result) > worldgpuchunk.MaxCells {
			return nil, worldgpuchunk.ErrChunk
		}
	}

	slices.SortFunc(result, compareGPUCellPlans)

	for index := range result {
		result[index].chunk.ID = index
	}

	return result, nil
}

func compareGPUCellPlans(left, right gpuCellPlan) int {
	for _, pair := range [][2]int64{
		{int64(left.space), int64(right.space)}, {int64(left.group), int64(right.group)},
		{int64(left.grid[1]), int64(right.grid[1])}, {int64(left.grid[0]), int64(right.grid[0])},
		{int64(left.span), int64(right.span)},
	} {
		if pair[0] < pair[1] {
			return -1
		}

		if pair[0] > pair[1] {
			return 1
		}
	}

	return int(left.sectors[0]) - int(right.sectors[0])
}

func gpuCellSpaces(
	document *world.Document,
	physical worldchunk.PlanResult,
	terrain *worldterrain.Document,
	spaces []uint32,
	spatial *spatialChunkSettings,
) ([]gpuCellSpace, error) {
	if len(physical.SectorChunks) != len(document.Sectors) {
		return nil, worldgpuchunk.ErrChunk
	}

	surfaces, err := worldlightmap.Surfaces(document)
	if err != nil {
		return nil, err
	}

	solidParents, err := gpuSolidParents(physical, spaces)
	if err != nil {
		return nil, err
	}

	var prepared *spatialWorldChunks
	if spatial != nil {
		prepared, err = prepareSpatialWorldChunks(manifest{Chunks: &chunkBuildSettings{Spatial: spatial}}, document, terrain)
		if err != nil {
			return nil, err
		}
	}

	groups := make(map[[2]uint32]*gpuCellSpace)

	for surfaceIndex, surface := range surfaces {
		if len(surface.Polygons) == 0 {
			continue
		}

		binding := surface.Binding
		if terrain != nil && binding.Kind == "sector-floor" && slices.Contains(terrain.ExteriorSectors, uint16(binding.Index)) {
			continue
		}

		var parent int

		if strings.HasPrefix(binding.Kind, "solid-") {
			var found bool

			parent, found = solidParents[binding.Index]
			if !found {
				return nil, worldgpuchunk.ErrChunk
			}
		} else {
			parent = physical.SectorChunks[binding.Index]
		}

		if parent < 0 || parent >= len(physical.Chunks) {
			return nil, worldgpuchunk.ErrChunk
		}

		space := spaces[physical.Chunks[parent].Sectors[0]]

		quality := gpuSourceGroup(prepared, surfaceIndex)

		key := [2]uint32{space, quality}

		group := groups[key]
		if group == nil {
			group = &gpuCellSpace{id: space, group: quality, bounds: [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}}
			groups[key] = group
		}

		bounds := spatialSurfaceBounds(surface)
		group.bounds = [4]float64{
			min(group.bounds[0], bounds[0]),
			min(group.bounds[1], bounds[1]),
			max(group.bounds[2], bounds[2]),
			max(group.bounds[3], bounds[3]),
		}
		group.surfaces = append(group.surfaces, gpuCellSurface{surface: surface, physical: parent})
	}

	result := make([]gpuCellSpace, 0, len(groups))
	for _, group := range groups {
		result = append(result, *group)
	}

	slices.SortFunc(result, func(left, right gpuCellSpace) int {
		if left.id < right.id {
			return -1
		}

		if left.id > right.id {
			return 1
		}

		return int(left.group) - int(right.group)
	})

	return result, nil
}

func selectGPUCell(
	physical worldchunk.PlanResult,
	space gpuCellSpace,
	bounds [4]float64,
	liquidEdgeStep float64,
) (worldlightmap.SurfaceSelection, []uint32, []int, []int) {
	selection := worldlightmap.SurfaceSelection{Clip: &bounds, LiquidEdgeStep: liquidEdgeStep}

	var (
		parents         []uint32
		sectors, solids []int
	)

	for _, source := range space.surfaces {
		if !spatialSurfaceInTile(source.surface, bounds, space.bounds) {
			continue
		}

		binding := source.surface.Binding
		binding.Chart = -1
		selection.Bindings = append(selection.Bindings, binding)

		parents = append(parents, uint32(source.physical))
		if strings.HasPrefix(binding.Kind, "solid-") {
			solids = append(solids, binding.Index)
			if space.sectors == nil {
				sectors = append(sectors, physical.Chunks[source.physical].Sectors...)
			}
		} else {
			sectors = append(sectors, binding.Index)
		}
	}

	if space.sectors != nil {
		sectors = append(sectors, space.sectors...)
		for _, sector := range space.sectors {
			parents = append(parents, uint32(physical.SectorChunks[sector]))
		}
	}

	slices.Sort(parents)
	parents = slices.Compact(parents)

	slices.Sort(sectors)
	sectors = slices.Compact(sectors)

	slices.Sort(solids)
	solids = slices.Compact(solids)

	return selection, parents, sectors, solids
}

func gpuIDs(values []int) []uint32 {
	ids := make([]uint32, len(values))
	for index, value := range values {
		ids[index] = uint32(value)
	}

	return ids
}

func compileGPUCellGeometry(
	document *world.Document,
	sectors, solids []int,
	lighting worldlightmap.Options,
	selection worldlightmap.SurfaceSelection,
	bounds [4]float64, minimumPages ...int,
) (worldchunk.PlannedChunk, error) {
	if lighting.PageSize > 0 {
		minimum := 32
		if len(minimumPages) > 0 && minimumPages[0] > minimum {
			minimum = minimumPages[0]
		}

		return compileSpatialLeafWithMinimum(
			document,
			worldchunk.PlannedChunk{Sectors: sectors, Solids: solids},
			lighting,
			selection,
			spatialBuildTile{bounds: bounds, density: lighting.TexelsPerUnit}, minimum,
		)
	}

	mesh, err := worldchunk.CompileSelectedMesh(*document, sectors, solids, selection)
	if err != nil {
		return worldchunk.PlannedChunk{}, err
	}

	return worldchunk.PlannedChunk{
		Sectors:   sectors,
		Solids:    solids,
		Mesh:      mesh,
		Bounds:    mesh.Bounds,
		MeshBytes: worldchunk.DecodedMeshBytes(mesh),
	}, nil
}

func planGPUCellSpace(
	ctx context.Context,
	document *world.Document,
	physical worldchunk.PlanResult,
	space gpuCellSpace,
	cellSize float64,
	lighting worldlightmap.Options,
	liquidEdgeStep float64,
	policy gpuCellPolicy,
	work, references *int,
) ([]gpuCellPlan, error) {
	tiles, err := gpuSourceTiles(space, cellSize, lighting.TexelsPerUnit, policy.spatial)
	if err != nil {
		return nil, err
	}

	var planned []gpuCellPlan

	for _, tile := range tiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		*work += len(space.surfaces)
		if *work > 8*1024*1024 {
			return nil, fmt.Errorf("GPU cell clipping work budget: %w", worldgpuchunk.ErrChunk)
		}

		selection, parents, sectors, solids := selectGPUCell(physical, space, tile.bounds, liquidEdgeStep)
		selection.LiquidMaxSubdivisions = tile.liquidCap

		*references += len(selection.Bindings)
		if *references > gpuMaxSourceReferences {
			return nil, fmt.Errorf("GPU cell source reference budget: %w", worldgpuchunk.ErrChunk)
		}

		if len(selection.Bindings) == 0 {
			continue
		}

		sourceLighting := lighting
		sourceLighting.TexelsPerUnit = tile.density

		chunk, err := compileGPUCellGeometry(document, sectors, solids, sourceLighting, selection, tile.bounds, policy.minimumPage)
		if err != nil {
			return nil, fmt.Errorf("GPU cell space%d grid%v: %w", space.id, tile.grid, err)
		}

		if len(chunk.Mesh.Triangles) == 0 {
			continue
		}

		planned = append(
			planned,
			gpuCellPlan{
				space:    space.id,
				group:    space.group,
				grid:     tile.grid,
				physical: parents,
				sectors:  gpuIDs(sectors),
				chunk:    chunk,
				span:     tile.span,
			},
		)
	}

	planned, err = mergeGPUCellPlans(planned, space, physical, document, cellSize, policy)
	if err != nil {
		return nil, err
	}

	return planned, nil
}

func gpuSolidParents(physical worldchunk.PlanResult, spaces []uint32) (map[int]int, error) {
	solidParents := make(map[int]int)

	for index, chunk := range physical.Chunks {
		if len(chunk.Sectors) == 0 {
			return nil, worldgpuchunk.ErrChunk
		}

		for _, solid := range chunk.Solids {
			solidParents[solid] = index
		}

		for _, sector := range chunk.Sectors {
			if spaces[sector] != spaces[chunk.Sectors[0]] {
				return nil, worldgpuchunk.ErrChunk
			}
		}
	}

	return solidParents, nil
}

func gpuSourceGroup(prepared *spatialWorldChunks, index int) uint32 {
	if prepared != nil && prepared.kinds[index] == "spatial" {
		return 1
	}

	return 0
}
