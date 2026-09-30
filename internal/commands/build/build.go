// Package build defines the client build command.
package build

import (
	"context"
	"os"

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
			&cli.StringFlag{Name: "go", Usage: "path to a Go executable"},
			&cli.StringFlag{Name: "tinygo", Usage: "path to a TinyGo executable"},
			&cli.StringFlag{Name: "wasm-tools", Usage: "path to a wasm-tools executable"},
			&cli.StringFlag{Name: "host", Usage: "path to a pre-built Karty host artifact"},
			&cli.StringFlag{Name: "platform", Usage: "native distribution platform, e.g. windows-arm64 (default: current machine)"},
			&cli.StringFlag{Name: "target", Value: "native", Usage: "target to stage: native or web"},
		},
		Action: run,
	}
}

func run(ctx context.Context, command *cli.Command) error {
	return ui.RunTask(ctx, "Building client", func(taskContext context.Context) error {
		return buildservice.RunWithOptions(taskContext, ".", buildservice.Options{
			Go:        command.String("go"),
			TinyGo:    command.String("tinygo"),
			WasmTools: command.String("wasm-tools"),
			Host:      command.String("host"),
			Target:    command.String("target"),
			Platform:  command.String("platform"),
			AirProxy:  os.Getenv("KARTY_AIR_PROXY") == "1",
		})
	})
}
