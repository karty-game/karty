package assetpipeline

import (
	"image"
	"math"
	"slices"

	"github.com/karty-game/karty-sdk/format/worldmaterial"
)

const waterNormalSize = 64

func validateGPUWaterOverrides(layout worldmaterial.Layout, options []map[uint32]float64) (map[uint32]float64, error) {
	if len(options) > 1 {
		return nil, ErrCacheRecipe
	}

	if len(options) == 0 {
		return map[uint32]float64{}, nil
	}

	for id, amplitude := range options[0] {
		if math.IsNaN(amplitude) || math.IsInf(amplitude, 0) || amplitude < 0 || amplitude > 1 ||
			!slices.ContainsFunc(layout.Materials, func(rect worldmaterial.Rect) bool { return rect.MaterialID == id }) {
			return nil, ErrCacheRecipe
		}
	}

	return options[0], nil
}

// Preserve the former waterNormal(64x64) + nearest array expansion offline.
// This renderer-authored procedural normal intentionally replaces the water
// material's base normal, as the historical water shader did. B/A stay intact.
func prepareGPUWaterNormal(detail *image.NRGBA, amplitude float64) {
	var normals [waterNormalSize * waterNormalSize][2]byte
	for row := range waterNormalSize {
		for column := range waterNormalSize {
			u, v := float64(column)*2*math.Pi/waterNormalSize, float64(row)*2*math.Pi/waterNormalSize
			x, y := amplitude*(1.2*math.Sin(u+2*v)+.6*math.Cos(3*u-v)), amplitude*(1.2*math.Cos(2*u-v)+.6*math.Sin(u+3*v))
			unit := [3]float32{float32(x), float32(y), float32(math.Sqrt(max(0, 1-x*x-y*y)))}

			length := float32(math.Sqrt(float64(unit[0]*unit[0] + unit[1]*unit[1] + unit[2]*unit[2])))
			for axis := range unit {
				unit[axis] /= length
			}

			normals[row*waterNormalSize+column] = [2]byte{
				byte(math.Round(min(1, max(0, float64(unit[0])*.5+.5)) * 255)),
				byte(math.Round(min(1, max(0, float64(unit[1])*.5+.5)) * 255)),
			}
		}
	}

	for row := range detail.Rect.Dy() {
		for column := range detail.Rect.Dx() {
			normal := normals[row*waterNormalSize/detail.Rect.Dy()*waterNormalSize+column*waterNormalSize/detail.Rect.Dx()]
			offset := detail.PixOffset(column, row)
			detail.Pix[offset], detail.Pix[offset+1] = normal[0], normal[1]
		}
	}
}
