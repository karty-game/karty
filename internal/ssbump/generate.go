// Package ssbump prepares linear RGB self-shadowed RNM material weights offline.
// It has no renderer or SDK dependency; the cartridge integration is separate.
package ssbump

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
)

// Algorithm identifies the generation policy for future content-cache keys.
const Algorithm = "karty-ssbump-height-v1"

// MaterialAlgorithm keeps independently authored normals and occlusion strength
// separate from the height field used to trace self-shadowing.
const MaterialAlgorithm = "karty-ssbump-material-v2"

// ErrInput identifies unsupported input dimensions, data or generation controls.
var ErrInput = errors.New("invalid SSBump input")

const (
	// MaxDimension and MaxPixels bound decoded input and output allocations.
	MaxDimension = 1024
	MaxPixels    = 1024 * 1024
	maxWork      = 512 * 1024 * 1024
	opaque8      = 255
	opaque16     = 65535
	defaultDepth = 4
	defaultRange = 16
	defaultRays  = 32
	maxDepth     = 64
	maxRadius    = 128
	minSamples   = 8
	maxSamples   = 128
	maxSteps     = 64
)

// Options controls the heightfield approximation. Depth is the distance between
// height values 0 and 1 in input texels. Radius is a horizontal distance in input
// texels. Samples are cosine-weighted hemisphere rays and Steps are the maximum
// height tests per ray. Tile boundaries always wrap periodically.
type Options struct {
	Depth   float64 `json:"depth"`
	Radius  float64 `json:"radius"`
	Samples int     `json:"samples"`
	Steps   int     `json:"steps"`
}

// Defaults is a bounded starting quality, not a material's physical depth.
func Defaults() Options {
	return Options{Depth: defaultDepth, Radius: defaultRange, Samples: defaultRays, Steps: defaultRange}
}

// Validate rejects non-finite, excessive and empty workloads before allocation.
func (options Options) Validate(bounds image.Rectangle) error {
	width, height := bounds.Dx(), bounds.Dy()
	if width < 1 || height < 1 || width > MaxDimension || height > MaxDimension || width*height > MaxPixels {
		return fmt.Errorf("%w: height dimensions must be 1..%d with at most %d pixels", ErrInput, MaxDimension, MaxPixels)
	}

	if math.IsNaN(options.Depth) || math.IsInf(options.Depth, 0) || options.Depth < 0 || options.Depth > maxDepth {
		return fmt.Errorf("%w: depth must be finite and within [0,64] input texels", ErrInput)
	}

	if math.IsNaN(options.Radius) || math.IsInf(options.Radius, 0) || options.Radius <= 0 || options.Radius > maxRadius {
		return fmt.Errorf("%w: radius must be finite and within (0,128] input texels", ErrInput)
	}

	if options.Samples < minSamples || options.Samples > maxSamples || options.Steps < 1 || options.Steps > maxSteps {
		return fmt.Errorf("%w: samples must be 8..128 and steps must be 1..64", ErrInput)
	}

	if int64(width)*int64(height)*int64(options.Samples)*int64(options.Steps) > maxWork {
		return fmt.Errorf("%w: generation exceeds %d ray-step tests; reduce size or quality", ErrInput, maxWork)
	}

	return nil
}

// Generate returns 8-bit linear RGB RNM weights, with alpha 255. PNG columns
// are +tangent X, rows are +tangent Y, and height is +Z. RGB correspond to the
// canonical RNM directions (+sqrt(2/3),0,+sqrt(1/3)), (-sqrt(1/6),+sqrt(1/2),
// +sqrt(1/3)), (-sqrt(1/6),-sqrt(1/2),+sqrt(1/3)).
//
// The approximation multiplies normalized squared normal projections by a
// directional visibility estimate. Visibility comes from finite-radius rays
// against the periodic heightfield; it is not scalar AO repacked into RGB.
// Occluded output weights are deliberately not renormalized. A flat input is
// RGB (85,85,85). Color data is interpreted as grayscale height without sRGB
// conversion; opaque grayscale PNGs are the intended input.
func Generate(ctx context.Context, source image.Image, options Options) (*image.NRGBA, error) {
	return generateSurface(ctx, source, nil, options, 1, 1)
}

