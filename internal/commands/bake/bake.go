// Package bake defines the offline directional lightmap command.
package bake

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/ui"
	"github.com/urfave/cli/v3"
)

var (
	ErrSDK     = errors.New("offline baking requires an SDK supporting world/lightmaps-prebaked@1")
	ErrOptions = errors.New("invalid bake options")
)

// Command creates offline level bakers that do not compile game code.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "bake",
		Usage: "bake level lighting",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "level", Usage: "level directory or logical name (default: all enabled levels)"},
			&cli.IntFlag{Name: "samples", DefaultText: "level bake_samples or 16", Usage: "hemisphere samples per texel"},
			&cli.IntFlag{Name: "bounces", DefaultText: "level bake_bounces or 1", Usage: "diffuse bounce count; 0 is direct only"},
			&cli.StringFlag{
				Name:        "denoise",
				DefaultText: "level bake_denoise or medium",
				Usage:       "indirect lighting denoiser: off, low or medium",
			},
			&cli.IntFlag{Name: "workers", DefaultText: "available CPUs, up to 64", Usage: "CPU worker count (1–64)"},
			&cli.BoolFlag{Name: "incremental", Usage: "reuse completed bakes matching current sources and bake settings"},
		},
		Action: run,
	}
}

func run(ctx context.Context, command *cli.Command) error {
	config, err := project.Load(".")
	if err != nil {
		return err
	}

	manifest, err := sdk.ResolveContext(ctx, config.SDK.Version)
	if err != nil {
		return err
	}

	if err := validateBakeSDK(manifest); err != nil {
		return err
	}

	options := levelbuild.BakeOptions{
		Level: command.String("level"), Workers: command.Int("workers"), Incremental: command.Bool("incremental"),
	}
	if command.IsSet("samples") {
		value := command.Int("samples")
		options.Samples = &value
	}

	if command.IsSet("bounces") {
		value := command.Int("bounces")
		options.Bounces = &value
	}

	if command.IsSet("denoise") {
		value := command.String("denoise")
		if value == "" {
			return fmt.Errorf("%w: denoise must be off, low or medium", ErrOptions)
		}

		if _, err := worldlightmap.OfflineDenoiseProducer(value); err != nil {
			return fmt.Errorf("%w: %w", ErrOptions, err)
		}

		options.Denoise = &value
	}

	if options.Samples != nil && (*options.Samples < 1 || *options.Samples > worldlightmap.MaxOfflineSamples) ||
		options.Bounces != nil && (*options.Bounces < 0 || *options.Bounces > worldlightmap.MaxOfflineBounces) ||
		options.Workers < 0 ||
		options.Workers > 64 {
		return fmt.Errorf(
			"%w: samples must be 1–%d, bounces 0–%d and workers 0–64 (0 selects available CPUs)",
			ErrOptions,
			worldlightmap.MaxOfflineSamples,
			worldlightmap.MaxOfflineBounces,
		)
	}

	started := time.Now()

	var outputErr error

	reports, err := levelbuild.BakeAll(ctx, ".", options, func(report levelbuild.BakeReport) {
		if outputErr == nil {
			outputErr = writeReport(command.Writer, report)
		}
	})
	if err != nil {
		return err
	}

	if outputErr != nil {
		return outputErr
	}

	return writeSummary(command.Writer, reports, time.Since(started))
}

func writeReport(output io.Writer, report levelbuild.BakeReport) error {
	if report.Reused {
		_, err := fmt.Fprintf(
			output,
			"Reused %s in %s\n  Samples: %d | Bounces: %d | Denoise: %s\n  No rays traced or files written\n  %s\n  %s\n",
			report.Name,
			report.Duration.Round(time.Millisecond),
			report.Stats.Samples,
			report.Stats.Bounces,
			report.Stats.Denoise,
			report.Manifest,
			report.Image,
		)

		return err
	}

	var text strings.Builder

	fmt.Fprintf(
		&text,
		"Baked %s in %s\n  Receiver texels: %d | Rays: %d | Throughput: %.2f Mrays/sec\n  Samples: %d | Bounces: %d\n  Denoise: %s in %s\n  Files: 2, %s\n  %s\n  %s\n",
		report.Name,
		report.Duration.Round(time.Millisecond),
		report.Stats.ReceiverTexels,
		report.Stats.Rays,
		millionRaysPerSecond(report.Stats.Rays, report.Duration),
		report.Stats.Samples,
		report.Stats.Bounces,
		report.Stats.Denoise,
		report.Stats.DenoiseDuration.Round(time.Millisecond),
		ui.Size(report.OutputBytes),
		report.Manifest,
		report.Image,
	)

	if !report.Automatic {
		fmt.Fprintln(
			&text,
			"  Set [lightmap] offline = true in this level's level.toml and remove explicit prebake paths to package this bake.",
		)
	}

	_, err := io.WriteString(output, text.String())

	return err
}

func writeSummary(output io.Writer, reports []levelbuild.BakeReport, duration time.Duration) error {
	var (
		bytes int64
		rays  uint64
		files int
	)

	for _, report := range reports {
		bytes += report.OutputBytes

		rays += report.Stats.Rays
		if !report.Reused {
			files += 2
		}
	}

	_, err := fmt.Fprintf(
		output,
		"\nBake complete in %s\n  Levels: %d | Chunks: 0\n  Rays: %d | Throughput: %.2f Mrays/sec\n  Wrote %d files, %s, under %s\n  Standalone baking writes whole-level lighting. Builds bake chunk lighting.\n\nNext: karty build\n",
		duration.Round(time.Millisecond),
		len(reports),
		rays,
		millionRaysPerSecond(rays, duration),
		files,
		ui.Size(bytes),
		filepath.ToSlash(filepath.Join(".karty", "bakes")),
	)

	return err
}

func millionRaysPerSecond(rays uint64, duration time.Duration) float64 {
	const raysPerMillion = 1_000_000

	if duration <= 0 {
		return 0
	}

	return float64(rays) / duration.Seconds() / raysPerMillion
}

func validateBakeSDK(manifest sdk.Manifest) error {
	if !slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldLightmapsPrebakedV1) {
		return fmt.Errorf("%w; project selects %s", ErrSDK, manifest.Version)
	}

	return nil
}
