// karty-test-sdk prepares the current SDK used by all contributor tests.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/testtoolchain"
	"github.com/karty-game/karty/internal/toolchain"
)

func main() {
	goPath := flag.Bool("go-path", false, "prepare and print the selected SDK's exact Go compiler path")
	ktxPath := flag.Bool("ktx-path", false, "prepare and print the selected SDK's CPU texture encoder path")

	flag.Parse()

	if err := prepare(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *goPath {
		path, err := prepareGo()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		fmt.Fprintln(os.Stdout, path)
	}

	if *ktxPath {
		path, err := prepareKTX()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		fmt.Fprintln(os.Stdout, path)
	}
}

func prepareGo() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	manifest, err := sdk.ResolveContext(ctx, release.SDKVersion())
	if err != nil {
		return "", err
	}

	return testtoolchain.Go(ctx, manifest)
}

func prepareKTX() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	manifest, err := sdk.ResolveContext(ctx, release.SDKVersion())
	if err != nil {
		return "", err
	}

	return toolchain.EnsureKTX(ctx, manifest, os.Getenv("KARTY_KTX"))
}

func prepare() error {
	return prepareVersion(release.SDKVersion())
}

func prepareVersion(version string) error {
	if _, err := sdk.Resolve(version); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	return sdk.InstallPublished(ctx, version)
}
