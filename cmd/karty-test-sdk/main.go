// karty-test-sdk prepares released SDKs used by default and compatibility tests.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func main() {
	if err := prepare(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func prepare() error {
	for _, version := range []string{release.SDKVersion(), "0.0.7", "0.0.5"} {
		if err := prepareVersion(version); err != nil {
			return err
		}
	}

	return nil
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
