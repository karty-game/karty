package levelbuild

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

type gpuRoomKey struct {
	space          uint32
	group          int
	room, instance string
}

// Explicit build-only groups join reviewed source-room fragments. Neither
// coordinate overlap nor physical transport bins determine render ownership.
func gpuRoomSectorGroups(document *world.Document, configured [][]string) ([]int, []string, error) {
	spaces, err := worldgpuchunk.PortalSpaces(document)
	if err != nil {
		return nil, nil, err
	}

	known := make(map[string]bool)
	for _, sector := range document.Sectors {
		known[sector.SourceRoom] = true
	}

	explicit := make(map[string]int)

	for group, rooms := range configured {
		if len(rooms) == 0 || len(configured) > world.MaxSectors {
			return nil, nil, fmt.Errorf("GPU room_groups contains an empty or oversized group: %w", ErrManifest)
		}

		for _, room := range rooms {
			if !known[room] || room == "" {
				return nil, nil, fmt.Errorf("GPU room_groups references unknown room %q: %w", room, ErrManifest)
			}

			if _, duplicate := explicit[room]; duplicate {
				return nil, nil, fmt.Errorf("GPU room_groups repeats room %q: %w", room, ErrManifest)
			}

			explicit[room] = group + 1
		}
	}

	keys := make(map[gpuRoomKey]int)
	assignments := make([]int, len(document.Sectors))

	var names []string

	for index, sector := range document.Sectors {
		key := gpuRoomKey{space: spaces[index], group: explicit[sector.SourceRoom], room: sector.SourceRoom, instance: sector.Instance}
		name := sector.SourceRoom

		if key.group != 0 {
			key.room = ""
			name = strings.Join(configured[key.group-1], ", ")
		}

		group, exists := keys[key]
		if !exists {
			group = len(names)
			keys[key] = group

			names = append(names, name)
		}

		assignments[index] = group
	}

	if err := validateGPURoomConnectivity(document, assignments, names); err != nil {
		return nil, nil, err
	}

	return assignments, names, nil
}

func validateGPURoomConnectivity(document *world.Document, assignments []int, names []string) error {
	filtered := *document
	filtered.Sectors = slices.Clone(document.Sectors)

	for index := range filtered.Sectors {
		filtered.Sectors[index].Walls = slices.Clone(document.Sectors[index].Walls)
		for edge := range filtered.Sectors[index].Walls {
			wall := &filtered.Sectors[index].Walls[edge]
			if wall.Portal >= 0 && assignments[index] != assignments[wall.Portal] {
				wall.Portal, wall.PortalWall = -1, 0
			}
		}
	}

	spaces, err := worldgpuchunk.PortalSpaces(&filtered)
	if err != nil {
		return err
	}

	roots := make(map[int]uint32)
	for sector, group := range assignments {
		if root, exists := roots[group]; exists && root != spaces[sector] {
			return fmt.Errorf("GPU room group %q is not portal-connected: %w", names[group], ErrManifest)
		}

		roots[group] = spaces[sector]
	}

	return nil
}

func gpuRoomSpaces(document *world.Document, source []gpuCellSpace, configured [][]string) ([]gpuCellSpace, error) {
	assignments, names, err := gpuRoomSectorGroups(document, configured)
	if err != nil {
		return nil, err
	}

	owners, err := worldchunk.SolidOwners(document)
	if err != nil {
		return nil, err
	}

	var result []gpuCellSpace

	for _, original := range source {
		if original.group != 0 {
			result = append(result, original)

			continue
		}

		groups := make(map[int]*gpuCellSpace)

		for _, surface := range original.surfaces {
			owner := surface.surface.Binding.Index
			if strings.HasPrefix(surface.surface.Binding.Kind, "solid-") {
				owner = owners[owner]
			}

			groupIndex := assignments[owner]
			group := groups[groupIndex]

			if group == nil {
				group = &gpuCellSpace{
					id: original.id, room: names[groupIndex],
					bounds: [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)},
				}
				for sector, assignment := range assignments {
					if assignment == groupIndex {
						group.sectors = append(group.sectors, sector)
					}
				}

				groups[groupIndex] = group
			}

			bounds := spatialSurfaceBounds(surface.surface)
			group.bounds = [4]float64{
				min(group.bounds[0], bounds[0]), min(group.bounds[1], bounds[1]),
				max(group.bounds[2], bounds[2]), max(group.bounds[3], bounds[3]),
			}
			group.surfaces = append(group.surfaces, surface)
		}

		for _, group := range groups {
			result = append(result, *group)
		}
	}

	return result, nil
}

func planGPURoom(ctx context.Context, document *world.Document, physical worldchunk.PlanResult, room gpuCellSpace,
	cellSize float64, lighting worldlightmap.Options, liquidEdgeStep float64, policy gpuCellPolicy,
) (gpuCellPlan, error) {
	if err := ctx.Err(); err != nil {
		return gpuCellPlan{}, err
	}

	bounds := room.bounds
	if liquidEdgeStep > 0 {
		bounds = [4]float64{math.Floor(bounds[0]/liquidEdgeStep) * liquidEdgeStep, math.Floor(bounds[1]/liquidEdgeStep) * liquidEdgeStep,
			math.Ceil(bounds[2]/liquidEdgeStep) * liquidEdgeStep, math.Ceil(bounds[3]/liquidEdgeStep) * liquidEdgeStep}
	}

	selection, parents, sectors, solids := selectGPUCell(physical, room, bounds, liquidEdgeStep)
	if liquidEdgeStep == 0 {
		selection.Clip = nil
	}

	chunk, err := compileGPUCellGeometry(document, sectors, solids, lighting, selection, bounds, policy.minimumPage)
	if err != nil {
		if errors.Is(err, worldlightmap.ErrPageCapacity) {
			return gpuCellPlan{}, fmt.Errorf(
				"GPU room %q exceeds authored page capacity %d at density %g: %w",
				room.room, lighting.PageSize, lighting.TexelsPerUnit, err,
			)
		}

		return gpuCellPlan{}, fmt.Errorf("GPU room %q: %w", room.room, err)
	}

	grid, span, err := gpuGridBounds(bounds, cellSize)
	if err != nil {
		return gpuCellPlan{}, err
	}

	return gpuCellPlan{space: room.id, grid: grid, span: span, physical: parents, sectors: gpuIDs(sectors), chunk: chunk}, nil
}
