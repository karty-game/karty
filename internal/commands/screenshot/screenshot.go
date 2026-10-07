// Package screenshot captures an authored world with a local native host.
package screenshot

import (
	"context"
	"fmt"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ui "github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
	"github.com/urfave/cli/v3"
)

type camera struct {
	position                   [3]float64
	yaw, pitch, fov, near, far float64
	width, height              int
}

// Command creates a native, one-shot world screenshot command.
func Command() *cli.Command {
	return &cli.Command{
		Name: "screenshot", Usage: "render a level with the local native host, save a PNG and exit", ArgsUsage: "<level>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "position", Value: "0,0,1.7", Usage: "camera eye position as X,Y,Z in world units"},
			&cli.Float64Flag{Name: "yaw", Usage: "yaw in degrees; zero looks along +Y"},
			&cli.Float64Flag{Name: "pitch", Usage: "pitch in degrees; positive looks up"},
			&cli.Float64Flag{Name: "fov", Usage: "vertical FOV in degrees (default: project camera)"},
			&cli.IntFlag{Name: "width", Usage: "PNG width, at most 1280 (default: project resolution)"},
			&cli.IntFlag{Name: "height", Usage: "PNG height, at most 720 (default: project resolution)"},
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "PNG path (default: dist/screenshots/<level>-<timestamp>.png)"},
			&cli.StringFlag{Name: "batch", Usage: "JSON array of camera views; capture all in one native session"},
			&cli.StringFlag{Name: "host", Usage: "local native karty-host executable (or KARTY_HOST_NATIVE)"},
			&cli.DurationFlag{Name: "timeout", Value: time.Minute, Usage: "maximum native capture duration"},
		}, Action: run,
	}
}

func run(ctx context.Context, command *cli.Command) error {
	if command.NArg() != 1 {
		return fmt.Errorf("usage: karty screenshot [options] <level>: %w", os.ErrInvalid)
	}

	config, err := project.Load(".")
	if err != nil {
		return err
	}

	pose, err := cameraOptions(command, config)
	if err != nil {
		return err
	}

	if command.Duration("timeout") <= 0 {
		return fmt.Errorf("timeout must be positive: %w", os.ErrInvalid)
	}

	requests, err := viewOptions(command, pose)
	if err != nil {
		return err
	}

	manifest, err := sdk.Resolve(config.SDK.Version)
	if err != nil {
		return err
	}

	host, err := nativeHost(ctx, command.String("host"), manifest)
	if err != nil {
		return err
	}

	version := ui.SchemaInteractionPolish
	if manifest.API.Version == "0.0.7" {
		version = 11
	}

	fmt.Fprintln(command.Root().ErrWriter, "Building level", command.Args().First())

	artifact, err := levelbuild.BuildNamedForScreenshot(ctx, ".", command.Args().First(), version, config.Assets.Theme.Source, manifest)
	if err != nil {
		return err
	}

	cache, err := os.MkdirTemp("", "karty-screenshot-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cache)

	levelPath, err := filepath.Abs(filepath.Join(cache, artifact.ContentSHA256+".kld"))
	if err != nil {
		return err
	}

	if err := os.WriteFile(levelPath, artifact.Bytes, 0600); err != nil {
		return err
	}

	captureCtx, cancel := context.WithTimeout(ctx, command.Duration("timeout"))
	defer cancel()

	if command.String("batch") == "" {
		err = capture(captureCtx, host, levelPath, requests[0].output, artifact.Features, requests[0].pose, command.Root().ErrWriter)
	} else {
		err = captureBatch(captureCtx, host, levelPath, artifact.Features, requests, command.Root().ErrWriter)
	}

	if err != nil {
		return err
	}

	for _, request := range requests {
		if _, err := fmt.Fprintln(command.Root().Writer, request.output); err != nil {
			return err
		}
	}

	return nil
}

func cameraOptions(command *cli.Command, config project.Config) (camera, error) {
	pose := camera{
		yaw: command.Float64("yaw"), pitch: command.Float64("pitch"),
		fov: config.Project.Camera.FOVYDegrees, near: config.Project.Camera.Near, far: config.Project.Camera.Far,
		width: config.Project.Resolution.Width, height: config.Project.Resolution.Height,
	}

	parts := strings.Split(command.String("position"), ",")
	if len(parts) != len(pose.position) {
		return pose, fmt.Errorf("position must be X,Y,Z: %w", os.ErrInvalid)
	}

	for index, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return pose, fmt.Errorf("position must be X,Y,Z: %w", err)
		}

		pose.position[index] = value
	}

	if command.IsSet("fov") {
		pose.fov = command.Float64("fov")
	}

	if command.IsSet("width") {
		pose.width = command.Int("width")
	}

	if command.IsSet("height") {
		pose.height = command.Int("height")
	}

	return pose, pose.validate()
}

