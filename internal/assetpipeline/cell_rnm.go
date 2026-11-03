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
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

const (
	cellRNMProcessor = "world-cell-rnm-rgba8@1"
	cellRNMLayers    = 3
)

// EncodeCellRNM packages exactly one cell's three final planar RGBM lobes.
// No resampling, chart remapping, mip generation or global array is involved.
func EncodeCellRNM(ctx context.Context, root string, manifest sdk.Manifest, width, height uint32, pixels []byte) ([]byte, Artifact, error) {
	if width < 1 || height < 1 || width > 2048 || height > 2048 || uint64(len(pixels)) != uint64(width)*uint64(height)*12 {
		return nil, Artifact{}, ErrCacheRecipe
	}

	override := os.Getenv("KARTY_KTX")

	encoder, err := toolchain.EnsureKTX(ctx, manifest, override)
	if err != nil {
		return nil, Artifact{}, err
	}
	// Reuse the complete pinned executable/library identity, then select a
	// distinct lossless recipe instead of the material UASTC recipe/options.
	identity, err := materialKTX2Recipe(ctx, encoder, manifest, KTX2EncodeRequest{}, override == "")
	if err != nil {
		return nil, Artifact{}, err
	}

	recipe, err := NewRecipe("world-cell-rnm", cellRNMProcessor, identity.revisions, struct {
		Width, Height uint32
		Zstd, Threads int
	}{width, height, ktxZstdLevel, ktxEncoderThreads})
	if err != nil {
		return nil, Artifact{}, err
	}

	digest := sha256.Sum256(pixels)

	source, err := SnapshotBytes(digest[:])
	if err != nil {
		return nil, Artifact{}, err
	}

	cached, err := NewProjectCache(
		root,
	).Resolve(ctx, source, recipe, func(ctx context.Context, _ SourceSnapshot, _ Recipe) (Processed, error) {
		encoded, err := runCellRNMEncoder(ctx, encoder, width, height, pixels)

		return Processed{Encoding: cellRNMProcessor, Payload: encoded}, err
	})
	if err != nil {
		return nil, Artifact{}, err
	}

	encoded, err := readBoundedMaterialFile(cached.PayloadPath)
	if err != nil {
		return nil, Artifact{}, err
	}

	if cached.Encoding != cellRNMProcessor || validateCellRNM(encoded, width, height) != nil {
		return nil, Artifact{}, ErrCacheOutput
	}

	return encoded, cached, nil
}

func runCellRNMEncoder(ctx context.Context, encoder string, width, height uint32, pixels []byte) ([]byte, error) {
	directory, err := os.MkdirTemp("", "karty-cell-rnm-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)

	arguments := []string{
		"create",
		"--raw",
		"--format",
		"R8G8B8A8_UNORM",
		"--width",
		strconv.FormatUint(uint64(width), 10),
		"--height",
		strconv.FormatUint(uint64(height), 10),
		"--layers",
		"3",
		"--levels",
		"1",
		"--zstd",
		strconv.Itoa(ktxZstdLevel),
		"--threads",
		strconv.Itoa(ktxEncoderThreads),
	}
	layerBytes := len(pixels) / cellRNMLayers

	for layer := range 3 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		name := strconv.Itoa(layer) + ".raw"
		if err := os.WriteFile(filepath.Join(directory, name), pixels[layer*layerBytes:(layer+1)*layerBytes], 0o600); err != nil {
			return nil, err
		}

		arguments = append(arguments, name)
	}

	arguments = append(arguments, "cell.ktx2")
	// #nosec G204 G702 -- Explicit validated or SDK-pinned encoder; generated bounded per-cell inputs.
	command := exec.CommandContext(ctx, encoder, arguments...)
	command.Dir = directory
	command.Env = ktxEncoderEnvironment(os.Environ())

	var log materializeLog

	command.Stdout, command.Stderr = &log, &log
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		return nil, fmt.Errorf("KTX cell RNM encoder: %w\n%s", err, log.buffer.String())
	}

	encoded, err := readBoundedMaterialFile(filepath.Join(directory, "cell.ktx2"))
	if err != nil {
		return nil, err
	}

	if err := validateCellRNM(encoded, width, height); err != nil {
		return nil, err
	}

	return encoded, nil
}

func validateCellRNM(encoded []byte, width, height uint32) error {
	info, err := ktx2.InspectRGBA8(encoded)
	if err != nil || info.Width != width || info.Height != height || info.Layers != 3 || len(encoded) > level.MaxEntrySize {
		return ErrCacheOutput
	}

	return nil
}
