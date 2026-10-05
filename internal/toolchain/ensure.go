// Package toolchain resolves and installs tools selected by a project's pinned SDK.
package toolchain

import (
	"context"
	"fmt"
	"runtime"

	"github.com/karty-game/karty/internal/sdk"
)

const errArtifactUnavailable staticError = "SDK does not provide this tool for the current platform"

// EnsureOptions selects the SDK tools needed by a command. An explicit tool
// path is deliberately never downloaded or replaced.
type EnsureOptions struct {
	GoOverride          string
	TinyGoOverride      string
	WasmToolsOverride   string
	AirOverride         string
	MaterializeOverride string
	NeedGo              bool
	NeedTinyGo          bool
	NeedWasmTools       bool
	NeedAir             bool
	NeedMaterialize     bool
}

// EnsurePaths reports the resolved locations of tools that Ensure checked.
type EnsurePaths struct {
	Go          string
	TinyGo      string
	WasmTools   string
	Air         string
	Materialize string
}

// Ensure verifies that the selected SDK tools are ready to run, downloading
// missing managed tools into the checksum-verified Karty cache. It does not
// download tools supplied through explicit command-line overrides.
func Ensure(ctx context.Context, manifest sdk.Manifest, options EnsureOptions) (EnsurePaths, error) {
	paths := EnsurePaths{}
	platform := runtime.GOOS + "-" + runtime.GOARCH

	// Reject invalid managed Materialize selections before any tool writes.
	if options.NeedMaterialize && options.MaterializeOverride == "" {
		if _, err := materializeSpecForPlatform(manifest, platform); err != nil {
			return EnsurePaths{}, err
		}
	}

	if options.NeedGo {
		path, err := Go(ctx, GoOptions{Override: options.GoOverride, Version: manifest.Tools.Go})
		if err != nil {
			return EnsurePaths{}, fmt.Errorf("resolve SDK Go: %w", err)
		}

		paths.Go = path
	}

	if options.NeedTinyGo {
		path, err := ensureTinyGo(ctx, manifest, options.TinyGoOverride, platform)
		if err != nil {
			return EnsurePaths{}, err
		}

		paths.TinyGo = path
	}

	if options.NeedWasmTools {
		path, err := ensureWasmTools(ctx, manifest, options.WasmToolsOverride, platform)
		if err != nil {
			return EnsurePaths{}, err
		}

		paths.WasmTools = path
	}

	if options.NeedAir {
		path, err := ensureAir(ctx, manifest, options.AirOverride, platform)
		if err != nil {
			return EnsurePaths{}, err
		}

		paths.Air = path
	}

	if options.NeedMaterialize {
		path, err := EnsureMaterialize(ctx, manifest, options.MaterializeOverride)
		if err != nil {
			return EnsurePaths{}, err
		}

		paths.Materialize = path
	}

	return paths, nil
}

func ensureTinyGo(ctx context.Context, manifest sdk.Manifest, override, platform string) (string, error) {
	if override != "" {
		return TinyGo(TinyGoOptions{Override: override, Version: manifest.Tools.TinyGo})
	}

	value, err := artifact(manifest.Version, "TinyGo", platform, manifest.Artifacts.TinyGo)
	if err != nil {
		return "", err
	}

	path, err := InstallTinyGo(ctx, InstallTinyGoOptions{Version: manifest.Tools.TinyGo, Artifact: value})
	if err != nil {
		return "", fmt.Errorf("install TinyGo: %w", err)
	}

	return path, nil
}

func ensureWasmTools(ctx context.Context, manifest sdk.Manifest, override, platform string) (string, error) {
	if override != "" {
		return executable("--wasm-tools", override)
	}

	value, err := artifact(manifest.Version, "wasm-tools", platform, manifest.Artifacts.WasmTools)
	if err != nil {
		return "", err
	}

	path, err := InstallWasmTools(ctx, InstallWasmToolsOptions{Version: manifest.Tools.WasmTools, Artifact: value})
	if err != nil {
		return "", fmt.Errorf("install wasm-tools: %w", err)
	}

	return path, nil
}

func ensureAir(ctx context.Context, manifest sdk.Manifest, override, platform string) (string, error) {
	if override != "" {
		return Air(AirOptions{Override: override, Version: manifest.Tools.Air})
	}

	value, err := artifact(manifest.Version, "Air", platform, manifest.Artifacts.Air)
	if err != nil {
		return "", err
	}

	path, err := InstallAir(ctx, InstallAirOptions{Version: manifest.Tools.Air, Artifact: value})
	if err != nil {
		return "", fmt.Errorf("install Air: %w", err)
	}

	return path, nil
}

func artifact(version, name, platform string, artifacts map[string]sdk.ToolArtifact) (sdk.ToolArtifact, error) {
	value, found := artifacts[platform]
	if !found {
		return sdk.ToolArtifact{}, fmt.Errorf("SDK %s does not provide %s for %s: %w", version, name, platform, errArtifactUnavailable)
	}

	return value, nil
}
