// Package testtoolchain selects SDK tools for contributor checks. Production
// commands resolve user-installed tools through package toolchain instead.
package testtoolchain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/karty-game/karty/internal/process"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

// Go resolves the SDK's exact guest compiler independently of the compiler used
// to build repository tools. An explicit KARTY_GO is checked without installing
// or substituting another compiler. Otherwise mise supplies the SDK manifest pin.
func Go(ctx context.Context, manifest sdk.Manifest) (string, error) {
	path := os.Getenv("KARTY_GO")
	if path == "" {
		var err error

		path, err = goFromMise(ctx, manifest.Tools.Go, func(ctx context.Context, args ...string) ([]byte, error) {
			command := process.CommandContext(ctx, "mise", args...)
			command.Stderr = os.Stderr

			return command.Output()
		})
		if err != nil {
			return "", fmt.Errorf("prepare SDK %s Go: %w", manifest.Version, err)
		}
	}

	return toolchain.Go(ctx, toolchain.GoOptions{Override: path, Version: manifest.Tools.Go})
}

func goFromMise(ctx context.Context, version string, run func(context.Context, ...string) ([]byte, error)) (string, error) {
	if version == "" {
		return "", fmt.Errorf("SDK does not specify Go: %w", os.ErrInvalid)
	}

	output, err := run(ctx, "exec", "go@"+version, "--", "go", "env", "GOROOT")
	if err != nil {
		return "", fmt.Errorf("mise Go %s: %w\n%s", version, err, output)
	}

	root := strings.TrimSpace(string(output))
	if !filepath.IsAbs(root) || strings.ContainsAny(root, "\r\n") {
		return "", fmt.Errorf("mise Go %s returned invalid GOROOT %q: %w", version, root, os.ErrInvalid)
	}

	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	return filepath.Join(root, "bin", name), nil
}
