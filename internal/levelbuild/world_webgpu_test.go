package levelbuild

import (
	"reflect"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
)

func TestWebGPUOneHotProjectionPreservesExactCoordinatesAndOwnership(t *testing.T) {
	t.Parallel()

	for selected := range 3 {
		source := &world.SurfaceUV{
			Projections: []world.UVProjection{
				{U: world.UVPlane{X: 2, Offset: 3}, V: world.UVPlane{Y: -4}},
				{U: world.UVPlane{Y: 5, Offset: -2}, V: world.UVPlane{Z: 3}},
				{U: world.UVPlane{X: -7}, V: world.UVPlane{Z: 2, Offset: 11}},
			},
			Weights: make([]float64, 3),
		}
		source.Weights[selected] = 1
		converted := source
		reduceWebGPUProjection(&converted)

		if len(converted.Projections) != 1 || converted.Projections[0] != source.Projections[selected] ||
			!reflect.DeepEqual(converted.Weights, []float64{1}) {
			t.Fatalf("projection %d lost its affine mapping", selected)
		}

		converted.Projections[0].U.Offset++
		if len(source.Projections) != 3 || source.Projections[selected].U.Offset == converted.Projections[0].U.Offset {
			t.Fatal("reduced mapping aliases its authoring source")
		}
	}
}

func TestWebGPUProjectionNeverDropsBlendedContribution(t *testing.T) {
	t.Parallel()

	for _, weights := range [][3]float64{{.5, .5, 0}, {1 - 1e-10, 1e-10, 0}, {0, .25, .75}, {1, 1, 0}} {
		source := &world.SurfaceUV{Projections: make([]world.UVProjection, 3), Weights: append([]float64(nil), weights[:]...)}
		converted := source
		reduceWebGPUProjection(&converted)

		if converted != source || !reflect.DeepEqual(converted.Weights, weights[:]) {
			t.Fatalf("dropped a weighted projection: %v", weights)
		}
	}
}

func TestWebGPUProjectionKeepsPortalAndCollisionTopology(t *testing.T) {
	t.Parallel()

	mapping := func() *world.SurfaceUV {
		return &world.SurfaceUV{Projections: make([]world.UVProjection, 3), Weights: []float64{0, 1, 0}}
	}
	document := world.Document{Sectors: []world.Sector{{FloorUV: mapping(), CeilingUV: mapping(), Walls: []world.Wall{
		{Start: world.Vec2{X: 3}, End: world.Vec2{Y: 7}, Portal: 2, PortalWall: 5, UV: mapping(),
			Secondary:    &world.SurfaceSecondary{Material: 9, Strength: .7, UV: mapping()},
			FrameRegions: []world.WallFrameRegion{{Material: 8, Coverage: world.FrameCoverageMasked, UV: mapping()}}},
	}}}}
	wall := document.Sectors[0].Walls[0]
	reduceWebGPUProjections(&document)

	converted := document.Sectors[0].Walls[0]
	if converted.Start != wall.Start ||
		converted.End != wall.End ||
		converted.Portal != wall.Portal ||
		converted.PortalWall != wall.PortalWall ||
		converted.Secondary.Material != 9 ||
		converted.Secondary.Strength != .7 ||
		converted.FrameRegions[0].Coverage != world.FrameCoverageMasked {
		t.Fatal("projection normalization changed geometry, links or visual layers")
	}

	if len(converted.UV.Projections) != 1 || len(converted.Secondary.UV.Projections) != 3 ||
		len(converted.FrameRegions[0].UV.Projections) != 1 || len(document.Sectors[0].FloorUV.Projections) != 1 {
		t.Fatal("primary/frame projection normalization changed an omitted secondary source")
	}
}
