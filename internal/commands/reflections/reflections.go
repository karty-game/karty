// Package reflections builds saved static reflection environments.
package reflections

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldreflection"
	buildservice "github.com/karty-game/karty/internal/build"
	"github.com/karty-game/karty/internal/commands/screenshot"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/urfave/cli/v3"
)

const captureTimeout = 5 * time.Minute

// Command captures all automatic indoor environments outside gameplay.
func Command() *cli.Command {
	return &cli.Command{Name: "reflections", Usage: "bake static parallax-corrected cubemaps", ArgsUsage: "<level>", Flags: []cli.Flag{
		&cli.StringFlag{Name: "host", Usage: "verified native host bundle or declared entrypoint (or KARTY_HOST_NATIVE)"},
		screenshot.BackendFlag(),
		&cli.DurationFlag{Name: "timeout", Value: captureTimeout, Usage: "maximum native capture duration"},
	}, Action: run}
}

func run(ctx context.Context, command *cli.Command) error {
	if command.NArg() != 1 || command.Duration("timeout") <= 0 {
		return fmt.Errorf("usage: %s [options] <level>: %w", command.FullName(), os.ErrInvalid)
	}

	config, err := project.Load(".")
	if err != nil {
		return err
	}

	manifest, err := sdk.ResolveContext(ctx, config.SDK.Version)
	if err != nil {
		return err
	}

	if !slices.Contains(manifest.Assets.Capabilities.Runtime, asset.Capability(worldreflection.Feature)) {
		return fmt.Errorf("selected runtime does not support saved reflection cubemaps: %w", os.ErrInvalid)
	}

	host, err := screenshot.ResolveNativeHost(ctx, command.String("host"), manifest)
	if err != nil {
		return err
	}

	webgpu, err := buildservice.ValidateLocalHost(ctx, host, "native", "")
	if err != nil {
		return err
	}

	options, err := screenshot.CaptureBackend(command.String("backend"), webgpu)
	if err != nil {
		return err
	}

	artifact, err := levelbuild.BuildNamedForReflections(
		ctx,
		".",
		command.Args().First(),
		manifest.Compatibility.UISchema,
		config.Assets.Theme.Source,
		manifest,
	)
	if err != nil {
		return err
	}

	cache, err := os.MkdirTemp("", "karty-reflections-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cache)

	levelPath, err := stageCaptureInputs(cache, artifact)
	if err != nil {
		return err
	}

	captureCtx, cancel := context.WithTimeout(ctx, command.Duration("timeout"))
	defer cancel()

	output := filepath.Join(cache, "capture.krc")
	if err := captureWithOptions(captureCtx, host, levelPath, output, artifact.Features, command.Root().ErrWriter, options); err != nil {
		return err
	}

	file, err := os.Open(output)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, worldreflection.MaxEncodedSize+1))
	if err != nil {
		return err
	}

	path, err := levelbuild.SaveReflectionCapture(".", artifact, data)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(command.Root().Writer, path)

	return err
}

func capture(ctx context.Context, host, levelPath, output string, features []string, diagnostics io.Writer) error {
	return captureWithOptions(ctx, host, levelPath, output, features, diagnostics, screenshot.CaptureOptions{})
}

func captureWithOptions(
	ctx context.Context,
	host, levelPath, output string,
	features []string,
	diagnostics io.Writer,
	options screenshot.CaptureOptions,
) error {
	arguments := append(options.Arguments(),
		"--reflection-protocol",
		"1",
		"--reflection-level",
		levelPath,
		"--reflection-output",
		output,
		"--reflection-features",
		strings.Join(features, ","),
	)
	// #nosec G204 -- The executable is an explicitly selected or verified SDK native host; arguments are passed without a shell.
	command := exec.CommandContext(ctx, host, arguments...)

	command.Stdout, command.Stderr = diagnostics, diagnostics
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		return fmt.Errorf("native reflection capture failed; use a host supporting reflection-protocol 1: %w", err)
	}

	return nil
}

func stageCaptureInputs(directory string, artifact levelbuild.Artifact) (string, error) {
	return screenshot.StageCaptureInputs(directory, artifact)
}
