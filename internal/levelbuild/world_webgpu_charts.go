package levelbuild

import (
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
)

// Independent finite-footprint charts occupy each existing 256px tile: four
// 120px charts normally, or sixteen 56px charts to fit encoded level entry bounds.
// This is an explicit offline approximation, not source-texel equivalence.
// Extrusion prevents adjacent chart color bleeding.
const (
	webGPUChartGutter         = 4
	webGPUChartGutterExtent   = 2 * webGPUChartGutter
	webGPUChartTileSize       = 256
	webGPUChartDefaultGrid    = 2
	webGPUChartGridMultiplier = 2
	chartReflectanceChannel   = 2
	webGPUChartByteMax        = 255
	webGPUChartNormalMidpoint = 128
	webGPUChartNormalScale    = 127
	webGPUChartTexelCenter    = .5
	webGPUChartEpsilon        = 1e-12
	chartSRGBEncodedThreshold = .04045
	chartSRGBLinearThreshold  = .0031308
	chartSRGBLinearSlope      = 12.92
	chartSRGBOffset           = .055
	chartSRGBScale            = 1.055
	chartSRGBExponent         = 2.4
)

type chartVector [3]float64

type webGPUChart struct {
	primary uint32
	mapping *world.SurfaceUV
	band    *world.WallFrameRegion
	points  []chartVector
	normal  chartVector
}

type webGPUChartTile struct {
	id                        uint32
	albedo, data, reflectance *image.NRGBA
	ssbump                    *image.NRGBA
	masked                    bool
}

type webGPUChartSlot struct{ tile, cell int }

type webGPUChartBaker struct {
	document  *world.Document
	source    worldmaterial.DecodedPair
	materials map[uint32]worldmaterial.Rect
	tiles     []webGPUChartTile
	seen      map[[32]byte]webGPUChartSlot
	nextID    uint32
	ssbump    *worldSSBumpBuild
	grid      int
	charts    int
}

func newWebGPUChartBaker(document *world.Document, pair worldmaterial.Pair, nextID uint32) (*webGPUChartBaker, error) {
	if err := validateWebGPUSingleLayer(document); err != nil {
		return nil, err
	}

	decoded, err := pair.Decode()
	if err != nil {
		return nil, err
	}

	baker := &webGPUChartBaker{document: document, source: decoded, nextID: nextID, grid: webGPUChartDefaultGrid,
		materials: make(map[uint32]worldmaterial.Rect), seen: make(map[[32]byte]webGPUChartSlot)}
	for _, rect := range pair.Layout.Materials {
		baker.materials[rect.MaterialID] = rect
	}

	return baker, nil
}

func (b *webGPUChartBaker) bake(ctx context.Context, chart webGPUChart) (uint32, *world.SurfaceUV, error) {
	if err := b.checkEffects(chart); err != nil {
		return 0, nil, err
	}

	frame, err := chart.frame()
	if err != nil {
		return 0, nil, err
	}

	tile, err := b.rasterize(ctx, chart, frame)
	if err != nil {
		return 0, nil, err
	}

	slot, err := b.storeChart(tile)
	if err != nil {
		return 0, nil, err
	}

	return b.tiles[slot.tile].id, frame.mapping(slot.cell, b.grid), nil
}

func (b *webGPUChartBaker) rasterize(ctx context.Context, chart webGPUChart, frame chartFrame) (webGPUChartTile, error) {
	size := webGPUChartTileSize/b.grid - webGPUChartGutterExtent
	albedo, data, reflectance := image.NewNRGBA(image.Rect(0, 0, size, size)),
		image.NewNRGBA(image.Rect(0, 0, size, size)), image.NewNRGBA(image.Rect(0, 0, size, size))

	var coefficients *image.NRGBA
	if b.ssbump != nil {
		coefficients = image.NewNRGBA(image.Rect(0, 0, size, size))
	}

	masked := false

	for row := range size {
		if err := ctx.Err(); err != nil {
			return webGPUChartTile{}, err
		}

		for column := range size {
			point := frame.point(float64(column)/float64(size-1), float64(row)/float64(size-1))

			pixel, err := b.sampleChart(chart, point)
			if err != nil {
				return webGPUChartTile{}, err
			}

			normal := chartVector{
				chartDot(pixel.normal, frame.tangent),
				chartDot(pixel.normal, frame.bitangent),
				chartDot(pixel.normal, chart.normal),
			}
			a := color.NRGBA{chartSRGB(pixel.color[0]), chartSRGB(pixel.color[1]), chartSRGB(pixel.color[2]), chartByte(pixel.alpha)}
			albedo.SetNRGBA(column, row, a)
			masked = masked || a.A != webGPUChartByteMax

			data.SetNRGBA(
				column,
				row,
				color.NRGBA{chartNormalByte(normal[0]), chartNormalByte(normal[1]), chartByte(pixel.height), chartByte(pixel.ao)},
			)
			reflectance.SetNRGBA(
				column,
				row,
				color.NRGBA{chartByte(pixel.reflectance[0]), chartByte(pixel.reflectance[1]), 0, webGPUChartByteMax},
			)

			if coefficients != nil {
				weights, err := b.sampleChartSSBump(chart, point, frame)
				if err != nil {
					return webGPUChartTile{}, err
				}

				coefficients.SetNRGBA(
					column,
					row,
					color.NRGBA{chartByte(weights[0]), chartByte(weights[1]), chartByte(weights[2]), webGPUChartByteMax},
				)
			}
		}
	}

	return webGPUChartTile{albedo: albedo, data: data, reflectance: reflectance, ssbump: coefficients, masked: masked}, nil
}