func (pose camera) validate() error {
	for _, value := range append(pose.position[:], pose.yaw, pose.pitch, pose.fov) {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e6 {
			return fmt.Errorf("camera values must be finite and bounded: %w", os.ErrInvalid)
		}
	}

	if pose.width < 1 || pose.width > 1280 || pose.height < 1 || pose.height > 720 || pose.fov < 5 || pose.fov > 150 || pose.pitch <= -90 ||
		pose.pitch >= 90 {
		return fmt.Errorf("invalid screenshot size or FOV: %w", os.ErrInvalid)
	}

	return nil
}

func nativeHost(ctx context.Context, override string, manifest sdk.Manifest) (string, error) {
	if override == "" {
		override = os.Getenv("KARTY_HOST_NATIVE")
	}

	if override == "" {
		platform, err := toolchain.NativePlatform("")
		if err != nil {
			return "", err
		}

		for _, directory := range []string{filepath.Join("dist", "native", platform), filepath.Join("dist", "native")} {
			candidate := filepath.Join(directory, toolchain.NativeHostName(platform))
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
				override = candidate

				break
			}
		}
	}

	if override != "" {
		return filepath.Abs(override)
	}

	return toolchain.EnsureHost(ctx, manifest, "native")
}

func capture(ctx context.Context, host, levelPath, output string, features []string, pose camera, diagnostics io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(output), 0750); err != nil {
		return err
	}

	staging, err := os.CreateTemp(filepath.Dir(output), ".capture-*.png")
	if err != nil {
		return err
	}

	path := staging.Name()
	defer os.Remove(path)

	if err := staging.Close(); err != nil {
		return err
	}
	// An empty staging file cannot be mistaken for an older successful image.
	arguments := make([]string, 0, 28)

	arguments = append(arguments,
		"--screenshot-protocol",
		"1",
		"--screenshot-level",
		levelPath,
		"--screenshot-output",
		path,
		"--screenshot-features",
		strings.Join(features, ","),
		"--screenshot-width",
		strconv.Itoa(pose.width),
		"--screenshot-height",
		strconv.Itoa(pose.height),
	)
	for _, field := range []struct {
		name  string
		value float64
	}{
		{"x", pose.position[0]}, {"y", pose.position[1]}, {"z", pose.position[2]}, {"yaw", pose.yaw}, {"pitch", pose.pitch},
		{"fov", pose.fov}, {"near", pose.near}, {"far", pose.far},
	} {
		arguments = append(arguments, "--camera-"+field.name, strconv.FormatFloat(field.value, 'g', -1, 64))
	}

	command := exec.CommandContext(ctx, host, arguments...)

	command.Stdout, command.Stderr = diagnostics, diagnostics
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("native screenshot: %w", ctx.Err())
		}

		return fmt.Errorf("native screenshot failed; use a host supporting screenshot-v1 (--host or KARTY_HOST_NATIVE): %w", err)
	}

	if err := validatePNG(path, pose); err != nil {
		return err
	}

	return os.Rename(path, output)
}

func validatePNG(path string, pose camera) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("host did not save screenshot: %w", err)
	}
	defer file.Close()

	image, err := png.DecodeConfig(file)
	if err != nil {
		return fmt.Errorf("host output is not a PNG: %w", err)
	}

	if image.Width != pose.width || image.Height != pose.height {
		return fmt.Errorf("host returned incorrect screenshot dimensions: %w", os.ErrInvalid)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	const maxPNGSize = 16 * 1024 * 1024
	if _, err := png.Decode(io.LimitReader(file, maxPNGSize)); err != nil {
		return fmt.Errorf("incomplete screenshot PNG: %w", err)
	}

	if err := file.Close(); err != nil {
		return err
	}

	return nil
}
