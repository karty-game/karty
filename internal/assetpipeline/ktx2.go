package assetpipeline

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/karty-game/karty-sdk/codec/ktx2"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

const (
	materialGPUProcessor = "world-material-gpu-uastc@1"
	ktxUASTCQuality      = 4
	ktxZstdLevel         = 18
	ktxEncoderThreads    = 2
)

// KTX2EncodeRequest is the final GPU array layout. Levels contain all layers in
// layer order as tightly packed straight RGBA8; no encoder mip generation,
// normal-mode swizzle or alpha premultiplication is permitted.
type KTX2EncodeRequest struct {
	Width, Height, Layers uint32
	SRGB                  bool
	Levels                [][]byte
}

// EncodeMaterialKTX2 encodes one portable UASTC4x4+Zstd asset. The cache binds
// every prepared byte and the actual selected encoder executable. KARTY_KTX is
// an explicit candidate override; released SDKs install their pinned archive.
func EncodeMaterialKTX2(ctx context.Context, root string, manifest sdk.Manifest, request KTX2EncodeRequest) ([]byte, Artifact, error) {
	if err := validateKTX2Request(request); err != nil {
		return nil, Artifact{}, err
	}

	override := os.Getenv("KARTY_KTX")

	encoder, err := toolchain.EnsureKTX(ctx, manifest, override)
	if err != nil {
		return nil, Artifact{}, err
	}

	hash := sha256.New()
	for _, data := range request.Levels {
		hash.Write(data)
	}

	source, err := SnapshotBytes(hash.Sum(nil))
	if err != nil {
		return nil, Artifact{}, err
	}

	recipe, err := materialKTX2Recipe(ctx, encoder, manifest, request, override == "")
	if err != nil {
		return nil, Artifact{}, err
	}

	cached, err := NewProjectCache(
		root,
	).Resolve(ctx, source, recipe, func(ctx context.Context, _ SourceSnapshot, _ Recipe) (Processed, error) {
		payload, err := runKTX2Encoder(ctx, encoder, request)

		return Processed{Encoding: materialGPUProcessor, Payload: payload}, err
	})
	if err != nil {
		return nil, Artifact{}, err
	}

	encoded, err := readBoundedMaterialFile(cached.PayloadPath)
	if err != nil {
		return nil, Artifact{}, err
	}

	if cached.Encoding != materialGPUProcessor || validateEncodedKTX2(encoded, request) != nil {
		return nil, Artifact{}, ErrCacheOutput
	}

	return encoded, cached, nil
}

func validateKTX2Request(request KTX2EncodeRequest) error {
	if request.Width != 256 || request.Height != 256 || request.Layers < 1 || request.Layers > 196 || len(request.Levels) != 9 {
		return ErrCacheRecipe
	}

	for level, data := range request.Levels {
		size := max(uint64(request.Width)>>level, 1) * max(uint64(request.Height)>>level, 1) * uint64(request.Layers) * materialPixelBytes
		if uint64(len(data)) != size {
			return ErrCacheRecipe
		}
	}

	return nil
}

func validateEncodedKTX2(encoded []byte, request KTX2EncodeRequest) error {
	info, err := ktx2.Inspect(encoded)
	if err != nil || info.Width != request.Width || info.Height != request.Height || info.Layers != request.Layers ||
		info.SRGB != request.SRGB || len(info.Levels) != len(request.Levels) {
		return ErrCacheOutput
	}

	return nil
}

func runKTX2Encoder(ctx context.Context, encoder string, request KTX2EncodeRequest) ([]byte, error) {
	directory, err := os.MkdirTemp("", "karty-material-ktx2-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)

	arguments, err := writeKTX2Inputs(ctx, directory, request)
	if err != nil {
		return nil, err
	}

	// #nosec G204 G702 -- The encoder is an explicit validated executable or SDK-pinned tool; all arguments are generated from bounded prepared textures.
	command := exec.CommandContext(ctx, encoder, arguments...)
	command.Dir = directory
	command.Env = ktxEncoderEnvironment(os.Environ())

	var log materializeLog

	command.Stdout, command.Stderr = &log, &log
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		return nil, fmt.Errorf("KTX UASTC encoder: %w\n%s", err, log.buffer.String())
	}

	encoded, err := readBoundedMaterialFile(filepath.Join(directory, "material.ktx2"))
	if err != nil {
		return nil, err
	}

	if err := validateEncodedKTX2(encoded, request); err != nil {
		return nil, err
	}

	return encoded, nil
}

// The official encoder resolves its adjacent libktx through its own loader
// paths. Inherited loader injection must not select unrecorded library bytes.
func ktxEncoderEnvironment(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, variable := range environment {
		name, _, _ := strings.Cut(variable, "=")

		name = strings.ToUpper(name)
		if !strings.HasPrefix(name, "LD_") && !strings.HasPrefix(name, "DYLD_") {
			filtered = append(filtered, variable)
		}
	}

	return filtered
}

func writeKTX2Inputs(ctx context.Context, directory string, request KTX2EncodeRequest) ([]string, error) {
	format := "R8G8B8A8_UNORM"
	if request.SRGB {
		format = "R8G8B8A8_SRGB"
	}

	arguments := []string{"create", "--raw", "--format", format, "--width", strconv.FormatUint(uint64(request.Width), 10),
		"--height", strconv.FormatUint(uint64(request.Height), 10), "--layers", strconv.FormatUint(uint64(request.Layers), 10),
		"--levels", strconv.Itoa(len(request.Levels)), "--encode", "uastc", "--uastc-quality", strconv.Itoa(ktxUASTCQuality),
		"--zstd", strconv.Itoa(ktxZstdLevel), "--threads", strconv.Itoa(ktxEncoderThreads)}

	for level, data := range request.Levels {
		layerBytes := len(data) / int(request.Layers)
		for layer := range int(request.Layers) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			// Short deterministic names keep even the full 196-layer mip
			// chain below Windows command line length limits. KTX 4.4.2
			// does not support the input list files added in later versions.
			name := fmt.Sprintf("%04x", level*int(request.Layers)+layer)
			if err := os.WriteFile(filepath.Join(directory, name), data[layer*layerBytes:(layer+1)*layerBytes], 0o600); err != nil {
				return nil, err
			}

			arguments = append(arguments, name)
		}
	}

	return append(arguments, "material.ktx2"), nil
}
