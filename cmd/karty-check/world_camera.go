package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/karty-game/karty/internal/build"
)

var (
	errNativeHostMissing = errors.New("KARTY_HOST_NATIVE must name the native host artifact")
	errWebHostMissing    = errors.New("KARTY_HOST_WEB must name the browser host artifact")
	errHostCheckMissing  = errors.New("KARTY_HOST_CHECK must name the karty-host-check executable")
	errWebCheckMissing   = errors.New("KARTY_WORLD_CAMERA_WEB_CHECK must name the browser acceptance script")
	errBuildChanged      = errors.New("repeated world-camera build produced different distribution bytes")
)

func runWorldCamera(ctx context.Context, root string) error {
	host := os.Getenv("KARTY_HOST_NATIVE")
	if host == "" {
		return errNativeHostMissing
	}

	webHost := os.Getenv("KARTY_HOST_WEB")
	if webHost == "" {
		return errWebHostMissing
	}

	hostCheck := os.Getenv("KARTY_HOST_CHECK")
	if hostCheck == "" {
		return errHostCheckMissing
	}

	webCheck := os.Getenv("KARTY_WORLD_CAMERA_WEB_CHECK")
	if webCheck == "" {
		return errWebCheckMissing
	}

	temporary, err := os.MkdirTemp("", "karty-world-camera-check-")
	if err != nil {
		return err
	}

	if os.Getenv("KARTY_CHECK_KEEP_PROJECT") == "1" {
		fmt.Fprintln(os.Stdout, "Retained check project:", filepath.Join(temporary, "world-camera"))
	} else {
		defer os.RemoveAll(temporary)
	}

	project := filepath.Join(temporary, "world-camera")
	if err := copyWorldCameraProject(root, project); err != nil {
		return err
	}

	nativeOptions := build.Options{
		Go:        os.Getenv("KARTY_GO"),
		TinyGo:    os.Getenv("KARTY_TINYGO"),
		WasmTools: os.Getenv("KARTY_WASM_TOOLS"),
		Host:      host,
		Target:    "native",
	}

	if err := build.RunWithOptions(ctx, project, nativeOptions); err != nil {
		return fmt.Errorf("build world-camera sample: %w", err)
	}

	distribution := filepath.Join(project, "dist", "native")

	firstDigest, err := distributionDigest(distribution)
	if err != nil {
		return err
	}

	if err := build.RunWithOptions(ctx, project, nativeOptions); err != nil {
		return fmt.Errorf("repeat world-camera sample build: %w", err)
	}

	secondDigest, err := distributionDigest(distribution)
	if err != nil {
		return err
	}

	if firstDigest != secondDigest {
		return fmt.Errorf("first %s, second %s: %w", firstDigest, secondDigest, errBuildChanged)
	}

	if err := command(ctx, distribution, nil, hostCheck, "--cartridge", "game.kart", "--camera-switch-check"); err != nil {
		return err
	}

	webOptions := nativeOptions
	webOptions.Host = webHost
	webOptions.Target = "web"

	if err := build.RunWithOptions(ctx, project, webOptions); err != nil {
		return fmt.Errorf("build browser world-camera sample: %w", err)
	}

	webDistribution := filepath.Join(project, "dist", "web")

	firstWebDigest, err := distributionDigest(webDistribution)
	if err != nil {
		return err
	}

	if err := build.RunWithOptions(ctx, project, webOptions); err != nil {
		return fmt.Errorf("repeat browser world-camera sample build: %w", err)
	}

	secondWebDigest, err := distributionDigest(webDistribution)
	if err != nil {
		return err
	}

	if firstWebDigest != secondWebDigest {
		return fmt.Errorf("browser first %s, second %s: %w", firstWebDigest, secondWebDigest, errBuildChanged)
	}

	return command(ctx, project, nil, "node", webCheck, webDistribution)
}

func distributionDigest(root string) (string, error) {
	paths := make([]string, 0, 4)

	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.Type().IsRegular() {
			paths = append(paths, path)
		}

		return nil
	}); err != nil {
		return "", fmt.Errorf("walk world-camera distribution: %w", err)
	}

	slices.Sort(paths)

	digest := sha256.New()

	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}

		digest.Write([]byte(filepath.ToSlash(relative)))
		digest.Write([]byte{0})
		digest.Write(contents)
	}

	return hex.EncodeToString(digest.Sum(nil)), nil
}

func copyWorldCameraProject(root, destination string) error {
	source := filepath.Join(root, "samples", "world-camera")

	if err := os.MkdirAll(destination, 0o750); err != nil {
		return err
	}

	for _, name := range []string{"go.mod", "karty.toml"} {
		contents, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			return err
		}
		//nolint:gosec // name is selected from the fixed project-root file list above.
		if err := os.WriteFile(filepath.Join(destination, name), contents, 0o600); err != nil {
			return err
		}
	}

	for _, name := range []string{"levels", "src", "ui"} {
		if err := os.CopyFS(filepath.Join(destination, name), os.DirFS(filepath.Join(source, name))); err != nil {
			return err
		}
	}

	return nil
}
