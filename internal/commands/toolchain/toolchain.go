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
		Usage: "manage build tools for this project",
		Commands: []*cli.Command{
			{
				Name:   "install",
				Usage:  "install this project's build tools",
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

	manifest, err := sdk.ResolveContext(ctx, config.SDK.Version)
	if err != nil {
		return err
	}

	paths, err := toolchainservice.Ensure(ctx, manifest, installOptions(manifest))
	if err != nil {
		return err
	}

	slog.Info("Karty toolchain installed", "go", paths.Go, "tinygo", paths.TinyGo, "wasm-tools", paths.WasmTools)

	if paths.Materialize != "" {
		slog.Info("Materialize installed", "path", paths.Materialize)
	}

	return nil
}

func installOptions(manifest sdk.Manifest) toolchainservice.EnsureOptions {
	return toolchainservice.EnsureOptions{
		NeedGo: true, NeedTinyGo: true, NeedWasmTools: true,
		NeedMaterialize: manifest.Tools.Materialize != "",
	}
}
