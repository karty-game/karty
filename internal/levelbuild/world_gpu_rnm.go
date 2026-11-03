package levelbuild

import (
	"math"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

const (
	cellRGBMPixelBytes      = 4
	cellRGBMAtlasPixelBytes = 3 * cellRGBMPixelBytes
)

// cellRNMPixels performs the final atlas-to-planar transfer during the build.
// Constant charts keep their original atlas rectangles and geometry UVs.
func cellRNMPixels(layout worldlightmap.Layout, fixed worldlightmap.FixedLighting, encoded []byte) ([]byte, error) {
	if len(layout.Pages) != 1 {
		return nil, worldlightmap.ErrLayout
	}

	page := layout.Pages[0]
	if page.Width < 1 || page.Height < 1 || page.Width > 2048 || page.Height > 2048 {
		return nil, worldlightmap.ErrLayout
	}

	pixels := make([]byte, page.Width*page.Height*3*cellRGBMPixelBytes)

	if len(fixed.ConstantRNM) > 0 {
		if len(fixed.ConstantRNM) != len(layout.Charts) {
			return nil, worldlightmap.ErrLayout
		}

		for index, chart := range layout.Charts {
			if chart.Rect[0] < 0 || chart.Rect[1] < 0 || chart.Rect[2] > page.Width || chart.Rect[3] > page.Height {
				return nil, worldlightmap.ErrLayout
			}

			for layer := range 3 {
				value := cellEncodeRGBM(fixed.ConstantRNM[index].Coefficients[layer*3:layer*3+3], fixed.RGBMRange)

				for y := chart.Rect[1]; y < chart.Rect[3]; y++ {
					for x := chart.Rect[0]; x < chart.Rect[2]; x++ {
						copy(pixels[((layer*page.Height+y)*page.Width+x)*cellRGBMPixelBytes:], value[:])
					}
				}
			}
		}

		return pixels, nil
	}

	metadata, decoded, err := qoi.Decode(encoded)
	if err != nil || int(metadata.Width) != page.Width*3 || int(metadata.Height) != page.Height ||
		metadata.Colorspace != qoi.ColorspaceLinear ||
		metadata.Channels != qoi.ChannelsRGBA {
		return nil, worldlightmap.ErrLayout
	}

	rowBytes := page.Width * cellRGBMPixelBytes
	for layer := range 3 {
		for y := range page.Height {
			from := y*decoded.Stride + layer*rowBytes
			copy(pixels[(layer*page.Height+y)*rowBytes:], decoded.Pix[from:from+rowBytes])
		}
	}

	return pixels, nil
}

func cellEncodeRGBM(rgb []float32, span float64) [4]byte {
	scale := min(1, max(1.0/255, math.Ceil(max(float64(rgb[0]), float64(rgb[1]), float64(rgb[2]))/span*255)/255))

	return [4]byte{
		byte(math.Floor(min(1, max(0, float64(rgb[0])/(span*scale)))*255 + .5)),
		byte(math.Floor(min(1, max(0, float64(rgb[1])/(span*scale)))*255 + .5)),
		byte(math.Floor(min(1, max(0, float64(rgb[2])/(span*scale)))*255 + .5)),
		byte(math.Floor(scale*255 + .5)),
	}
}
