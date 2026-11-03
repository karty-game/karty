// Command karty-ssbump prepares height-derived SSBump textures offline.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/karty-game/karty/internal/ssbump"
)

const maxEncodedBytes = 32 << 20

var ErrCommand = errors.New("invalid SSBump command")

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Stdout, os.Stderr)

	cancel()

	if err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "karty-ssbump:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, stdout, stderr io.Writer) error {
	options := ssbump.Defaults()
	flags := flag.NewFlagSet("karty-ssbump", flag.ContinueOnError)
	flags.SetOutput(stderr)
	inputPath := flags.String("height", "", "opaque height PNG (white is high, interpreted as linear data)")
	outputPath := flags.String("output", "", "output linear RGB SSBump PNG")
	flags.Float64Var(&options.Depth, "depth", options.Depth, "height range in input texels [0,64]")
	flags.Float64Var(&options.Radius, "radius", options.Radius, "periodic occlusion radius in input texels (0,128]")
	flags.IntVar(&options.Samples, "samples", options.Samples, "hemisphere rays per texel [8,128]")
	flags.IntVar(&options.Steps, "steps", options.Steps, "height tests per ray [1,64]")

	if err := flags.Parse(arguments); err != nil {
		return err
	}

	if *inputPath == "" || *outputPath == "" || flags.NArg() != 0 {
		return fmt.Errorf("%w: require --height input.png --output ssbump.png and no positional arguments", ErrCommand)
	}

	if err := distinctPaths(*inputPath, *outputPath); err != nil {
		return err
	}

	height, err := readPNG(*inputPath, options)
	if err != nil {
		return err
	}

	result, err := ssbump.Generate(ctx, height, options)
	if err != nil {
		return fmt.Errorf("generate SSBump: %w", err)
	}

	checksum, err := writePNG(ctx, *outputPath, result)
	if err != nil {
		return err
	}

	return json.NewEncoder(stdout).Encode(struct {
		Algorithm string         `json:"algorithm"`
		Width     int            `json:"width"`
		Height    int            `json:"height"`
		Options   ssbump.Options `json:"options"`
		SHA256    string         `json:"sha256"`
	}{ssbump.Algorithm, result.Bounds().Dx(), result.Bounds().Dy(), options, checksum})
}

func distinctPaths(input, output string) error {
	inputPath, err := filepath.Abs(input)
	if err != nil {
		return err
	}

	outputPath, err := filepath.Abs(output)
	if err != nil {
		return err
	}

	if inputPath == outputPath {
		return fmt.Errorf("%w: output must not replace the input height image", ErrCommand)
	}

	inputInfo, inputErr := os.Stat(inputPath)

	outputInfo, outputErr := os.Stat(outputPath)
	if inputErr == nil && outputErr == nil && os.SameFile(inputInfo, outputInfo) {
		return fmt.Errorf("%w: output must not alias the input height image", ErrCommand)
	}

	return nil
}

func readPNG(path string, options ssbump.Options) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open height PNG: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() || info.Size() > maxEncodedBytes {
		return nil, fmt.Errorf("%w: height PNG must be a regular file no larger than 32 MiB", ErrCommand)
	}

	configuration, err := png.DecodeConfig(io.LimitReader(file, maxEncodedBytes))
	if err != nil {
		return nil, fmt.Errorf("inspect height PNG: %w", err)
	}

	if err := options.Validate(image.Rect(0, 0, configuration.Width, configuration.Height)); err != nil {
		return nil, err
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	height, err := png.Decode(io.LimitReader(file, maxEncodedBytes))
	if err != nil {
		return nil, fmt.Errorf("decode height PNG: %w", err)
	}

	return height, nil
}

func writePNG(ctx context.Context, path string, pixels image.Image) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".ssbump-*.png")
	if err != nil {
		return "", fmt.Errorf("prepare SSBump output: %w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	hash := sha256.New()
	if err := png.Encode(io.MultiWriter(file, hash), pixels); err != nil {
		return "", fmt.Errorf("encode SSBump PNG: %w", err)
	}

	if err := file.Close(); err != nil {
		return "", err
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	if err := os.Rename(file.Name(), path); err != nil {
		return "", fmt.Errorf("publish SSBump PNG: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
