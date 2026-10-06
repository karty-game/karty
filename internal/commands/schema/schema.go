// Package schema defines editor schema generation without running a build.
package schema

import (
	"context"
	"fmt"

	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/urfave/cli/v3"
)

// Command creates the YAML schema refresh command.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "schema",
		Usage: "refresh world YAML editor schemas with level materials, textures and prefabs",
		Flags: []cli.Flag{&cli.BoolFlag{Name: "check", Usage: "also validate world YAML without processing assets or compiling code"}},
		Action: func(_ context.Context, command *cli.Command) error {
			if command.NArg() != 0 {
				return fmt.Errorf("schema takes no arguments: %w", cli.Exit("run karty schema from the project root", 1))
			}

			count, err := levelbuild.GenerateSchemas(".", command.Bool("check"))
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(command.Root().Writer, "Updated world YAML schemas in .karty/schemas (%d levels).\n", count)

			return err
		},
	}
}
