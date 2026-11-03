package levelbuild

import (
	"fmt"
	"math"
	"slices"

	"github.com/karty-game/karty-sdk/format/world"
)

func validateSpatialChunkSettings(settings *spatialChunkSettings, density float64) error {
	if settings == nil {
		return nil
	}

	finite := func(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

	if err := validateSpatialLoadingSettings(settings); err != nil {
		return err
	}

	if err := validateSpatialLiquidSettings(settings); err != nil {
		return err
	}

	if settings.Density != 0 {
		if !finite(settings.Density) || settings.Density <= 0 || settings.Density > density {
			return fmt.Errorf("spatial density must be positive and no higher than lightmap density: %w", ErrManifest)
		}

		density = settings.Density
	}

	if !finite(settings.TileSize) || settings.TileSize < 1 || settings.TileSize > 1024 ||
		!finite(settings.BackgroundTileSize) || settings.BackgroundTileSize < settings.TileSize || settings.BackgroundTileSize > 4096 ||
		!finite(settings.BackgroundDensity) || settings.BackgroundDensity <= 0 || settings.BackgroundDensity > density ||
		len(settings.DetailBounds) != 4 || len(settings.Solids) == 0 {
		return fmt.Errorf("spatial chunks require bounded tile sizes, background density, detail bounds and named solids: %w", ErrManifest)
	}

	ratio := settings.BackgroundTileSize / settings.TileSize
	if ratio != math.Trunc(ratio) || math.Log2(ratio) != math.Trunc(math.Log2(ratio)) {
		return fmt.Errorf("spatial background_tile_size must be a power-of-two multiple of tile_size: %w", ErrManifest)
	}

	for _, value := range settings.DetailBounds {
		if !finite(value) || math.Abs(value) > 1e6 {
			return fmt.Errorf("spatial detail_bounds: %w", ErrManifest)
		}
	}

	if settings.DetailBounds[0] >= settings.DetailBounds[2] || settings.DetailBounds[1] >= settings.DetailBounds[3] {
		return fmt.Errorf("spatial detail_bounds must have positive area: %w", ErrManifest)
	}

	for index, name := range settings.Solids {
		if name == "" || slices.Contains(settings.Solids[:index], name) {
			return fmt.Errorf("spatial solids must contain unique source names: %w", ErrManifest)
		}
	}

	return nil
}

func validateSpatialLiquidSettings(settings *spatialChunkSettings) error {
	finite := func(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
	if !finite(settings.LiquidEdgeStep) || settings.LiquidEdgeStep < 0 || settings.LiquidEdgeStep > settings.TileSize ||
		settings.LiquidEdgeStep != 0 &&
			(math.Mod(settings.TileSize, settings.LiquidEdgeStep) != 0 || math.Mod(settings.BackgroundTileSize, settings.LiquidEdgeStep) != 0) {
		return fmt.Errorf("spatial liquid_edge_step must divide both tile sizes: %w", ErrManifest)
	}

	if settings.BackgroundLiquidSubdivisions != 0 && settings.LiquidEdgeStep == 0 {
		return fmt.Errorf("spatial liquid geometry limits require a common liquid_edge_step: %w", ErrManifest)
	}

	if settings.BackgroundLiquidSubdivisions < 0 || settings.BackgroundLiquidSubdivisions > 32 ||
		settings.BackgroundLiquidSubdivisions != 0 && len(settings.GeometryDetailBounds) != spatialBoundsCoordinates {
		return fmt.Errorf("spatial background liquid subdivisions require a 1..32 cap and geometry_detail_bounds: %w", ErrManifest)
	}

	if len(settings.GeometryDetailBounds) != 0 {
		if len(settings.GeometryDetailBounds) != spatialBoundsCoordinates {
			return ErrManifest
		}

		for _, value := range settings.GeometryDetailBounds {
			if !finite(value) || math.Abs(value) > 1e6 {
				return ErrManifest
			}
		}

		if settings.GeometryDetailBounds[0] >= settings.GeometryDetailBounds[2] ||
			settings.GeometryDetailBounds[1] >= settings.GeometryDetailBounds[3] {
			return ErrManifest
		}
	}

	return nil
}

// Tile every source point exactly once. The authored detail rectangle is an
// explicit quality policy, not a claim that lighting outside it is constant.
// Large distant tiles use small, lower-density atlases; nothing is discarded
// merely because it lies far from the initial camera.
//
//nolint:gocognit // Keep the bounded quadtree traversal and its exact coverage policy together.
func spatialSurfaceTiles(bounds [4]float64, settings *spatialChunkSettings, density float64) ([]spatialBuildTile, error) {
	if err := validateSpatialChunkSettings(settings, density); err != nil {
		return nil, err
	}

	if settings == nil || bounds[0] >= bounds[2] || bounds[1] >= bounds[3] {
		return nil, ErrManifest
	}

	if settings.Density != 0 {
		density = settings.Density
	}

	for _, value := range bounds {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e6 {
			return nil, ErrManifest
		}
	}

	step := settings.BackgroundTileSize
	minColumn, maxColumn := math.Floor(bounds[0]/step), math.Ceil(bounds[2]/step)

	minRow, maxRow := math.Floor(bounds[1]/step), math.Ceil(bounds[3]/step)
	if (maxColumn-minColumn)*(maxRow-minRow) > maxSpatialBuildTiles {
		return nil, fmt.Errorf("spatial partition exceeds %d tiles: %w", maxSpatialBuildTiles, ErrManifest)
	}

	var result []spatialBuildTile

	visits := 0

	var visit func([4]float64) error

	visit = func(tile [4]float64) error {
		visits++
		if visits > maxSpatialBuildTiles*8 {
			return fmt.Errorf("spatial partition exceeds bounded subdivision work: %w", ErrManifest)
		}

		if tile[0] >= bounds[2] || tile[1] >= bounds[3] || tile[2] <= bounds[0] || tile[3] <= bounds[1] {
			return nil
		}

		detail := tile[0] < settings.DetailBounds[2] && tile[2] > settings.DetailBounds[0] &&
			tile[1] < settings.DetailBounds[3] && tile[3] > settings.DetailBounds[1]
		if detail && tile[2]-tile[0] > settings.TileSize {
			x, y := (tile[0]+tile[2])*spatialHalf, (tile[1]+tile[3])*spatialHalf
			for _, child := range [][4]float64{{tile[0], tile[1], x, y}, {x, tile[1], tile[2], y},
				{tile[0], y, x, tile[3]}, {x, y, tile[2], tile[3]}} {
				if err := visit(child); err != nil {
					return err
				}
			}

			return nil
		}

		tile = [4]float64{max(tile[0], bounds[0]), max(tile[1], bounds[1]), min(tile[2], bounds[2]), min(tile[3], bounds[3])}
		if tile[0] >= tile[2] || tile[1] >= tile[3] {
			return nil
		}

		if len(result) == maxSpatialBuildTiles {
			return fmt.Errorf("spatial partition exceeds %d tiles: %w", maxSpatialBuildTiles, ErrManifest)
		}

		texels := settings.BackgroundDensity
		if detail {
			texels = density
		}

		result = append(result, spatialBuildTile{bounds: tile, density: texels})

		return nil
	}

	for y := minRow; y < maxRow; y++ {
		for x := minColumn; x < maxColumn; x++ {
			if err := visit([4]float64{x * step, y * step, (x + 1) * step, (y + 1) * step}); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

func spatialSolidIndices(settings *spatialChunkSettings, document *world.Document) ([]int, error) {
	if settings == nil {
		return nil, nil
	}

	var result []int

	for _, name := range settings.Solids {
		found := false

		if document.StaticSolids != nil {
			for index, solid := range document.StaticSolids.Items {
				if solid.ID == name {
					result = append(result, index)
					found = true

					break
				}
			}
		}

		if !found {
			return nil, fmt.Errorf("spatial source solid %q is unknown: %w", name, ErrManifest)
		}
	}

	slices.Sort(result)

	return result, nil
}

func spatialRectanglesOverlap(tile [4]float64, rectangle []float64) bool {
	return len(rectangle) == 4 && tile[0] < rectangle[2] && tile[2] > rectangle[0] && tile[1] < rectangle[3] && tile[3] > rectangle[1]
}

func spatialOnTileEdge(minimum, maximum, edge, outer float64) bool {
	if minimum != maximum {
		return false
	}

	return minimum == edge && minimum != outer
}
