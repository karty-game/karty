// Package newcommand defines the project creation command.
package newcommand

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/ui"
	"github.com/urfave/cli/v3"
)

// Command creates a guided project setup command with noninteractive flags.
func Command() *cli.Command {
	return &cli.Command{
		Name:        "new",
		Usage:       "create a game project",
		ArgsUsage:   "[name]",
		Description: "Choose a starter and optional Git setup. Use --yes to skip questions.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "sdk", Value: release.SDKVersion(), Usage: "exact Karty SDK version"},
			&cli.StringFlag{
				Name:    "template",
				Aliases: []string{"t"},
				Value:   "2d",
				Usage:   "starter: blank, 2d, fps, isometric or ui (none aliases blank; game aliases 2d)",
			},
			&cli.BoolFlag{Name: "no-template", Usage: "create only the base folders and minimal entry point (same as --template blank)"},
			&cli.BoolFlag{Name: "git", Usage: "initialize a local Git repository without creating a commit"},
			&cli.BoolFlag{Name: "gitignore", Value: true, Usage: "include a .gitignore for generated Karty output"},
			&cli.BoolFlag{Name: "github-actions", Usage: "include a GitHub Actions web build and artifact workflow"},
			&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "skip interactive questions and use flags/defaults"},
		},
		Action: run,
	}
}

func run(ctx context.Context, command *cli.Command) error {
	options, err := setup(ctx, command)
	if err != nil {
		return err
	}

	selection, err := scaffold.NormalizeTemplate(options.template)
	if err != nil {
		return err
	}

	destination := filepath.Join(".", options.name)
	if err := scaffold.ValidateDestination(destination, options.name); err != nil {
		return err
	}

	if options.git {
		if _, err := exec.LookPath("git"); err != nil {
			return fmt.Errorf("git initialization requested but git is unavailable; install Git or use --git=false: %w", err)
		}
	}

	manifest, err := sdk.ResolveContext(ctx, command.String("sdk"))
	if errors.Is(err, sdk.ErrNotInstalled) {
		if err = ui.RunTask(ctx, "Installing Karty SDK "+command.String("sdk"), func(ctx context.Context) error {
			return sdk.InstallPublished(ctx, command.String("sdk"))
		}); err != nil {
			return err
		}

		manifest, err = sdk.ResolveContext(ctx, command.String("sdk"))
	}

	if err != nil {
		return err
	}

	if err := scaffold.ValidateTemplate(ctx, manifest, selection); err != nil {
		return err
	}

	if err := scaffold.CreateTemplateContext(ctx, destination, options.name, manifest, selection); err != nil {
		return fmt.Errorf("create project %s: %w", options.name, err)
	}

	if err := scaffold.AddExtras(ctx, destination, manifest, scaffold.Extras{
		Git: options.git, Gitignore: options.gitignore, GitHubActions: options.githubActions,
	}); err != nil {
		return fmt.Errorf("project created at %s but optional setup failed: %w", destination, err)
	}

	_, err = fmt.Fprintf(
		command.Writer,
		"Created %s (%s) using SDK %s.\n\nNext:\n  cd %s\n  karty dev\n\nEdit src/main.go and save to rebuild. See README.md and .karty/docs/.\n",
		options.name,
		options.template,
		manifest.Version,
		options.name,
	)

	return err
}
