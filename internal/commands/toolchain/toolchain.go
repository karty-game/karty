// Package toolchain defines commands for managing the SDK toolchain.
package toolchain

import (
	"context"
	"log/slog"

	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
	toolchainservice "github.com/karty-game/karty/internal/toolchain"
	"github.com/urfave/cli/v3"
)

// Command creates the toolchain command group.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "toolchain",
		Usage: "manage tools pinned by the current project SDK",
		Commands: []*cli.Command{
			{
				Name:   "install",
				Usage:  "install tools for the current project SDK",
				Action: install,
			},
		},
	}
}

func install(ctx context.Context, _ *cli.Command) error {
	config, err := project.Load(".")
	if err != nil {
		return err
	}

	manifest, err := sdk.Resolve(config.SDK.Version)
	if err != nil {
		return err
	}

	paths, err := toolchainservice.Ensure(ctx, manifest, toolchainservice.EnsureOptions{
		NeedGo: true, NeedTinyGo: true, NeedWasmTools: true, NeedAir: true,
	})
	if err != nil {
		return err
	}

	slog.Info("Karty toolchain installed", "go", paths.Go, "air", paths.Air, "tinygo", paths.TinyGo, "wasm-tools", paths.WasmTools)

	return nil
}
