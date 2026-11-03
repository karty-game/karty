package toolchain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/karty-game/karty/internal/sdk"
)

const errHostMetadata staticError = "host artifact metadata is incomplete"
const errHostChecksum staticError = "host artifact checksum mismatch"
const errHostNotFile staticError = "managed Karty host is not a regular file"

// EnsureHost resolves only the current complete, verified WebGPU bundle.
func EnsureHost(ctx context.Context, manifest sdk.Manifest, target string) (string, error) {
	bundle, err := EnsureManagedHostBundle(ctx, manifest, target)
	if err != nil {
		return "", err
	}

	return filepath.Join(bundle.Directory, bundle.Entrypoint), nil
}

// NativePlatform validates a game distribution target independently of installed tools.
func NativePlatform(platform string) (string, error) {
	if platform == "" {
		platform = runtime.GOOS + "-" + runtime.GOARCH
	}

	switch platform {
	case "linux-amd64", "linux-arm64", "darwin-arm64", "windows-amd64", "windows-arm64":
		return platform, nil
	default:
		return "", fmt.Errorf("unsupported game platform %q: %w", platform, os.ErrInvalid)
	}
}

// NativeHostName uses the destination OS, not the build machine's OS.
func NativeHostName(platform string) string {
	if platform == "" {
		platform = runtime.GOOS + "-" + runtime.GOARCH
	}

	if strings.HasPrefix(platform, "windows-") {
		return "karty-host.exe"
	}

	return "karty-host"
}

func hostPlatform(target string) (string, error) {
	if target == "web" {
		return target, nil
	}

	if target == "native" {
		target = ""
	}

	return NativePlatform(target)
}
