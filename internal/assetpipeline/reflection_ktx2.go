package assetpipeline

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/karty-game/karty-sdk/codec/ktx2"
	"github.com/karty-game/karty-sdk/format/worldreflection"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

const reflectionGPUProcessor = "world-reflection-rgba8-mips@1"

func encodeReflectionKTX2(ctx context.Context, root string, manifest sdk.Manifest, request KTX2EncodeRequest) ([]byte, Artifact, error) {
	override := os.Getenv("KARTY_KTX")

	encoder, err := toolchain.EnsureKTX(ctx, manifest, override)
	if err != nil {
		return nil, Artifact{}, err
	}

	identity, err := materialKTX2Recipe(ctx, encoder, manifest, KTX2EncodeRequest{}, override == "")
	if err != nil {
		return nil, Artifact{}, err
	}

	recipe, err := NewRecipe("world-reflection", reflectionGPUProcessor, identity.revisions, struct {
		Width, Height, Layers uint32
		Levels, Zstd, Threads int
		Filter                string
	}{request.Width, request.Height, request.Layers, len(request.Levels), ktxZstdLevel, ktxEncoderThreads, "independent-linear-faces@1"})
	if err != nil {
		return nil, Artifact{}, err
	}

	hash := sha256.New()
	for _, level := range request.Levels {
		hash.Write(level)
	}

	source, err := SnapshotBytes(hash.Sum(nil))
	if err != nil {
		return nil, Artifact{}, err
	}

	artifact, err := NewProjectCache(
		root,
	).Resolve(ctx, source, recipe, func(ctx context.Context, _ SourceSnapshot, _ Recipe) (Processed, error) {
		data, err := runReflectionEncoder(ctx, encoder, request)

		return Processed{Encoding: reflectionGPUProcessor, Payload: data}, err
	})
	if err != nil {
		return nil, Artifact{}, err
	}

	data, err := readBoundedMaterialFile(artifact.PayloadPath)
	if err != nil {
		return nil, Artifact{}, err
	}

	if artifact.Encoding != reflectionGPUProcessor || validateReflectionKTX2(data, request.Layers) != nil {
		return nil, Artifact{}, ErrCacheOutput
	}

	return data, artifact, nil
}

func runReflectionEncoder(ctx context.Context, encoder string, request KTX2EncodeRequest) ([]byte, error) {
	directory, err := os.MkdirTemp("", "karty-reflection-ktx2-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)

	arguments := []string{"create", "--raw", "--format", "R8G8B8A8_SRGB", "--width", strconv.FormatUint(uint64(request.Width), 10),
		"--height", strconv.FormatUint(uint64(request.Height), 10), "--layers", strconv.FormatUint(uint64(request.Layers), 10),
		"--levels", strconv.Itoa(len(request.Levels)), "--zstd", strconv.Itoa(ktxZstdLevel), "--threads", strconv.Itoa(ktxEncoderThreads)}
	for level, pixels := range request.Levels {
		layerBytes := len(pixels) / int(request.Layers)
		for layer := range int(request.Layers) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			name := fmt.Sprintf("%04x", level*int(request.Layers)+layer)
			if err := os.WriteFile(filepath.Join(directory, name), pixels[layer*layerBytes:(layer+1)*layerBytes], 0o600); err != nil {
				return nil, err
			}

			arguments = append(arguments, name)
		}
	}

	arguments = append(arguments, "reflections.ktx2")
	// #nosec G204 G702 -- SDK-pinned encoder and generated bounded raw inputs.
	command := exec.CommandContext(ctx, encoder, arguments...)
	command.Dir, command.Env = directory, ktxEncoderEnvironment(os.Environ())

	var log materializeLog

	command.Stdout, command.Stderr = &log, &log
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		return nil, fmt.Errorf("KTX reflection encoder: %w\n%s", err, log.buffer.String())
	}

	data, err := readBoundedMaterialFile(filepath.Join(directory, "reflections.ktx2"))
	if err != nil {
		return nil, err
	}

	if err := validateReflectionKTX2(data, request.Layers); err != nil {
		return nil, err
	}

	return data, nil
}

func validateReflectionKTX2(data []byte, layers uint32) error {
	info, err := ktx2.InspectRGBA8Array(data)
	if err != nil || !info.SRGB || info.Width != worldreflection.FaceSize*worldreflection.Faces ||
		info.Height != worldreflection.FaceSize || info.Layers != layers || len(info.Levels) != worldreflection.GPUMipLevels ||
		info.RGBABytes > worldreflection.MaxDecodedBytes {
		return ErrCacheOutput
	}

	return nil
}
