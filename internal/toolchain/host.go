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

const errHostMetadata staticError = "host artifact metadata is incomplete"
const errHostChecksum staticError = "host artifact checksum mismatch"
const errHostNotFile staticError = "managed Karty web host is not a regular file"

// Host returns a cached SDK-pinned native or web host artifact.
func Host(version, target string) (string, error) {
	cacheDir, err := kartyHome("")
	if err != nil {
		return "", err
	}

	platform := runtime.GOOS + "-" + runtime.GOARCH
	name := "karty-host"

	switch target {
	case "web":
		platform = "web"
		name = "karty-host.wasm"
	case "web-runtime":
		platform = "web-runtime"
		name = "wasm_exec.js"
	}

	path := filepath.Join(cacheDir, "hosts", version, platform, name)
	if target == "web" || target == "web-runtime" {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return "", statErr
		}

		if !info.Mode().IsRegular() {
			return "", errHostNotFile
		}

		return path, nil
	}

	return executable("managed Karty host", path)
}

// InstallHost downloads, verifies, and atomically installs a raw host artifact.
func InstallHost(ctx context.Context, version, target string, artifact sdk.ToolArtifact) (string, error) {
	if version == "" || artifact.URL == "" || artifact.SHA256 == "" {
		return "", errHostMetadata
	}

	cacheDir, err := kartyHome("")
	if err != nil {
		return "", err
	}

	platform := runtime.GOOS + "-" + runtime.GOARCH
	name := "karty-host"
	mode := os.FileMode(0o700)

	switch target {
	case "web":
		platform = "web"
		name = "karty-host.wasm"
		mode = 0o600
	case "web-runtime":
		platform = "web-runtime"
		name = "wasm_exec.js"
		mode = 0o600
	}

	installDir := filepath.Join(cacheDir, "hosts", version, platform)

	path := filepath.Join(installDir, name)
	if target == "web" || target == "web-runtime" {
		if info, statErr := os.Stat(path); statErr == nil && info.Mode().IsRegular() {
			return path, nil
		}
	} else if managed, executableErr := executable("managed Karty host", path); executableErr == nil {
		return managed, nil
	}

	if err := os.MkdirAll(filepath.Dir(installDir), 0o750); err != nil {
		return "", fmt.Errorf("create host cache directory: %w", err)
	}

	contents, err := downloadHost(ctx, artifact)
	if err != nil {
		return "", err
	}

	staging, err := os.MkdirTemp(filepath.Dir(installDir), ".host-")
	if err != nil {
		return "", fmt.Errorf("create host staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	if err := os.WriteFile(filepath.Join(staging, name), contents, mode); err != nil {
		return "", fmt.Errorf("stage host: %w", err)
	}

	if err := os.Rename(staging, installDir); err != nil {
		return "", fmt.Errorf("install host: %w", err)
	}

	if target == "web" || target == "web-runtime" {
		return path, nil
	}

	return executable("managed Karty host", path)
}

func downloadHost(ctx context.Context, artifact sdk.ToolArtifact) ([]byte, error) {
	response, err := downloadURL(ctx, artifact.URL)
	if err != nil {
		return nil, fmt.Errorf("download Karty host: %w", err)
	}
	defer response.Close()

	contents, err := io.ReadAll(response)
	if err != nil {
		return nil, fmt.Errorf("read Karty host: %w", err)
	}

	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != artifact.SHA256 {
		return nil, fmt.Errorf("karty host checksum = %x, want %s: %w", digest, artifact.SHA256, errHostChecksum)
	}

	return contents, nil
}

// EnsureHost installs the host selected by the project SDK when that SDK
// publishes a checksum-pinned release artifact.
func EnsureHost(ctx context.Context, manifest sdk.Manifest, target string) (string, error) {
	if cached, err := Host(manifest.Host.Version, target); err == nil {
		return cached, nil
	}

	manifest, err := manifestWithPublishedHost(ctx, manifest, target)
	if err != nil {
		return "", err
	}

	if target == "web" {
		if manifest.Artifacts.Host.Web.URL == "" {
			return "", errHostMetadata
		}

		return InstallHost(ctx, manifest.Host.Version, target, manifest.Artifacts.Host.Web)
	}

	if target == "web-runtime" {
		return InstallHost(ctx, manifest.Host.Version, target, manifest.Artifacts.Host.WebRuntime)
	}

	platform := runtime.GOOS + "-" + runtime.GOARCH

	artifact, found := manifest.Artifacts.Host.Native[platform]
	if !found {
		return "", errHostMetadata
	}

	return InstallHost(ctx, manifest.Host.Version, target, artifact)
}

func manifestWithPublishedHost(ctx context.Context, manifest sdk.Manifest, target string) (sdk.Manifest, error) {
	hasHost := manifest.Artifacts.Host.Web.URL != ""
	if target == "web-runtime" {
		hasHost = manifest.Artifacts.Host.WebRuntime.URL != ""
	} else if target != "web" {
		_, hasHost = manifest.Artifacts.Host.Native[runtime.GOOS+"-"+runtime.GOARCH]
	}

	if hasHost {
		return manifest, nil
	}

	published, err := sdk.ResolvePublished(ctx, manifest.Version)
	if err != nil {
		return sdk.Manifest{}, fmt.Errorf("resolve published SDK %s host: %w", manifest.Version, err)
	}

	if published.Host.Version != manifest.Host.Version {
		return sdk.Manifest{}, errHostMetadata
	}

	return published, nil
}
