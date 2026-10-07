// Package bake defines the offline directional lightmap command.
package bake

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/urfave/cli/v3"
)

var (
	ErrSDK     = errors.New("offline baking requires an SDK supporting world/lightmaps-prebaked@1")
	ErrOptions = errors.New("invalid bake options")
)

// Command creates a CPU-only baker that does not compile game code.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "bake",
		Usage: "bake static directional lighting and diffuse radiosity in the current project",
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
		},
		Action: run,
	}
}

func run(ctx context.Context, command *cli.Command) error {
	config, err := project.Load(".")
	if err != nil {
		return err
	}

	manifest, err := sdk.Resolve(config.SDK.Version)
	if err != nil {
		return err
	}

	if err := validateBakeSDK(manifest); err != nil {
		return err
	}

	options := levelbuild.BakeOptions{Level: command.String("level"), Workers: command.Int("workers")}
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

	_, err = levelbuild.BakeAll(ctx, ".", options, func(report levelbuild.BakeReport) {
		fmt.Fprintf(
			os.Stdout,
			"Baked %s in %s: %d receiver texels, %d rays, %d samples, %d bounces\n  Denoise: %s (%s)\n  %s\n",
			report.Name,
			report.Duration.Round(time.Millisecond),
			report.Stats.ReceiverTexels,
			report.Stats.Rays,
			report.Stats.Samples,
			report.Stats.Bounces,
			report.Stats.Denoise,
			report.Stats.DenoiseDuration.Round(time.Millisecond),
			report.Manifest,
		)

		if !report.Automatic {
			fmt.Fprintln(
				os.Stdout,
				"  Set [lightmap] offline = true in this level's level.toml and remove explicit prebake paths to package this generated bake.",
			)
		}
	})

	return err
}

func validateBakeSDK(manifest sdk.Manifest) error {
	if !slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldLightmapsPrebakedV1) {
		return fmt.Errorf("%w; project selects %s", ErrSDK, manifest.Version)
	}

	return nil
}
