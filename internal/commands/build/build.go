// Package build defines the client build command.
package build

import (
	"context"

	buildservice "github.com/karty-game/karty/internal/build"
	"github.com/karty-game/karty/internal/ui"
	"github.com/urfave/cli/v3"
)

// Command creates the build command.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "build",
		Usage: "build the client cartridge in the current project",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "compiler", Usage: "cartridge compiler: go or tinygo (development defaults to go)"},
			&cli.StringFlag{Name: "go", Usage: "path to a Go executable"},
			&cli.StringFlag{Name: "tinygo", Usage: "path to a TinyGo executable"},
			&cli.StringFlag{Name: "wasm-tools", Usage: "path to a wasm-tools executable"},
			&cli.StringFlag{Name: "host", Usage: "path to a pre-built Karty host artifact"},
			&cli.StringFlag{Name: "platform", Usage: "native distribution platform, e.g. windows-arm64 (default: current machine)"},
			&cli.StringFlag{Name: "target", Value: "native", Usage: "target to stage: native or web"},
			&cli.StringFlag{Name: "cdn", Usage: "web host URL to load at runtime, verified against the pinned SDK release"},
			&cli.BoolFlag{Name: "bundled-runtime", Usage: "include the web runtime locally instead of using project.web.runtime_url"},
			&cli.BoolFlag{Name: "development", Usage: "reuse matching prebuilt lighting; otherwise build mesh-only preview chunks"},
		},
		Action: run,
	}
}

func run(ctx context.Context, command *cli.Command) error {
	var summary buildservice.Summary

	err := ui.RunTaskWithProgress(ctx, "Building game", func(taskContext context.Context, progress func(string)) error {
		return buildservice.RunWithOptions(taskContext, ".", buildservice.Options{
			Compiler:       command.String("compiler"),
			Go:             command.String("go"),
			TinyGo:         command.String("tinygo"),
			WasmTools:      command.String("wasm-tools"),
			Host:           command.String("host"),
			Target:         command.String("target"),
			CDN:            command.String("cdn"),
			BundledRuntime: command.Bool("bundled-runtime"),
			Platform:       command.String("platform"),
			Development:    command.Bool("development"),
			Progress:       progress,
			Summary:        func(report buildservice.Summary) { summary = report },
		})
	})
	if err != nil {
		return err
	}

	return writeSummary(command.Writer, summary)
}
