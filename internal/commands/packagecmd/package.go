// Package packagecmd implements local package export, import and restoration.
package packagecmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/internal/packages"
	"github.com/karty-game/karty/internal/project"
	"github.com/urfave/cli/v3"
)

func Command() *cli.Command {
	return &cli.Command{Name: "package", Usage: "share local Go, prefab and texture packages", Commands: []*cli.Command{
		{
			Name:      "export",
			Usage:     "export a package folder to a deterministic .kpkg archive",
			ArgsUsage: "<folder>",
			Flags:     []cli.Flag{&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Required: true, Usage: "new .kpkg output file"}},
			Action:    exportPackage,
		},
		{
			Name:      "import",
			Usage:     "add a local archive or linked development folder to this project",
			ArgsUsage: "<archive.kpkg|folder>",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "replace",
					Usage: "explicitly update the source and Go module mapping for this package's manifest name",
				},
			},
			Action: importPackage,
		},
		{Name: "restore", Usage: "recreate package caches and development links from karty.toml", Action: restorePackages},
	}}
}

func exportPackage(ctx context.Context, command *cli.Command) error {
	if command.NArg() != 1 {
		return fmt.Errorf("%w: expected one package folder", packages.ErrPackage)
	}

	definition, digest, err := packages.Export(ctx, command.Args().First(), command.String("output"))
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		command.Writer,
		"Exported %s %s to %s\nSHA-256: %s\n",
		definition.Name,
		definition.Version,
		command.String("output"),
		digest,
	)

	return err
}

func importPackage(ctx context.Context, command *cli.Command) error {
	if command.NArg() != 1 {
		return fmt.Errorf("%w: expected one local archive or development folder", packages.ErrPackage)
	}

	if _, err := project.Load("."); err != nil {
		return err
	}

	definition, err := packages.Import(ctx, ".", command.Args().First(), command.Bool("replace"))
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		command.Writer,
		"Imported %s %s\nRecorded in karty.toml; resolved at %s\n",
		definition.Name,
		definition.Version,
		filepath.Join(".karty/packages", definition.Name),
	)

	return err
}

func restorePackages(ctx context.Context, command *cli.Command) error {
	if command.NArg() != 0 {
		return fmt.Errorf("%w: restore takes no arguments", packages.ErrPackage)
	}

	if _, err := project.Load("."); err != nil {
		return err
	}

	directory, err := os.Getwd()
	if err != nil {
		return err
	}

	set, err := packages.Restore(ctx, directory)
	if err != nil {
		return err
	}
	defer set.Close()

	if err := packages.CheckModules(ctx, directory, set); err != nil {
		return err
	}

	_, err = fmt.Fprintf(command.Writer, "Restored %d packages in .karty/packages\n", len(set.Items))

	return err
}
