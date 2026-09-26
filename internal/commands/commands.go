// Package commands defines the user-facing Karty CLI command tree.
package commands

import (
	"github.com/karty-game/karty/internal/commands/build"
	"github.com/karty-game/karty/internal/commands/dev"
	newcommand "github.com/karty-game/karty/internal/commands/new"
	"github.com/karty-game/karty/internal/commands/serve"
	"github.com/karty-game/karty/internal/commands/toolchain"
	"github.com/urfave/cli/v3"
)

// New creates the root Karty command.
func New() *cli.Command {
	return &cli.Command{
		Name:  "karty",
		Usage: "build and run Karty projects",
		Commands: []*cli.Command{
			newcommand.Command(),
			sdkCommand(),
			build.Command(),
			dev.Command(),
			serve.Command(),
			toolchain.Command(),
		},
	}
}
