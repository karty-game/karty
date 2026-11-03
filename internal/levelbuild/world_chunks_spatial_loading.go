package levelbuild

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func validateSpatialLoadingSettings(settings *spatialChunkSettings) error {
	if settings.LoadTileSize == 0 {
		return nil
	}

	ratio := settings.LoadTileSize / settings.TileSize
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 1 || settings.LoadTileSize > 4096 ||
		ratio != math.Trunc(ratio) || math.Log2(ratio) != math.Trunc(math.Log2(ratio)) {
		return fmt.Errorf("spatial load_tile_size must be a bounded power-of-two multiple of tile_size: %w", ErrManifest)
	}

	return nil
}

// Larger units reuse the original clipped geometry cells. Only their atlas
// packing and loading identity change; density, materials and source ownership
// remain explicit. Failed capacity trials keep the original accepted units.
func groupSpatialLoadingUnits(document *world.Document, chunks []worldchunk.PlannedChunk, parents []int,
	settings *spatialChunkSettings, options worldchunk.Options, ready *worldchunk.ReadyOptions,
) ([]worldchunk.PlannedChunk, []int, error) {
	if settings.LoadTileSize == 0 {
		return chunks, parents, nil
	}

	maximum := min(settings.LoadTileSize, options.MaxSpan)
	// Bounded passes combine rows, then columns. Original source order makes
	// repeated builds canonical; an explicit work cap also bounds sparse inputs.
	work := 0

	for axis := range 2 {
		for changed := true; changed; {
			var err error

			chunks, parents, changed, err = mergeSpatialLoadingAxis(document, chunks, parents, axis, maximum, options, ready, &work)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return chunks, parents, nil
}

func mergeSpatialLoadingAxis(document *world.Document, chunks []worldchunk.PlannedChunk, parents []int,
	axis int, maximum float64, options worldchunk.Options, ready *worldchunk.ReadyOptions, work *int,
) ([]worldchunk.PlannedChunk, []int, bool, error) {
	changed := false

	for left := 0; left < len(chunks); left++ {
		for right := left + 1; right < len(chunks); right++ {
			*work++
			if *work > 8*1024*1024 {
				return nil, nil, false, fmt.Errorf("spatial loading-unit grouping exceeds build work budget: %w", ErrManifest)
			}

			if parents[left] != parents[right] {
				continue
			}

			merged, fits, err := trySpatialLoadingMerge(document, chunks[left], chunks[right], axis, maximum, options, ready)
			if err != nil {
				return nil, nil, false, err
			}

			if !fits {
				continue
			}

			chunks[left] = merged
			chunks = slices.Delete(chunks, right, right+1)
			parents = slices.Delete(parents, right, right+1)
			changed = true

			break
		}
	}

	return chunks, parents, changed, nil
}

func trySpatialLoadingMerge(document *world.Document, left, right worldchunk.PlannedChunk,
	axis int, maximum float64, options worldchunk.Options, ready *worldchunk.ReadyOptions,
) (worldchunk.PlannedChunk, bool, error) {
	selection, bounds, ok := spatialLoadingUnion(left, right, axis, maximum)
	if !ok || len(left.Mesh.Triangles)+len(right.Mesh.Triangles) > worldchunk.MaxTriangles {
		return worldchunk.PlannedChunk{}, false, nil
	}

	merged, err := compileSpatialLeaf(document, left, options.Lightmap, selection,
		spatialBuildTile{bounds: bounds, density: left.Layout.TexelsPerUnit})
	if errors.Is(err, worldlightmap.ErrPageCapacity) {
		return worldchunk.PlannedChunk{}, false, nil
	}

	if err != nil {
		return worldchunk.PlannedChunk{}, false, err
	}

	fits, err := spatialLoadingFits(&merged, document, options, ready)

	return merged, fits, err
}

func spatialLoadingUnion(a, b worldchunk.PlannedChunk, axis int, maximum float64,
) (worldlightmap.SurfaceSelection, [4]float64, bool) {
	left, right := a.Layout.Selection, b.Layout.Selection
	if left == nil || right == nil || left.Clip == nil || right.Clip == nil ||
		a.Layout.TexelsPerUnit != b.Layout.TexelsPerUnit || left.LiquidEdgeStep != right.LiquidEdgeStep ||
		left.LiquidMaxSubdivisions != right.LiquidMaxSubdivisions || !slices.Equal(left.Bindings, right.Bindings) {
		return worldlightmap.SurfaceSelection{}, [4]float64{}, false
	}

	first, second := *left.Clip, *right.Clip

	other := 1 - axis
	if first[other] != second[other] || first[other+2] != second[other+2] ||
		first[axis+2] != second[axis] && second[axis+2] != first[axis] {
		return worldlightmap.SurfaceSelection{}, [4]float64{}, false
	}

	bounds := [4]float64{min(first[0], second[0]), min(first[1], second[1]), max(first[2], second[2]), max(first[3], second[3])}
	if bounds[2]-bounds[0] > maximum || bounds[3]-bounds[1] > maximum {
		return worldlightmap.SurfaceSelection{}, [4]float64{}, false
	}

	selection := *left
	selection.Clip = &bounds

	selection.LiquidTiles = append(spatialLoadingCells(left), spatialLoadingCells(right)...)
	if len(selection.LiquidTiles) > worldlightmap.MaxLiquidTiles {
		return worldlightmap.SurfaceSelection{}, [4]float64{}, false
	}

	return selection, bounds, true
}

func spatialLoadingCells(selection *worldlightmap.SurfaceSelection) []worldlightmap.LiquidTile {
	if len(selection.LiquidTiles) != 0 {
		return slices.Clone(selection.LiquidTiles)
	}

	return []worldlightmap.LiquidTile{{Clip: *selection.Clip, LiquidMaxSubdivisions: selection.LiquidMaxSubdivisions}}
}

func spatialLoadingFits(chunk *worldchunk.PlannedChunk, document *world.Document,
	options worldchunk.Options, ready *worldchunk.ReadyOptions,
) (bool, error) {
	descriptor, err := preflightSpatialGeometryDescriptor(chunk, false, document, nil, ready)
	if errors.Is(err, worldchunk.ErrReadyCapacity) {
		return false, nil
	}

	if err != nil {
		return false, err
	}
	// RNM stores three coefficient tiles. QOI's strict worst case is five
	// bytes per pixel; reserve the complete bounded lighting manifest and
	// envelope overhead as well, before accepting a larger loading unit.
	const envelopeOverhead = 64 << 10

	page := chunk.Layout.Pages[0]
	maximumLighting := uint64(page.Width)*uint64(page.Height)*15 + worldlightmap.MaxFixedLightingSize + envelopeOverhead

	return descriptor.MeshBytes <= options.MaxMeshBytes && descriptor.Size+maximumLighting <= options.MaxEncodedBytes, nil
}
