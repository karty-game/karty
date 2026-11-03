package assetpipeline

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
	"github.com/karty-game/karty/internal/sdk"
)

// PrepareWorldMaterialGPU builds the final GPU arrays before compression.
// SSBump tiles must already be generated independently and reprojected for any
// material charts. No rays are traced across packed chart boundaries here.
func PrepareWorldMaterialGPU(
	ctx context.Context,
	pair worldmaterial.Pair,
	tiles map[uint32]*image.NRGBA,
	waterOverrides ...map[uint32]float64,
) ([3]KTX2EncodeRequest, error) {
	var arrays [3]KTX2EncodeRequest
	if err := ctx.Err(); err != nil {
		return arrays, err
	}

	decoded, err := pair.Decode()
	if err != nil {
		return arrays, err
	}

	water, err := validateGPUWaterOverrides(pair.Layout, waterOverrides)
	if err != nil {
		return arrays, err
	}

	count := len(pair.Layout.Materials)
	if count == 0 || len(tiles) != count {
		return arrays, ErrCacheOutput
	}

	const size = worldmaterial.TileSize

	var total uint64

	for level := 0; level <= worldmaterial.MipLevels; level++ {
		dimension := max(size>>level, 1)
		total += uint64(dimension*dimension) * uint64(count) * materialPixelBytes * uint64(len(arrays))
	}

	if total > asset.MaxDecodedTextures {
		return arrays, ErrAssetResourceLimits
	}

	for index := range arrays {
		arrays[index] = KTX2EncodeRequest{Width: size, Height: size, Layers: uint32(count), SRGB: index == 0,
			Levels: make([][]byte, worldmaterial.MipLevels+1)}
		for level := range arrays[index].Levels {
			dimension := max(size>>level, 1)
			arrays[index].Levels[level] = make([]byte, dimension*dimension*count*materialPixelBytes)
		}
	}

	for layer, rect := range pair.Layout.Materials {
		if err := ctx.Err(); err != nil {
			return [3]KTX2EncodeRequest{}, err
		}

		coefficients := tiles[rect.MaterialID]
		if coefficients == nil || coefficients.Bounds() != image.Rect(0, 0, size, size) {
			return [3]KTX2EncodeRequest{}, fmt.Errorf("material %d SSBump tile: %w", rect.MaterialID, ErrCacheOutput)
		}

		albedo := materialInterior(decoded.Albedo(), rect)
		detail := materialInterior(decoded.Data(), rect)
		prepareGPUDetail(detail, decoded.Reflectance(), rect)

		if amplitude, found := water[rect.MaterialID]; found {
			prepareGPUWaterNormal(detail, amplitude)
		}

		if err := copyGPUMaterialLayer(ctx, &arrays, layer, [3]*image.NRGBA{albedo, coefficients, detail}); err != nil {
			return [3]KTX2EncodeRequest{}, err
		}
	}

	return arrays, nil
}

func copyGPUMaterialLayer(ctx context.Context, arrays *[3]KTX2EncodeRequest, layer int, images [3]*image.NRGBA) error {
	for level := 0; level <= worldmaterial.MipLevels; level++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		for index, pixels := range images {
			layerBytes := pixels.Rect.Dx() * pixels.Rect.Dy() * materialPixelBytes

			destination := arrays[index].Levels[level][layer*layerBytes : (layer+1)*layerBytes]
			for row := range pixels.Rect.Dy() {
				copy(
					destination[row*pixels.Rect.Dx()*materialPixelBytes:],
					pixels.Pix[row*pixels.Stride:row*pixels.Stride+pixels.Rect.Dx()*materialPixelBytes],
				)
			}
		}

		if level != worldmaterial.MipLevels {
			images[0], images[2] = downsampleMaterial(images[0], images[2])
			for offset := 3; offset < len(images[0].Pix); offset += materialPixelBytes {
				if images[0].Pix[offset] == 0 {
					// The shared filter's legacy neutral AO becomes zero smoothness.
					images[2].Pix[offset] = 0
				}
			}

			images[1] = downsampleGPUResponse(images[1])
		}
	}

	return nil
}

func prepareGPUDetail(detail, reflectance *image.NRGBA, rect worldmaterial.Rect) {
	strength := 1.0
	if rect.Strengths != nil {
		strength = rect.Strengths.Normal
	}

	for row := range detail.Rect.Dy() {
		for column := range detail.Rect.Dx() {
			offset := detail.PixOffset(column, row)
			x := min(max((float64(detail.Pix[offset])-materialNeutral)/materialMidpoint, -1), 1) * strength
			y := min(max((float64(detail.Pix[offset+1])-materialNeutral)/materialMidpoint, -1), 1) * strength
			length := math.Sqrt(max(1, x*x+y*y))
			detail.Pix[offset], detail.Pix[offset+1] = materialEncodeNormal(x/length), materialEncodeNormal(y/length)
			detail.Pix[offset+2], detail.Pix[offset+3] = 0, 0

			if reflectance != nil {
				value := reflectance.NRGBAAt(rect.X+column, rect.Y+row)
				detail.Pix[offset+2], detail.Pix[offset+3] = value.R, value.G
			}
		}
	}
}

// Response and emission are scalar linear data. In particular, coefficients
// must never be treated as unit normals or normalized after filtering.
func downsampleGPUResponse(source *image.NRGBA) *image.NRGBA {
	size := source.Rect.Dx() / materialMipScale

	result := image.NewNRGBA(image.Rect(0, 0, size, size))
	for row := range size {
		for column := range size {
			var sum [4]uint32

			for sample := range materialMipSamples {
				offset := source.PixOffset(column*2+sample%2, row*2+sample/2)
				for channel := range sum {
					sum[channel] += uint32(source.Pix[offset+channel])
				}
			}

			offset := result.PixOffset(column, row)
			for channel := range sum {
				result.Pix[offset+channel] = byte((sum[channel] + materialMipSamples/2) / materialMipSamples)
			}
		}
	}

	return result
}

// ProcessWorldMaterialGPU returns three completely validated portable assets.
// Prepared channel bytes are cached independently by exact encoder and input.
func ProcessWorldMaterialGPU(ctx context.Context, root string, manifest sdk.Manifest, base worldmaterial.Pair,
	tiles map[uint32]*image.NRGBA, waterOverrides ...map[uint32]float64,
) (worldmaterialgpu.Pair, []Artifact, error) {
	arrays, err := PrepareWorldMaterialGPU(ctx, base, tiles, waterOverrides...)
	if err != nil {
		return worldmaterialgpu.Pair{}, nil, err
	}

	descriptor, err := worldmaterialgpu.NewDescriptor(base.Layout)
	if err != nil {
		return worldmaterialgpu.Pair{}, nil, err
	}

	result := worldmaterialgpu.Pair{Descriptor: descriptor}

	artifacts := make([]Artifact, 0, len(arrays))
	for index, target := range []*[]byte{&result.Albedo, &result.SSBump, &result.Detail} {
		encoded, cached, err := EncodeMaterialKTX2(ctx, root, manifest, arrays[index])
		if err != nil {
			return worldmaterialgpu.Pair{}, nil, err
		}

		*target = encoded

		artifacts = append(artifacts, cached)
	}

	if err := result.Validate(base.Layout); err != nil {
		return worldmaterialgpu.Pair{}, nil, err
	}

	return result, artifacts, nil
}
