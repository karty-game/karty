package levelbuild

import (
	"errors"
	"math"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
)

func spatialTestSettings() *spatialChunkSettings {
	return &spatialChunkSettings{TileSize: 32, BackgroundTileSize: 256, BackgroundDensity: .125,
		DetailBounds: []float64{-96, -96, 96, 96}, Solids: []string{"ocean"}}
}

func TestSpatialSurfaceTilesPreserveCompleteCoverageAndLocalDensity(t *testing.T) {
	t.Parallel()

	bounds := [4]float64{-1024, -1024, 1024, 1024}

	tiles, err := spatialSurfaceTiles(bounds, spatialTestSettings(), 4)
	if err != nil {
		t.Fatal(err)
	}

	var (
		area      float64
		near, far int
	)

	for index, tile := range tiles {
		tileBounds := tile.bounds
		area += (tileBounds[2] - tileBounds[0]) * (tileBounds[3] - tileBounds[1])

		switch tile.density {
		case 4:
			near++

			if tileBounds[2]-tileBounds[0] > 32 || tileBounds[3]-tileBounds[1] > 32 {
				t.Fatal("detailed receiver was not partitioned before packing", tileBounds)
			}
		case .125:
			far++
		default:
			t.Fatal("requested density changed", tile)
		}

		for _, other := range tiles[:index] {
			c := other.bounds
			if min(tileBounds[2], c[2]) > max(tileBounds[0], c[0]) && min(tileBounds[3], c[3]) > max(tileBounds[1], c[1]) {
				t.Fatal("tile interiors overlap", tileBounds, c)
			}
		}
	}

	if area != 2048*2048 || near != 36 || far == 0 || len(tiles) > 160 {
		t.Fatal("partition lost source area or expanded the distant ocean into thousands of tiny chunks", area, near, far, len(tiles))
	}
	// Locations from the reported bunker, pier and lighthouse must retain detail.
	for _, point := range [][2]float64{{-29.4, -6.15}, {11.4, -7.15}, {48, 48}} {
		owners := 0

		for _, tile := range tiles {
			tileBounds := tile.bounds
			if point[0] >= tileBounds[0] && point[0] < tileBounds[2] && point[1] >= tileBounds[1] && point[1] < tileBounds[3] {
				owners++

				if tile.density != 4 {
					t.Fatal("island receiver lost its requested texel density", point)
				}
			}
		}

		if owners != 1 {
			t.Fatal("receiver has missing or duplicated spatial ownership", point, owners)
		}
	}
}

func TestSpatialSurfaceTilesClipUnalignedEdgesAndBoundWork(t *testing.T) {
	t.Parallel()

	settings := spatialTestSettings()
	bounds := [4]float64{-37.5, -12.25, 70.125, 83.5}

	tiles, err := spatialSurfaceTiles(bounds, settings, 4)
	if err != nil {
		t.Fatal(err)
	}

	var area float64

	for _, tile := range tiles {
		b := tile.bounds
		if b[0] < bounds[0] || b[1] < bounds[1] || b[2] > bounds[2] || b[3] > bounds[3] {
			t.Fatal("spatial child escaped its source", b)
		}

		area += (b[2] - b[0]) * (b[3] - b[1])
	}

	if area != (bounds[2]-bounds[0])*(bounds[3]-bounds[1]) {
		t.Fatal("unaligned source edges were truncated", area)
	}

	settings.DetailBounds = []float64{-100000, -100000, 100000, 100000}
	if _, err := spatialSurfaceTiles([4]float64{-2048, -2048, 2048, 2048}, settings, 4); !errors.Is(err, ErrManifest) {
		t.Fatal("unbounded high-detail partition was accepted", err)
	}
}

func TestSpatialChunkSettingsRejectInvalidPolicies(t *testing.T) {
	t.Parallel()

	for name, mutate := range map[string]func(*spatialChunkSettings){
		"zero tile":                 func(s *spatialChunkSettings) { s.TileSize = 0 },
		"nan":                       func(s *spatialChunkSettings) { s.TileSize = math.NaN() },
		"nonhierarchical":           func(s *spatialChunkSettings) { s.BackgroundTileSize = 96 },
		"higher background density": func(s *spatialChunkSettings) { s.BackgroundDensity = 8 },
		"unordered bounds":          func(s *spatialChunkSettings) { s.DetailBounds[2] = -100 },
		"duplicate source":          func(s *spatialChunkSettings) { s.Solids = []string{"ocean", "ocean"} },
		"unsupported water cap":     func(s *spatialChunkSettings) { s.BackgroundLiquidSubdivisions = 33 },
		"unshared water edges": func(s *spatialChunkSettings) {
			s.BackgroundLiquidSubdivisions = 1
			s.GeometryDetailBounds = []float64{-96, -96, 96, 96}
		},
		"unaligned water grid": func(s *spatialChunkSettings) { s.LiquidEdgeStep = 3 },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			settings := spatialTestSettings()
			mutate(settings)

			if err := validateSpatialChunkSettings(settings, 4); !errors.Is(err, ErrManifest) {
				t.Fatal("invalid spatial policy was accepted", err)
			}
		})
	}
}

func TestSpatialSolidNamesRetainRootIdentities(t *testing.T) {
	t.Parallel()

	document := world.Document{StaticSolids: &world.StaticSolids{Items: []world.Solid{{ID: "pier"}, {ID: "ocean"}}}}

	indices, err := spatialSolidIndices(spatialTestSettings(), &document)
	if err != nil || len(indices) != 1 || indices[0] != 1 {
		t.Fatal("spatial delegation changed the source solid identity", indices, err)
	}

	if _, err := spatialSolidIndices(spatialTestSettings(), &world.Document{}); !errors.Is(err, ErrManifest) {
		t.Fatal("unknown source name was silently omitted", err)
	}
}
