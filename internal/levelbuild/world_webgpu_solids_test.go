package levelbuild

import (
	"reflect"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
)

func TestWebGPUChartsCoverStaticAndReusableSlopedCaps(t *testing.T) {
	t.Parallel()

	blend := func() *world.SurfaceUV {
		return &world.SurfaceUV{
			Projections: []world.UVProjection{
				{U: world.UVPlane{Y: 1}, V: world.UVPlane{Z: 1}},
				{U: world.UVPlane{X: 1}, V: world.UVPlane{Z: 1}},
				{U: world.UVPlane{X: 1}, V: world.UVPlane{Y: 1}},
			}, Weights: []float64{.5, 0, .5}}
	}
	makeSolid := func(id string) world.Solid {
		return world.Solid{ID: id, Collision: id == "static",
			Footprint: []world.Vec2{{X: 0, Y: 0}, {X: 0, Y: 2}, {X: 2, Y: 2}, {X: 2, Y: 0}},
			Bottom:    world.Plane{A: 1}, Top: world.Plane{A: 1, C: 3}, SideMaterial: 1, TopMaterial: 1, BottomMaterial: 1,
			SideUV: webGPUChartTestMapping(), TopUV: blend(), BottomUV: blend()}
	}

	document := world.Document{StaticSolids: &world.StaticSolids{Items: []world.Solid{makeSolid("static")}},
		SolidRenderers: &world.SolidRenderers{Prefabs: []world.SolidPrefab{{ID: "prop", Solids: []world.Solid{makeSolid("local")}}}}}
	if !needsWebGPUCharts(&document) {
		t.Fatal("solid-only blends escaped chart detection")
	}

	baker, err := newWebGPUChartBaker(&document, webGPUChartTestPair(t), 4)
	if err != nil {
		t.Fatal(err)
	}

	if err := baker.convertSolids(t.Context()); err != nil {
		t.Fatal(err)
	}

	for _, solid := range []world.Solid{document.StaticSolids.Items[0], document.SolidRenderers.Prefabs[0].Solids[0]} {
		original := makeSolid(solid.ID)
		if !reflect.DeepEqual(solid.Footprint, original.Footprint) || solid.Bottom != original.Bottom || solid.Top != original.Top ||
			solid.Collision != original.Collision {
			t.Fatal("chart conversion changed physics geometry")
		}

		if len(solid.TopUV.Projections) != 1 || len(solid.BottomUV.Projections) != 1 || solid.TopMaterial < 4 || solid.BottomMaterial < 4 {
			t.Fatal("solid cap blend survived", solid.ID)
		}

		if !reflect.DeepEqual(solid.SideUV, original.SideUV) || solid.SideMaterial != original.SideMaterial {
			t.Fatal("ordinary side mapping changed")
		}
	}

	if needsWebGPUCharts(&document) {
		t.Fatal("converted cap still requires charts")
	}
}
