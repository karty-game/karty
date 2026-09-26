package toolchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/karty-game/karty/internal/sdk"
)

const errAirVersion staticError = "Air version is not pinned by the selected SDK"
const errAirMetadata staticError = "Air installation metadata is incomplete"
const errAirChecksum staticError = "Air archive checksum mismatch"

// AirOptions identifies a managed Air executable or an explicit override.
type AirOptions struct {
	Override string
	Version  string
	CacheDir string
}

// Air resolves Air from an explicit override or the Karty tool cache.
func Air(options AirOptions) (string, error) {
	if options.Override != "" {
		return executable("--air", options.Override)
	}

	if options.Version == "" {
		return "", errAirVersion
	}

	cacheDir, err := kartyHome(options.CacheDir)
	if err != nil {
		return "", err
	}

	return executable(
		"managed Air",
		filepath.Join(cacheDir, "tools", "air", options.Version, runtime.GOOS+"-"+runtime.GOARCH, executableName("air")),
	)
}

// InstallAir downloads, verifies, and atomically installs Air.
func InstallAir(ctx context.Context, options InstallAirOptions) (string, error) {
	if options.Version == "" || options.Artifact.URL == "" || options.Artifact.SHA256 == "" {
		return "", errAirMetadata
	}

	cacheDir, err := kartyHome(options.CacheDir)
	if err != nil {
		return "", err
	}

	platform := options.Platform
	if platform == "" {
		platform = runtime.GOOS + "-" + runtime.GOARCH
	}

	installDir := filepath.Join(cacheDir, "tools", "air", options.Version, platform)

	executablePath := filepath.Join(installDir, executableName("air"))
	if path, resolveErr := executable("managed Air", executablePath); resolveErr == nil {
		return path, nil
	}

	if err := os.MkdirAll(filepath.Dir(installDir), 0o750); err != nil {
		return "", fmt.Errorf("create Air cache directory: %w", err)
	}

	download := options.Download
	if download == nil {
		download = downloadURL
	}

	archive, err := download(ctx, options.Artifact.URL)
	if err != nil {
		return "", fmt.Errorf("download Air %s: %w", options.Version, err)
	}
	defer archive.Close()

	contents, err := io.ReadAll(archive)
	if err != nil {
		return "", fmt.Errorf("read Air: %w", err)
	}

	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != options.Artifact.SHA256 {
		return "", fmt.Errorf("Air checksum = %x, want %s: %w", digest, options.Artifact.SHA256, errAirChecksum)
	}

	staging, err := os.MkdirTemp(filepath.Dir(installDir), ".air-")
	if err != nil {
		return "", fmt.Errorf("create Air staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	staged := filepath.Join(staging, executableName("air"))
	if err := os.WriteFile(staged, contents, 0o600); err != nil {
		return "", fmt.Errorf("stage Air: %w", err)
	}

	if err := os.Chmod(staged, 0o700); err != nil {
		return "", fmt.Errorf("make Air executable: %w", err)
	}

	if err := os.Rename(staging, installDir); err != nil {
		return "", fmt.Errorf("install Air: %w", err)
	}

	return executable("managed Air", executablePath)
}

// InstallAirOptions describes a verified Air release artifact.
type InstallAirOptions struct {
	Version  string
	Artifact sdk.ToolArtifact
	CacheDir string
	Platform string
	Download func(context.Context, string) (io.ReadCloser, error)
}
