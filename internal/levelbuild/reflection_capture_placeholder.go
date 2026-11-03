package levelbuild

import (
	"context"
	"image"
	"slices"

	"github.com/karty-game/karty-sdk/bake/reflectionplan"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/worldreflection"
	"github.com/karty-game/karty/internal/assetpipeline"
)

// Capture alone uses a valid offline-built black array. The entry is excluded
// from InputDigest, so replacing it with captured pixels never rewrites cells.
// Ordinary builds reach only the checksum-bound saved-capture path.
func appendGPUReflectionCapturePlaceholder(ctx context.Context, module []byte, features []string,
	assets *assetBuild, chunks []ChunkArtifact,
) ([]byte, []string, error) {
	envelope, document, err := reflectionSource(module)
	if err != nil {
		return nil, nil, err
	}

	planned, err := reflectionplan.Plan(document, worldreflection.InputDigest(envelope))
	if err != nil {
		return nil, nil, err
	}

	if len(planned.Probes) == 0 {
		return module, features, nil
	}

	black := image.NewNRGBA(image.Rect(0, 0, worldreflection.FaceSize*worldreflection.Faces, worldreflection.FaceSize))
	for offset := 3; offset < len(black.Pix); offset += 4 {
		black.Pix[offset] = 255
	}

	encoded, _, err := qoi.Encode(black, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		return nil, nil, err
	}

	planned.Probes = slices.Clone(planned.Probes)
	for index := range planned.Probes {
		planned.Probes[index].Image = encoded
	}

	prepared, _, err := assetpipeline.ProcessReflectionGPU(ctx, assets.projectRoot, assets.manifest, planned)
	if err != nil {
		return nil, nil, err
	}

	prepared.DrawProbes, err = buildReflectionDrawProbes(ctx, envelope, chunks, planned)
	if err != nil {
		return nil, nil, err
	}

	data, err := worldreflection.EncodePrepared(prepared)
	if err != nil {
		return nil, nil, err
	}

	return appendReflectionResource(envelope, features, data,
		[]string{worldreflection.Feature, worldreflection.GPUFeature})
}
