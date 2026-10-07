// karty-test-sdk prepares the current SDK used by all contributor tests.
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
