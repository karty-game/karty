package levelbuild

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldterrain"
)

// Spatial is an opt-in render partition. Gameplay sector and fluid identities
// remain in the root document; source names identify render surfaces only.
type spatialChunkSettings struct {
	Density                      float64   `toml:"density"`
	TileSize                     float64   `toml:"tile_size"`
	LoadTileSize                 float64   `toml:"load_tile_size"`
	BackgroundTileSize           float64   `toml:"background_tile_size"`
	BackgroundDensity            float64   `toml:"background_density"`
	BackgroundLiquidSubdivisions int       `toml:"background_liquid_subdivisions"`
	LiquidEdgeStep               float64   `toml:"liquid_edge_step"`
	GeometryDetailBounds         []float64 `toml:"geometry_detail_bounds"`
	DetailBounds                 []float64 `toml:"detail_bounds"`
	Solids                       []string  `toml:"solids"`
}

const (
	maxSpatialBuildTiles     = 4096
	spatialAreaEpsilon       = 1e-8
	spatialBoundsCoordinates = 4
	spatialHalf              = .5
)

type spatialBuildTile struct {
	bounds  [4]float64
	density float64
}

type spatialWorldChunks struct {
	surfaces []worldlightmap.Surface
	kinds    []string
	primary  []worldlightmap.Binding
}

func prepareSpatialWorldChunks(definition manifest, document *world.Document, terrain *worldterrain.Document) (*spatialWorldChunks, error) {
	if definition.Chunks.Spatial == nil {
		//nolint:nilnil // An absent spatial policy intentionally has no prepared partition.
		return nil, nil
	}

	settings := definition.Chunks.Spatial

	indices, err := spatialSolidIndices(settings, document)
	if err != nil {
		return nil, err
	}

	surfaces, err := worldlightmap.Surfaces(document)
	if err != nil {
		return nil, err
	}

	result := &spatialWorldChunks{surfaces: surfaces, kinds: make([]string, len(surfaces))}
	for index, surface := range surfaces {
		binding := surface.Binding
		binding.Chart = -1

		kind := "primary"
		if binding.Kind == "sector-floor" && terrain != nil && slices.Contains(terrain.ExteriorSectors, uint16(binding.Index)) {
			kind = "terrain"
		} else if len(surface.Polygons) != 0 {
			bounds := spatialSurfaceBounds(surface)
			if slices.Contains(indices, binding.Index) && len(binding.Kind) > 6 && binding.Kind[:6] == "solid-" ||
				bounds[2]-bounds[0] > settings.TileSize || bounds[3]-bounds[1] > settings.TileSize {
				kind = "spatial"
			}
		}

		if kind == "spatial" && settings.LiquidEdgeStep == 0 && spatialDisplacedWater(surface, document) {
			return nil, fmt.Errorf("spatial animated water requires a shared liquid_edge_step before baking: %w", ErrManifest)
		}

		result.kinds[index] = kind
		if kind == "primary" {
			result.primary = append(result.primary, binding)
		}
	}

	return result, nil
}

func spatialDisplacedWater(surface worldlightmap.Surface, document *world.Document) bool {
	if document.Animations == nil || math.Abs(surface.Normal.X) >= 1e-9 || math.Abs(surface.Normal.Y) >= 1e-9 ||
		surface.Normal.Z <= .999999 {
		return false
	}

	var material uint32

	switch surface.Binding.Kind {
	case "sector-floor":
		material = document.Sectors[surface.Binding.Index].FloorMaterial
	case "solid-top":
		material = document.StaticSolids.Items[surface.Binding.Index].TopMaterial
	default:
		return false
	}

	for _, binding := range document.Animations.Materials {
		if binding.Material != material {
			continue
		}

		for _, preset := range document.Animations.Presets {
			if preset.Name == binding.Preset && preset.Kind == "liquid" && preset.SurfaceAmplitude > 0 {
				return true
			}
		}
	}

	return false
}

