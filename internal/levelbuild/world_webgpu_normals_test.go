package levelbuild

import (
	"context"
	"errors"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/world"
)

func TestWebGPUChartRebasesBandNormalAndBakesAOOnce(t *testing.T) {
	t.Parallel()

	pair := webGPUChartTestPair(t)

	decoded, err := pair.Decode()
	if err != nil {
		t.Fatal(err)
	}

	rect := pair.Layout.Materials[2]
	for row := range rect.Height {
		for column := range rect.Width {
			decoded.Data().SetNRGBA(rect.X+column, rect.Y+row, color.NRGBA{R: 255, G: 128, A: 128})
		}
	}

	pair.Data, _, err = qoi.Encode(decoded.Data(), qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		t.Fatal(err)
	}

	document := world.Document{}

	baker, err := newWebGPUChartBaker(&document, pair, 4)
	if err != nil {
		t.Fatal(err)
	}

	rotated := &world.SurfaceUV{Projections: []world.UVProjection{{U: world.UVPlane{Y: -1}, V: world.UVPlane{X: 1}}}, Weights: []float64{1}}
	chart := webGPUChart{primary: 1, mapping: webGPUChartTestMapping(), normal: chartVector{0, 0, 1},
		points: []chartVector{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}},
		band:   &world.WallFrameRegion{Material: 3, UV: rotated, RepeatU: true, RepeatV: true}}

	_, mapping, err := baker.bake(t.Context(), chart)
	if err != nil {
		t.Fatal(err)
	}

	u, v := chartPlane(mapping.Projections[0].U, chartVector{.5, .5, 0}), chartPlane(mapping.Projections[0].V, chartVector{.5, .5, 0})
	pixel := baker.tiles[0].data.NRGBAAt(int(u*256), int(v*256))
	coverage := 128.0 / 255
	normalY := -coverage / math.Sqrt(coverage*coverage+(1-coverage)*(1-coverage))
	wantGreen := byte(math.Round(128 + 127*normalY))

	wantAO := byte(math.Round(((1 - coverage) + coverage*coverage) * 255))
	if pixel.R != 128 || pixel.G != wantGreen || pixel.A != wantAO {
		t.Fatalf("band normal stayed in source basis or AO changed: %v, want G=%d AO=%d", pixel, wantGreen, wantAO)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, _, err := baker.bake(ctx, chart); !errors.Is(err, context.Canceled) || baker.charts != 1 {
		t.Fatal("canceled chart work published a partial tile")
	}
}

func TestWebGPUChartMipsUseLinearLightAndNormalizedNormals(t *testing.T) {
	t.Parallel()

	image := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for row := range 2 {
		image.SetNRGBA(0, row, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		image.SetNRGBA(1, row, color.NRGBA{A: 255})
	}

	if pixel := downsampleWebGPUChart(image, 0).NRGBAAt(0, 0); pixel != (color.NRGBA{R: 188, G: 188, B: 188, A: 255}) {
		t.Fatalf("albedo mip averaged encoded bytes: %v", pixel)
	}

	for row := range 2 {
		image.SetNRGBA(0, row, color.NRGBA{R: 255, G: 128, B: 32, A: 255})
		image.SetNRGBA(1, row, color.NRGBA{R: 1, G: 128, B: 96, A: 0})
	}

	if pixel := downsampleWebGPUChart(image, 1).NRGBAAt(0, 0); pixel != (color.NRGBA{R: 128, G: 128, B: 64, A: 128}) {
		t.Fatalf("material mip did not normalize opposing normals or average scalar channels: %v", pixel)
	}
}
