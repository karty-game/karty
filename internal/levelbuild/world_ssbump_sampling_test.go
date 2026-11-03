package levelbuild

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
)

func ssBumpTestTile(p color.NRGBA) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, 256, 256))

	for y := range 256 {
		for x := range 256 {
			result.SetNRGBA(x, y, p)
		}
	}

	return result
}

func TestWebGPUSSBumpChartsReprojectAndPreserveMaskedOcclusion(t *testing.T) {
	t.Parallel()

	document := world.Document{}

	baker, err := newWebGPUChartBaker(&document, webGPUChartTestPair(t), 4)
	if err != nil {
		t.Fatal(err)
	}

	baker.ssbump = &worldSSBumpBuild{tiles: map[uint32]*image.NRGBA{
		1: ssBumpTestTile(color.NRGBA{80, 30, 10, 255}),
		2: ssBumpTestTile(color.NRGBA{255, 255, 255, 255}),
		3: ssBumpTestTile(color.NRGBA{10, 20, 30, 255}),
	}}
	chart := webGPUChart{primary: 1, mapping: webGPUChartTestMapping(), normal: chartVector{0, 0, 1},
		points: []chartVector{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}},
		band:   &world.WallFrameRegion{Material: 3, UV: webGPUChartTestMapping(), RepeatU: true, RepeatV: true}}

	frame, err := chart.frame()
	if err != nil {
		t.Fatal(err)
	}

	weights, err := baker.sampleChartSSBump(chart, chartVector{.5, .5, 0}, frame)
	if err != nil {
		t.Fatal(err)
	}

	coverage := 128.0 / 255

	for i, base := range []float64{80, 30, 10} {
		band := []float64{10, 20, 30}[i]

		want := (base*(1-coverage) + band*coverage) / 255
		if math.Abs(weights[i]-want) > 1e-12 {
			t.Fatal("band lost directional coefficient energy", weights, want)
		}
	}

	angle := 2 * math.Pi / 3
	rotated := &world.SurfaceUV{Projections: []world.UVProjection{{
		U: world.UVPlane{X: math.Cos(angle), Y: math.Sin(angle)},
		V: world.UVPlane{X: -math.Sin(angle), Y: math.Cos(angle)},
	}}, Weights: []float64{1}}

	rotatedWeights, _, err := baker.sampleSSBump(1, rotated, chartVector{.5, .5, 0}, chart.normal, frame, nil)
	if err != nil {
		t.Fatal(err)
	}

	for i, want := range []float64{10, 80, 30} {
		if math.Abs(rotatedWeights[i]-want/255) > 1e-12 {
			t.Fatal("source basis not transferred", rotatedWeights)
		}
	}
}

func TestWebGPUSSBumpParticipatesInChartDeduplication(t *testing.T) {
	t.Parallel()

	document := world.Document{}

	baker, err := newWebGPUChartBaker(&document, webGPUChartTestPair(t), 4)
	if err != nil {
		t.Fatal(err)
	}

	baker.ssbump = &worldSSBumpBuild{tiles: map[uint32]*image.NRGBA{1: ssBumpTestTile(color.NRGBA{80, 30, 10, 255})}}
	chart := webGPUChart{
		primary: 1,
		mapping: webGPUChartTestMapping(),
		normal:  chartVector{0, 0, 1},
		points:  []chartVector{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}},
	}

	_, first, err := baker.bake(t.Context(), chart)
	if err != nil {
		t.Fatal(err)
	}

	baker.ssbump.tiles[1] = ssBumpTestTile(color.NRGBA{20, 10, 5, 255})

	_, second, err := baker.bake(t.Context(), chart)
	if err != nil {
		t.Fatal(err)
	}

	if baker.charts != 2 || first.Projections[0].U.Offset == second.Projections[0].U.Offset {
		t.Fatal("different directional visibility merged into one chart")
	}

	p := baker.tiles[0].ssbump
	if p.NRGBAAt(0, 0) != (color.NRGBA{80, 30, 10, 255}) || p.NRGBAAt(128, 0) != (color.NRGBA{20, 10, 5, 255}) {
		t.Fatal("SS chart gutters leaked", p.NRGBAAt(0, 0), p.NRGBAAt(128, 0))
	}
}