func spatialSurfaceBounds(surface worldlightmap.Surface) [4]float64 {
	bounds := [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}

	for _, polygon := range surface.Polygons {
		for _, point := range polygon {
			bounds[0], bounds[1], bounds[2], bounds[3] = min(
				bounds[0],
				point.X,
			), min(
				bounds[1],
				point.Y,
			), max(
				bounds[2],
				point.X,
			), max(
				bounds[3],
				point.Y,
			)
		}
	}

	return bounds
}

func compileSpatialLeaf(
	document *world.Document,
	parent worldchunk.PlannedChunk,
	options worldlightmap.Options,
	selection worldlightmap.SurfaceSelection,
	tile spatialBuildTile,
) (worldchunk.PlannedChunk, error) {
	return compileSpatialLeafWithMinimum(document, parent, options, selection, tile, 32)
}

func compileSpatialLeafWithMinimum(
	document *world.Document,
	parent worldchunk.PlannedChunk,
	options worldlightmap.Options,
	selection worldlightmap.SurfaceSelection,
	tile spatialBuildTile,
	minimumPage int,
) (worldchunk.PlannedChunk, error) {
	var err error

	layoutOptions := options
	layoutOptions.TexelsPerUnit = tile.density

	var layout worldlightmap.Layout

	for _, page := range []int{32, 64, 128, 256, 512, 1024} {
		if page < minimumPage {
			continue
		}

		if page > options.PageSize {
			break
		}

		layoutOptions.PageSize = page

		layout, err = worldlightmap.CompileSelectedReceivers(*document, layoutOptions, parent.Sectors, parent.Solids, selection)
		if err == nil {
			break
		}

		if !errors.Is(err, worldlightmap.ErrPageCapacity) {
			return worldchunk.PlannedChunk{}, err
		}
	}

	if err != nil {
		return worldchunk.PlannedChunk{}, err
	}

	mesh, err := worldchunk.CompileMesh(*document, parent.Sectors, parent.Solids, layout)
	if err != nil {
		return worldchunk.PlannedChunk{}, err
	}

	return worldchunk.PlannedChunk{Sectors: parent.Sectors, Solids: parent.Solids,
		Mesh: mesh, Layout: layout, Bounds: mesh.Bounds, MeshBytes: worldchunk.DecodedMeshBytes(mesh),
		LightmapBytes: uint64(layout.Pages[0].Width) * uint64(layout.Pages[0].Height) * bakeTextureChannels}, nil
}

func spatialSurfaceInTile(surface worldlightmap.Surface, tile, whole [4]float64) bool {
	b := spatialSurfaceBounds(surface)
	// A vertical face exactly on a grid edge has one owner, including at the
	// outermost source edge. Inclusive polygon clipping alone would duplicate it.
	if spatialOnTileEdge(b[0], b[2], tile[2], whole[2]) || spatialOnTileEdge(b[1], b[3], tile[3], whole[3]) {
		return false
	}

	for _, polygon := range surface.Polygons {
		clipped := worldlightmap.ClipSurfacePolygon(polygon, tile)
		if spatialPolygonArea(clipped) > spatialAreaEpsilon {
			return true
		}
	}

	return false
}

func spatialPolygonArea(points []world.Vec3) float64 {
	var area float64

	for index := 1; index+1 < len(points); index++ {
		a, b, c := points[0], points[index], points[index+1]
		ux, uy, uz, vx, vy, vz := b.X-a.X, b.Y-a.Y, b.Z-a.Z, c.X-a.X, c.Y-a.Y, c.Z-a.Z
		area += math.Sqrt((uy*vz-uz*vy)*(uy*vz-uz*vy)+(uz*vx-ux*vz)*(uz*vx-ux*vz)+(ux*vy-uy*vx)*(ux*vy-uy*vx)) * spatialHalf
	}

	return area
}
