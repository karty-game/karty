package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

// Resolve host delivery before compiling levels. A signed bundle declaration
// requires the current WebGPU bundle profile and a verified full inventory.
func resolveManagedHost(ctx context.Context, manifest sdk.Manifest, options *Options) error {
	if manifest.Host.Format != sdk.HostBundleFormat || manifest.Host.Renderer != sdk.HostBundleRenderer {
		return fmt.Errorf(
			"SDK %s is unsupported: current builds require %s/%s: %w",
			manifest.Version,
			sdk.HostBundleFormat,
			sdk.HostBundleRenderer,
			os.ErrInvalid,
		)
	}

	if options.Host != "" && options.hostBundle == nil {
		return fmt.Errorf(
			"SDK %s requires a verified %s host; select the bundle directory or its declared entrypoint: %w",
			manifest.Version,
			sdk.HostBundleFormat,
			os.ErrInvalid,
		)
	}

	if options.hostBundle != nil || options.Host != "" {
		if options.Target == "web" {
			return toolchain.ValidateWebHostSDK(manifest, options.hostBundle)
		}

		return nil
	}

	if selected := os.Getenv("KARTY_HOST_WEB"); options.Target == "web" && selected != "" {
		bundle, err := readHostBundle(ctx, selected, "web", "")
		if err != nil {
			return err
		}

		if err := toolchain.ValidateWebHostSDK(manifest, bundle); err != nil {
			return err
		}

		options.Host, options.hostBundle = selected, bundle

		return nil
	}

	target := options.Target
	if options.Platform != "" {
		target = options.Platform
	}

	if target == "" {
		target = "native"
	}

	bundle, err := toolchain.EnsureManagedHostBundle(ctx, manifest, target)
	if err != nil {
		return err
	}

	options.hostBundle = bundle
	// An explicitly raw-only build still compiles the selected renderer profile,
	// but does not gain a native distribution as a side effect of verification.
	if options.Target != "" {
		options.Host = filepath.Join(bundle.Directory, bundle.Entrypoint)
	}

	return nil
}
