package assetpipeline

import (
	"context"
	"slices"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/worldreflection"
	"github.com/karty-game/karty/internal/sdk"
)

const (
	reflectionMipScale   = 2
	reflectionMipSamples = reflectionMipScale * reflectionMipScale
)

// ProcessReflectionGPU performs all face filtering and layer packing offline.
// The capture stays in the build cache; the returned payload contains one final
// GPU array, whose mip bytes need only decompression and upload at admission.
func ProcessReflectionGPU(ctx context.Context, root string, manifest sdk.Manifest,
	capture worldreflection.Bundle,
) (worldreflection.Prepared, Artifact, error) {
	if err := capture.Validate(); err != nil {
		return worldreflection.Prepared{}, Artifact{}, err
	}

	if len(capture.Probes) == 0 || uint64(len(capture.Probes))*worldreflection.GPUImageBytes > worldreflection.MaxDecodedBytes {
		return worldreflection.Prepared{}, Artifact{}, ErrAssetResourceLimits
	}

	request, err := prepareReflectionMips(ctx, capture)
	if err != nil {
		return worldreflection.Prepared{}, Artifact{}, err
	}

	encoded, artifact, err := encodeReflectionKTX2(ctx, root, manifest, request)
	if err != nil {
		return worldreflection.Prepared{}, Artifact{}, err
	}

	metadata := capture
	metadata.Producer = worldreflection.GPUProducer

	metadata.Probes = slices.Clone(capture.Probes)
	for index := range metadata.Probes {
		metadata.Probes[index].Image = nil
	}

	result := worldreflection.Prepared{Metadata: metadata, Texture: encoded}
	if err := result.Validate(); err != nil {
		return worldreflection.Prepared{}, Artifact{}, err
	}

	return result, artifact, nil
}

func prepareReflectionMips(ctx context.Context, capture worldreflection.Bundle) (KTX2EncodeRequest, error) {
	request := KTX2EncodeRequest{Width: worldreflection.FaceSize * worldreflection.Faces,
		Height: worldreflection.FaceSize, Layers: uint32(len(capture.Probes)), SRGB: true,
		Levels: make([][]byte, worldreflection.GPUMipLevels)}
	for level := range request.Levels {
		request.Levels[level] = make(
			[]byte,
			(int(request.Width)>>level)*(int(request.Height)>>level)*int(request.Layers)*materialPixelBytes,
		)
	}

	for layer, probe := range capture.Probes {
		if err := ctx.Err(); err != nil {
			return KTX2EncodeRequest{}, err
		}

		_, pixels, err := qoi.Decode(probe.Image)
		if err != nil {
			return KTX2EncodeRequest{}, err
		}

		copy(request.Levels[0][layer*worldreflection.ImageBytes:], pixels.Pix)

		for level := 1; level < worldreflection.GPUMipLevels; level++ {
			size := worldreflection.FaceSize >> (level - 1)
			previousBytes := size * size * worldreflection.Faces * materialPixelBytes
			currentBytes := (size / reflectionMipScale) * (size / reflectionMipScale) * worldreflection.Faces * materialPixelBytes
			source := request.Levels[level-1][layer*previousBytes : (layer+1)*previousBytes]
			target := request.Levels[level][layer*currentBytes : (layer+1)*currentBytes]
			downsampleReflectionFaces(source, target, size)
		}
	}

	return request, nil
}

func downsampleReflectionFaces(source, target []byte, size int) {
	var linear [256]float64
	for value := range linear {
		linear[value] = materialDecodeSRGB(byte(value))
	}

	for face := range worldreflection.Faces {
		for row := range size / reflectionMipScale {
			for column := range size / reflectionMipScale {
				var (
					rgb   [3]float64
					alpha uint32
				)

				for sample := range reflectionMipSamples {
					x, y := column*reflectionMipScale+sample%reflectionMipScale, row*reflectionMipScale+sample/reflectionMipScale

					offset := (y*size*worldreflection.Faces + face*size + x) * materialPixelBytes
					for channel := range rgb {
						rgb[channel] += linear[source[offset+channel]]
					}

					alpha += uint32(source[offset+3])
				}

				offset := (row*(size/reflectionMipScale)*worldreflection.Faces + face*(size/reflectionMipScale) + column) * materialPixelBytes
				for channel := range rgb {
					target[offset+channel] = materialEncodeSRGB(rgb[channel] / reflectionMipSamples)
				}

				target[offset+3] = byte((alpha + reflectionMipSamples/2) / reflectionMipSamples)
			}
		}
	}
}
