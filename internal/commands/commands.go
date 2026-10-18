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
		Usage:   "build and run Karty projects",
		Version: release.Version,
		Commands: []*cli.Command{
			newcommand.Command(),
			packagecmd.Command(),
			quality.Command("fmt"),
			quality.Command("lint"),
			sdkCommand(),
			build.Command(),
			bake.Command(),
			screenshot.Command(),
			schema.Command(),
			dev.Command(),
			serve.Command(),
			toolchain.Command(),
		},
	}
}
