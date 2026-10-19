// Package commands defines the user-facing Karty CLI command tree.
package commands

import (
	"github.com/karty-game/karty/internal/commands/bake"
	"github.com/karty-game/karty/internal/commands/build"
	"github.com/karty-game/karty/internal/commands/dev"
	newcommand "github.com/karty-game/karty/internal/commands/new"
	"github.com/karty-game/karty/internal/commands/packagecmd"
	"github.com/karty-game/karty/internal/commands/quality"
	"github.com/karty-game/karty/internal/commands/schema"
	"github.com/karty-game/karty/internal/commands/screenshot"
	"github.com/karty-game/karty/internal/commands/serve"
	"github.com/karty-game/karty/internal/commands/toolchain"
	"github.com/karty-game/karty/internal/release"
	"github.com/urfave/cli/v3"
)

// New creates the root Karty command.
func New() *cli.Command {
	return &cli.Command{
		Name:    "karty",
		Usage:   "create, build and preview games",
		Version: release.Version,
		Commands: []*cli.Command{
			newcommand.Command(),
			dev.Command(),
			build.Command(),
			serve.Command(),
			quality.Command("fmt"),
			quality.Command("lint"),
			schema.Command(),
			packagecmd.Command(),
			renderCommand(),
			sdkCommand(),
			// Preserve existing scripts without crowding the root help.
			hidden(bake.Command()),
			hidden(screenshot.Command()),
			hidden(toolchain.Command()),
		},
	}
}

func renderCommand() *cli.Command {
	return &cli.Command{
		Name:  "render",
		Usage: "bake lighting and take screenshots",
		Commands: []*cli.Command{
			bake.Command(),
			screenshot.Command(),
		},
	}
}

func hidden(command *cli.Command) *cli.Command {
	command.Hidden = true

	return command
}
