package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/urfave/cli/v3"
)

func sdkCommand() *cli.Command {
	return &cli.Command{
		Name: "sdk", Usage: "install an exact SDK release",
		Commands: []*cli.Command{{
			Name: "current", Usage: "print the SDK selected by this CLI",
			Action: func(_ context.Context, command *cli.Command) error {
				_, err := fmt.Fprintln(command.Writer, release.SDKVersion())

				return err
			},
		}, {
			Name: "install", Usage: "install SDK VERSION (or a local candidate with --archive and --sha256)",
			Flags:  []cli.Flag{&cli.StringFlag{Name: "archive"}, &cli.StringFlag{Name: "sha256"}},
			Action: installSDK,
		}},
	}
}
func installSDK(ctx context.Context, command *cli.Command) error {
	if command.Args().Len() != 1 {
		return fmt.Errorf("usage: karty sdk install VERSION: %w", os.ErrInvalid)
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

		return sdk.InstallBundle(version, data, command.String("sha256"))
	}

	if command.String("sha256") != "" {
		return fmt.Errorf("--sha256 requires --archive: %w", os.ErrInvalid)
	}

	return sdk.InstallPublished(ctx, version)
}
