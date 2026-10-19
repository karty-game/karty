package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/karty-game/karty/internal/commands/toolchain"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/urfave/cli/v3"
)

func sdkCommand() *cli.Command {
	return &cli.Command{
		Name: "sdk", Usage: "manage SDK versions and build tools",
		Commands: []*cli.Command{{
			Name: "current", Usage: "show this CLI's default SDK version",
			Action: func(_ context.Context, command *cli.Command) error {
				_, err := fmt.Fprintln(command.Writer, release.SDKVersion())

				return err
			},
		}, {
			Name:      "install",
			Usage:     "install an SDK version",
			ArgsUsage: "VERSION",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "archive", Usage: "install from a local SDK archive (requires --sha256)"},
				&cli.StringFlag{Name: "sha256", Usage: "expected SHA-256 checksum for --archive"},
			},
			Action: installSDK,
		}, toolchain.Command()},
	}
}
func installSDK(ctx context.Context, command *cli.Command) error {
	if command.Args().Len() != 1 {
		return fmt.Errorf("usage: %s VERSION: %w", command.FullName(), os.ErrInvalid)
	}

	version := command.Args().First()
	if archive := command.String("archive"); archive != "" {
		file, err := os.Open(archive)
		if err != nil {
			return err
		}
		defer file.Close()

		const maxArchiveBytes = 32 << 20

		data, err := io.ReadAll(io.LimitReader(file, maxArchiveBytes+1))
		if err != nil {
			return err
		}

		return sdk.InstallBundleContext(ctx, version, data, command.String("sha256"))
	}

	if command.String("sha256") != "" {
		return fmt.Errorf("--sha256 requires --archive: %w", os.ErrInvalid)
	}

	return sdk.InstallPublished(ctx, version)
}