// GenerateMaterial uses linear encoded RG tangent normals for RNM projection,
// and the separate height field for directional visibility. Each strength is
// applied once offline; zero occlusion or depth preserves unshadowed normals.
func GenerateMaterial(ctx context.Context, source, normals image.Image, options Options,
	normalStrength, occlusionStrength float64,
) (*image.NRGBA, error) {
	if source == nil || normals == nil || source.Bounds() != normals.Bounds() {
		return nil, ErrInput
	}

	for _, strength := range []float64{normalStrength, occlusionStrength} {
		if math.IsNaN(strength) || math.IsInf(strength, 0) || strength < 0 || strength > 4 {
			return nil, ErrInput
		}
	}

	return generateSurface(ctx, source, normals, options, normalStrength, occlusionStrength)
}

func generateSurface(ctx context.Context, source, normals image.Image, options Options,
	normalStrength, occlusionStrength float64,
) (*image.NRGBA, error) {
	if source == nil {
		return nil, fmt.Errorf("%w: height image is nil", ErrInput)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	bounds := source.Bounds()
	if err := options.Validate(bounds); err != nil {
		return nil, err
	}

	field, err := readHeight(ctx, source, options.Depth)
	if err != nil {
		return nil, err
	}

	if field.constant {
		options.Depth = 0
	}

	output := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	rays := hemisphere(options.Samples)

	for row := range bounds.Dy() {
		for column := range bounds.Dx() {
			if column%16 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}

			var normal vector
			if normals != nil {
				normal, err = materialNormal(normals, column, row, normalStrength)
				if err != nil {
					return nil, err
				}
			} else {
				normal = field.normal(column, row)
			}

			weights := field.weightsForNormal(column, row, options, rays, normal, occlusionStrength)
			output.SetNRGBA(column, row, color.NRGBA{
				R: quantize(weights[0]), G: quantize(weights[1]), B: quantize(weights[2]), A: opaque8,
			})
		}
	}

	return output, nil
}

func quantize(value float64) uint8 {
	return uint8(math.Round(max(0, min(1, value)) * opaque8))
}

type heightfield struct {
	width, height int
	values        []float64
	constant      bool
}

func readHeight(ctx context.Context, source image.Image, depth float64) (heightfield, error) {
	bounds := source.Bounds()

	field := heightfield{width: bounds.Dx(), height: bounds.Dy(), values: make([]float64, bounds.Dx()*bounds.Dy()), constant: true}
	for row := range field.height {
		if err := ctx.Err(); err != nil {
			return heightfield{}, err
		}

		for column := range field.width {
			pixel := source.At(bounds.Min.X+column, bounds.Min.Y+row)

			_, _, _, alpha := pixel.RGBA()
			if alpha != opaque16 {
				return heightfield{}, fmt.Errorf("%w: height image must be opaque", ErrInput)
			}

			gray, _, _, _ := color.Gray16Model.Convert(pixel).RGBA()

			field.values[row*field.width+column] = float64(gray) / opaque16 * depth
			if field.values[row*field.width+column] != field.values[0] {
				field.constant = false
			}
		}
	}

	return field, nil
}

func wrap(value, size int) int {
	return ((value % size) + size) % size
}

func (field heightfield) at(column, row int) float64 {
	return field.values[wrap(row, field.height)*field.width+wrap(column, field.width)]
}

func (field heightfield) sample(column, row float64) float64 {
	x, y := math.Floor(column), math.Floor(row)
	u, v := column-x, row-y
	left := field.at(int(x), int(y))*(1-v) + field.at(int(x), int(y)+1)*v
	right := field.at(int(x)+1, int(y))*(1-v) + field.at(int(x)+1, int(y)+1)*v

	return left*(1-u) + right*u
}
