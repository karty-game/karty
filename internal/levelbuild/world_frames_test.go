package levelbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/pelletier/go-toml/v2"
)

func TestCompiledBandCoverageFastPaths(t *testing.T) {
	t.Parallel()

	vertices := []world.Vec2{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}
	mapping := func(offset float64) *world.SurfaceUV {
		return &world.SurfaceUV{
			Projections: []world.UVProjection{{U: world.UVPlane{X: 1, Offset: offset}, V: world.UVPlane{Z: 1}}},
			Weights:     []float64{1},
		}
	}
	wall := world.Wall{Material: 1, Start: world.Vec2{}, End: world.Vec2{X: 1}, FrameRegions: []world.WallFrameRegion{
		{Material: 2, Coverage: world.FrameCoverageMasked, Vertices: vertices, UV: mapping(0)},
		{Material: 3, Coverage: world.FrameCoverageMasked, Vertices: vertices, UV: mapping(0)},
		{Material: 4, Coverage: world.FrameCoverageMasked, Vertices: vertices, UV: mapping(1)},
	}}
	document := world.Document{Sectors: []world.Sector{{FloorMaterial: 1, CeilingMaterial: 1, Walls: []world.Wall{wall}}}}
	textures := []metadataTexture{
		{ID: 1, Name: "main"},
		{ID: 2, Name: "top", Coverage: "opaque"},
		{ID: 3, Name: "empty", Coverage: "empty"},
		{ID: 4, Name: "bottom", Coverage: "opaque"},
		{ID: 5, Name: "unused", Coverage: "masked"},
	}
	classifyCompiledFrames(&document, textures)

	regions := document.Sectors[0].Walls[0].FrameRegions
	if regions[0].Coverage != world.FrameCoverageOpaque || regions[1].Coverage != world.FrameCoverageMain || regions[1].Material != 1 ||
		regions[1].UV != nil ||
		regions[2].Coverage != world.FrameCoverageMasked {
		t.Fatal("coverage fast paths do not respect source alpha and UV domain")
	}
}

func TestWorldCameraCorridorUsesCompiledFrameRegions(t *testing.T) {
	t.Parallel()

	directory := filepath.Join("..", "..", "samples", "world-camera", "levels", "showcase")

	encoded, err := os.ReadFile(filepath.Join(directory, "level.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var definition manifest
	if err := toml.Unmarshal(encoded, &definition); err != nil {
		t.Fatal(err)
	}

	textures := make([]metadataTexture, 0, len(definition.Textures))
	for _, entry := range definition.Textures {
		textures = append(textures, metadataTexture{ID: uint32(len(textures) + 1), Name: entry.Name})
	}

	document, _, err := compileMaterialWorld(directory, definition, textures, nil)
	if err != nil {
		t.Fatal(err)
	}

	if document.MaterialLayers == nil || document.MaterialMapping == nil {
		t.Fatal("sample does not declare compiled frames")
	}

	trimmed := 0

	for _, sector := range document.Sectors {
		for _, wall := range sector.Walls {
			if wall.SourceEdge == "" && len(wall.FrameRegions) > 0 {
				t.Fatal("compiler subdivision received trim")
			}

			if len(wall.FrameRegions) == 0 {
				continue
			}

			trimmed++

			for _, region := range wall.FrameRegions {
				if region.Coverage != world.FrameCoverageMain && (region.UV == nil || len(region.UV.Projections) != 1) {
					t.Fatal("frame piece does not have compiled planar UV")
				}
			}

			if strings.HasPrefix(sector.Instance, "roman-court") {
				t.Fatal("corridor frame leaked into courtyard")
			}
		}
	}

	if trimmed < 16 {
		t.Fatalf("only %d corridor walls have frames", trimmed)
	}

	t.Logf("sample compiled %d sectors with %d framed authored walls", len(document.Sectors), trimmed)
}
