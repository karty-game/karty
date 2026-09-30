// Package newcommand defines the project creation command.
package newcommand

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
	"github.com/karty-game/karty/internal/ui"
	"github.com/urfave/cli/v3"
)

type staticError string

func (err staticError) Error() string {
	return string(err)
}

const errExpectedProjectName staticError = "expected a project name"

// Command creates the project scaffolding command.
func Command() *cli.Command {
	return &cli.Command{
		Name:      "new",
		Usage:     "create a game project",
		ArgsUsage: "<name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "sdk", Value: "0.0.7", Usage: "exact Karty SDK version"},
			&cli.StringFlag{Name: "template", Value: "game", Usage: "project template: game or ui"},
		},
		Action: run,
	}
}

func run(ctx context.Context, command *cli.Command) error {
	if command.NArg() != 1 {
		return errExpectedProjectName
	}

	name := command.Args().First()

	manifest, err := sdk.Resolve(command.String("sdk"))
	if err != nil {
		return err
	}

	if err := ui.RunTask(ctx, "Preparing Karty toolchain", func(taskContext context.Context) error {
		_, ensureErr := toolchain.Ensure(taskContext, manifest, toolchain.EnsureOptions{
			NeedTinyGo: true, NeedWasmTools: true, NeedAir: true,
		})

		return ensureErr
	}); err != nil {
		return fmt.Errorf("prepare SDK toolchain: %w", err)
	}

	if err := scaffold.CreateTemplate(filepath.Join(".", name), name, manifest, command.String("template")); err != nil {
		return fmt.Errorf("create project %s: %w", name, err)
	}

	return nil
}
