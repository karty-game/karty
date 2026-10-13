package assetpipeline

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"image/color"
	"image/png"
	"math"

	"github.com/karty-game/karty-sdk/format/worldsky"
)

const (
	skyColorMax        = 255
	skyPixelBytes      = 4
	skySRGBThreshold   = .04045
	skySRGBLinearScale = 12.92
	skySRGBOffset      = .055
	skySRGBScale       = 1.055
	skySRGBExponent    = 2.4
)

// CompileWorldSkyImage preserves authored PNG dimensions and alpha while
// converting sRGB into the renderer's linear, premultiplied RGBA8 representation.
// Dimensions are checked before decoding so compressed sources cannot bypass
// the public sky format's decoded-memory budget.
func CompileWorldSkyImage(input []byte) (*worldsky.Image, error) {
	_, animated, err := inspectPNG(input)
	if err != nil {
		return nil, fmt.Errorf("sky PNG structure: %w", err)
	}

	if animated {
		return nil, fmt.Errorf("sky APNG: %w", ErrAnimatedImage)
	}

	configuration, err := png.DecodeConfig(bytes.NewReader(input))
	if err != nil {
		return nil, fmt.Errorf("sky PNG configuration: %w", err)
	}

	if configuration.Width < 1 || configuration.Height < 1 || configuration.Width > worldsky.MaxDimension ||
		configuration.Height > worldsky.MaxDimension || configuration.Width > worldsky.MaxPixels/configuration.Height {
		return nil, fmt.Errorf("sky PNG dimensions: %w", worldsky.ErrBounds)
	}

	decoded, err := png.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, fmt.Errorf("sky PNG: %w", err)
	}

	var linear [256]float64
	for index := range linear {
		encoded := float64(index) / skyColorMax
		if encoded <= skySRGBThreshold {
			linear[index] = encoded / skySRGBLinearScale
		} else {
			linear[index] = math.Pow((encoded+skySRGBOffset)/skySRGBScale, skySRGBExponent)
		}
	}

	bounds := decoded.Bounds()
	pixels := make([]byte, bounds.Dx()*bounds.Dy()*skyPixelBytes)

	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			pixel, _ := color.NRGBAModel.Convert(decoded.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			alpha := float64(pixel.A)
			offset := (y*bounds.Dx() + x) * skyPixelBytes
			pixels[offset] = byte(math.Round(linear[pixel.R] * alpha))
			pixels[offset+1] = byte(math.Round(linear[pixel.G] * alpha))
			pixels[offset+2] = byte(math.Round(linear[pixel.B] * alpha))
			pixels[offset+3] = pixel.A
		}
	}

	var output bytes.Buffer

	writer, err := gzip.NewWriterLevel(&output, gzip.BestCompression)
	if err != nil {
		return nil, err
	}

	writer.OS = skyColorMax
	if _, err := writer.Write(pixels); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return &worldsky.Image{Width: bounds.Dx(), Height: bounds.Dy(), Packed: output.Bytes()}, nil
}