func (b *webGPUChartBaker) storeChart(source webGPUChartTile) (webGPUChartSlot, error) {
	hash := sha256.New()
	hash.Write(source.albedo.Pix)
	hash.Write(source.data.Pix)
	hash.Write(source.reflectance.Pix)

	if source.ssbump != nil {
		hash.Write(source.ssbump.Pix)
	}

	key := [32]byte(hash.Sum(nil))
	if slot, exists := b.seen[key]; exists {
		return slot, nil
	}

	slot := webGPUChartSlot{tile: b.charts / (b.grid * b.grid), cell: b.charts % (b.grid * b.grid)}
	if slot.cell == 0 {
		if err := b.appendChartTile(source.ssbump != nil); err != nil {
			return webGPUChartSlot{}, err
		}
	}

	b.seen[key] = slot
	b.charts++

	tile := &b.tiles[slot.tile]
	tile.masked = tile.masked || source.masked

	if source.ssbump != nil {
		putWebGPUChart(tile.ssbump, source.ssbump, slot.cell, b.grid)
	}

	for index, destination := range []*image.NRGBA{tile.albedo, tile.data, tile.reflectance} {
		putWebGPUChart(destination, []*image.NRGBA{source.albedo, source.data, source.reflectance}[index], slot.cell, b.grid)
	}

	return slot, nil
}

func (b *webGPUChartBaker) appendChartTile(coefficients bool) error {
	if len(b.materials)+len(b.tiles) >= worldmaterial.MaxMaterials || b.nextID == math.MaxUint32 {
		return fmt.Errorf("WebGPU charts exceed %d material slots: %w", worldmaterial.MaxMaterials, ErrManifest)
	}

	tile := webGPUChartTile{
		id:     b.nextID,
		albedo: image.NewNRGBA(image.Rect(0, 0, webGPUChartTileSize, webGPUChartTileSize)),
		data: image.NewNRGBA(
			image.Rect(0, 0, webGPUChartTileSize, webGPUChartTileSize),
		),
		reflectance: image.NewNRGBA(image.Rect(0, 0, webGPUChartTileSize, webGPUChartTileSize)),
	}
	if coefficients {
		tile.ssbump = image.NewNRGBA(image.Rect(0, 0, webGPUChartTileSize, webGPUChartTileSize))
		for offset := 0; offset < len(tile.ssbump.Pix); offset += bakeTextureChannels {
			copy(tile.ssbump.Pix[offset:offset+bakeTextureChannels], []byte{85, 85, 85, webGPUChartByteMax})
		}
	}

	b.nextID++

	for offset := 0; offset < len(tile.albedo.Pix); offset += bakeTextureChannels {
		copy(tile.albedo.Pix[offset:offset+bakeTextureChannels], []byte{0, 0, 0, webGPUChartByteMax})
		copy(
			tile.data.Pix[offset:offset+bakeTextureChannels],
			[]byte{webGPUChartNormalMidpoint, webGPUChartNormalMidpoint, 0, webGPUChartByteMax},
		)
		copy(tile.reflectance.Pix[offset:offset+bakeTextureChannels], []byte{0, 0, 0, webGPUChartByteMax})
	}

	b.tiles = append(b.tiles, tile)

	return nil
}

func putWebGPUChart(destination, source *image.NRGBA, cell, grid int) {
	cellSize, size := webGPUChartTileSize/grid, source.Bounds().Dx()

	x, y := cell%grid*cellSize, cell/grid*cellSize
	for row := range cellSize {
		for column := range cellSize {
			destination.SetNRGBA(x+column, y+row, source.NRGBAAt(min(size-1, max(0, column-webGPUChartGutter)),
				min(size-1, max(0, row-webGPUChartGutter))))
		}
	}
}

func (b *webGPUChartBaker) checkEffects(chart webGPUChart) error {
	ids := []uint32{chart.primary}
	if chart.band != nil {
		ids = append(ids, chart.band.Material)
	}

	for _, materialID := range ids {
		if b.document.Animations != nil {
			for _, binding := range b.document.Animations.Materials {
				if binding.Material == materialID {
					return fmt.Errorf(
						"animated material %d needs independently moving UV charts; refusing to freeze it: %w",
						materialID,
						ErrManifest,
					)
				}
			}
		}

		if b.document.Emission != nil {
			for _, emission := range b.document.Emission.Materials {
				if emission.Material == materialID && emission.Intensity != 0 {
					return fmt.Errorf("emitting material %d needs an emission-aware chart policy: %w", materialID, ErrManifest)
				}
			}
		}
	}

	return nil
}
