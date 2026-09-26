package toolchain

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type goError string

func (err goError) Error() string {
	return string(err)
}

const errGoVersionMismatch goError = "Go version does not match the SDK"

// GoOptions configures the Go compiler selection.
type GoOptions struct {
	Override string
	Version  string
	Getenv   func(string) string
}

// Go resolves the Go compiler from an explicit override, environment, or PATH.
func Go(ctx context.Context, options GoOptions) (string, error) {
	getenv := options.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}

	path := options.Override
	if path == "" {
		path = getenv("KARTY_GO")
	}

	if path == "" {
		var err error

		path, err = exec.LookPath("go")
		if err != nil {
			return "", fmt.Errorf("resolve Go compiler: %w", err)
		}
	}

	if _, err := executable("Go", path); err != nil {
		return "", err
	}

	if options.Version != "" {
		output, err := exec.CommandContext(ctx, path, "version").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("check Go version: %w\n%s", err, output)
		}

		want := "go" + options.Version
		if !strings.Contains(string(output), want) {
			return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), errGoVersionMismatch)
		}
	}

	return path, nil
}
