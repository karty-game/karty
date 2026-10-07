package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/internal/build"
	"github.com/karty-game/karty/internal/release"
)

// This fixture deliberately has no dependency on samples, offline baking,
// Materialize, UI compilation or startup GPU baking.
func runClientHooksCheck(ctx context.Context, root string, nativeOnly bool) error {
	host, checker := os.Getenv("KARTY_HOST_NATIVE"), os.Getenv("KARTY_HOST_CHECK")
	if host == "" {
		return errNativeHostMissing
	}

	if checker == "" {
		return errHostCheckMissing
	}

	webHost := os.Getenv("KARTY_HOST_WEB")
	if !nativeOnly && webHost == "" {
		return errWebHostMissing
	}

	temporary, err := os.MkdirTemp("", "karty-client-hooks-check-")
	if err != nil {
		return err
	}

	if os.Getenv("KARTY_CHECK_KEEP_PROJECT") == "1" {
		fmt.Fprintln(os.Stdout, "Retained check project:", temporary)
	} else {
		defer os.RemoveAll(temporary)
	}

	if err := prepareHooksFixture(root, temporary); err != nil {
		return err
	}

	options := build.Options{
		Go:        os.Getenv("KARTY_GO"),
		TinyGo:    os.Getenv("KARTY_TINYGO"),
		WasmTools: os.Getenv("KARTY_WASM_TOOLS"),
		Host:      host,
		Target:    "native",
	}

	for _, target := range []string{"native", "web"} {
		if target == "web" {
			if nativeOnly {
				break
			}

			options.Target, options.Host = target, webHost
		}

		distribution := filepath.Join(temporary, "dist", target)
		if err := build.RunWithOptions(ctx, temporary, options); err != nil {
			return err
		}

		first, err := distributionDigest(distribution)
		if err != nil {
			return err
		}

		if err := build.RunWithOptions(ctx, temporary, options); err != nil {
			return err
		}

		second, err := distributionDigest(distribution)
		if err != nil {
			return err
		}

		if first != second {
			return fmt.Errorf("%s: %w", target, errBuildChanged)
		}

		if target == "native" {
			if err := command(ctx, distribution, nil, checker, "--cartridge", "game.kart", "--client-hooks-check"); err != nil {
				return err
			}
		} else if err := command(ctx, root, nil, "node", "cmd/karty-check/testdata/client-hooks.test.mjs", distribution); err != nil {
			return err
		}
	}

	fmt.Fprintln(os.Stdout, "Client hooks crafted fixture passed: deterministic builds, authored actions and level remount")

	return nil
}

func prepareHooksFixture(repository, temporary string) error {
	if err := os.CopyFS(temporary, os.DirFS(filepath.Join(repository, "cmd/karty-check/testdata/client-hooks"))); err != nil {
		return err
	}

	configPath := filepath.Join(temporary, "karty.toml")

	config, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	config = fmt.Appendf(config, "\n[sdk]\nversion = %q\n", release.SDKVersion())
	//nolint:gosec // Fixed filename in a newly allocated private fixture directory.
	if err := os.WriteFile(configPath, config, 0600); err != nil {
		return err
	}

	// A single solid texel supplies every surface. No asset pipeline tool or
	// demo texture is needed to exercise the real camera and mounted actor.
	texture := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	texture.Set(0, 0, color.White)

	file, err := os.Create(filepath.Join(temporary, "levels/hooks/surface.png"))
	if err != nil {
		return err
	}

	encodeErr := png.Encode(file, texture)
	closeErr := file.Close()

	if encodeErr != nil {
		return encodeErr
	}

	if closeErr != nil {
		return closeErr
	}

	return nil
}
