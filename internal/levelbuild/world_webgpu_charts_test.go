package levelbuild

import (
	"context"
	"image"
	"image/color"
	"math"
	"reflect"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

func webGPUChartTestPair(t *testing.T) worldmaterial.Pair {
	t.Helper()

	layout, err := worldmaterial.NewLayoutV2([]uint32{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}

	layout.Materials[2].Coverage = worldmaterial.CoverageMasked

	albedo, data := image.NewNRGBA(
		image.Rect(0, 0, layout.Width, layout.Height),
	), image.NewNRGBA(
		image.Rect(0, 0, layout.Width, layout.Height),
	)
	for at := 3; at < len(albedo.Pix); at += 4 {
		albedo.Pix[at], data.Pix[at] = 255, 255
	}

	for index, rect := range layout.Materials {
		pixel := []color.NRGBA{{R: 200, G: 100, B: 50, A: 255}, {R: 188, G: 128, B: 64, A: 255}, {B: 255, A: 128}}[index]

		for row := range rect.Height {
			for column := range rect.Width {
				albedo.SetNRGBA(rect.X+column, rect.Y+row, pixel)
				data.SetNRGBA(rect.X+column, rect.Y+row, color.NRGBA{R: 128, G: 128, B: 32, A: 255})
			}
		}
	}

	encodedAlbedo, _, err := qoi.Encode(albedo, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		t.Fatal(err)
	}

	d, _, err := qoi.Encode(data, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		t.Fatal(err)
	}

	return worldmaterial.Pair{Layout: layout, Albedo: encodedAlbedo, Data: d}
}

func webGPUChartTestMapping() *world.SurfaceUV {
	return &world.SurfaceUV{Projections: []world.UVProjection{{U: world.UVPlane{X: 1}, V: world.UVPlane{Y: 1}}}, Weights: []float64{1}}
}

func TestWebGPUChartPrimaryAndBandSemanticsAndCellIsolation(t *testing.T) {
	t.Parallel()

	document := world.Document{}

	baker, err := newWebGPUChartBaker(&document, webGPUChartTestPair(t), 4)
	if err != nil {
		t.Fatal(err)
	}

	chart := webGPUChart{primary: 1, mapping: webGPUChartTestMapping(), normal: chartVector{0, 0, 1},
		points: []chartVector{{0, 0, 0}, {2, 0, 0}, {2, 2, 0}, {0, 2, 0}}}

	for cell := range 4 {
		setDenseChartTestColor(baker, baker.materials[1], byte(cell+1))

		id, mapping, err := baker.bake(t.Context(), chart)
		if err != nil || id != 4 {
			t.Fatalf("cell %d: material %d: %v", cell, id, err)
		}

		point := chartVector{1, 1, 0}
		u, v := chartPlane(mapping.Projections[0].U, point), chartPlane(mapping.Projections[0].V, point)

		pixel := baker.tiles[0].albedo.NRGBAAt(int(u*256), int(v*256))
		if pixel != (color.NRGBA{R: byte(cell + 1), G: 100, B: 50, A: 255}) {
			t.Fatalf("cell %d changed primary tile pixels: %v", cell, pixel)
		}

		if baker.tiles[0].albedo.NRGBAAt(cell%2*128, cell/2*128) != pixel {
			t.Fatal("constant chart border was not extruded through its gutter")
		}
	}

	if len(baker.tiles) != 1 || baker.charts != 4 {
		t.Fatal("independent charts did not share one bounded material tile")
	}

	chart.band = &world.WallFrameRegion{Material: 3, UV: webGPUChartTestMapping(), RepeatU: true, RepeatV: true}

	pixel, err := baker.sampleChart(chart, chartVector{.5, .5, 0})
	if err != nil {
		t.Fatal(err)
	}

	coverage := 128.0 / 255
	if math.Abs(pixel.color[2]-(chartLinear(50)*(1-coverage)+coverage)) > 1e-12 || pixel.alpha != 1 {
		t.Fatal("masked band did not composite in linear light over the opaque base")
	}
}

func webGPUChartTestBlend(mapping *world.SurfaceUV) *world.SurfaceUV {
	first := mapping.Projections[0]
	second := world.UVProjection{U: first.V, V: world.UVPlane{
		X: -first.U.X, Y: -first.U.Y, Z: -first.U.Z, Offset: -first.U.Offset,
	}}

	return &world.SurfaceUV{Projections: []world.UVProjection{first, second, first}, Weights: []float64{.5, .5, 0}}
}

func TestWebGPUChartPreparationPreservesTopologyAndSeparatesCache(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	document.Sectors[0].FloorMaterial = 1

	document.MaterialMapping = &world.MaterialMapping{Version: 1}
	for index := range document.Sectors {
		document.Sectors[index].FloorUV = webGPUChartTestMapping()

		document.Sectors[index].CeilingUV = webGPUChartTestMapping()
		for edge := range document.Sectors[index].Walls {
			document.Sectors[index].Walls[edge].UV = webGPUChartTestMapping()
		}
	}

	document.Sectors[0].FloorUV = webGPUChartTestBlend(webGPUChartTestMapping())

	document.Sectors[0].CeilingUV = webGPUChartTestMapping()
	for index := range document.Sectors[0].Walls {
		document.Sectors[0].Walls[index].UV = webGPUChartTestMapping()
	}

	walls := append([]world.Wall(nil), document.Sectors[0].Walls...)
	floor, ceiling := document.Sectors[0].Floor, document.Sectors[0].Ceiling
	pair := webGPUChartTestPair(t)
	calls := 0
	sourceKey := "ordinary-material-cache"
	assets := &assetBuild{
		projectRoot: t.TempDir(),
		materials: func(context.Context, string, sdk.Manifest, world.Document, map[uint32][]byte, ...map[uint32]assetpipeline.MaterializeOptions) (worldmaterial.Pair, assetpipeline.Artifact, error) {
			calls++

			return pair, assetpipeline.Artifact{CacheKey: sourceKey, CacheHit: true}, nil
		},
	}

	metadata, err := level.EncodeMetadata(level.Metadata{})
	if err != nil {
		t.Fatal(err)
	}

	owned, _, _, err := prepareWebGPUCharts(
		t.Context(),
		assets,
		&document,
		[]level.SourceEntry{
			{Name: level.TextureEntryName(1), Kind: level.EntryTexture, Data: pair.Albedo},
			{Name: level.TextureEntryName(2), Kind: level.EntryTexture, Data: pair.Albedo},
			{Name: level.TextureEntryName(3), Kind: level.EntryTexture, Data: pair.Albedo},
		},
		metadata,
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if owned == assets || calls != 1 || document.Sectors[0].Floor != floor || document.Sectors[0].Ceiling != ceiling ||
		!reflect.DeepEqual(document.Sectors[0].Walls, walls) {
		t.Fatal("chart preparation changed source topology or shared owner")
	}

	_, prepared, err := owned.materials(t.Context(), "", sdk.Manifest{}, document, nil)
	if err != nil || prepared.CacheKey == sourceKey || prepared.CacheHit {
		t.Fatal("prepared GPU chart reused the ordinary source cache identity", err)
	}

	if calls != 1 {
		t.Fatal("prepared material owner rebuilt source inputs")
	}
}
