package toolchain

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/karty-game/karty/internal/sdk"
)

// MaterializeSpec adapts a complete SDK pin without changing installer bounds.
// An absent pin is valid SDK metadata, but cannot select a managed installation.
func MaterializeSpec(manifest sdk.Manifest) (MaterialToolSpec, error) {
	if err := sdk.ValidateMaterialize(manifest); err != nil {
		return MaterialToolSpec{}, fmt.Errorf("SDK %s Materialize: %w", manifest.Version, err)
	}

	if manifest.Tools.Materialize == "" {
		return MaterialToolSpec{}, fmt.Errorf("SDK %s does not pin Materialize: %w", manifest.Version, errArtifactUnavailable)
	}

	spec := MaterialToolSpec{
		Tool: MaterialToolMaterialize, Revision: manifest.Tools.MaterializeRevision,
		Artifacts: make(map[string]MaterialToolArtifact, len(manifest.Artifacts.Materialize)),
	}
	for platform, artifact := range manifest.Artifacts.Materialize {
		spec.Artifacts[platform] = MaterialToolArtifact{
			URL: artifact.URL, SHA256: artifact.SHA256, Format: artifact.Format, Executable: artifact.Executable,
		}
	}

	return spec, nil
}

func materializeSpecForPlatform(manifest sdk.Manifest, platform string) (MaterialToolSpec, error) {
	spec, err := MaterializeSpec(manifest)
	if err != nil {
		return MaterialToolSpec{}, err
	}

	if _, err := materialArtifact(spec, platform); err != nil {
		return MaterialToolSpec{}, fmt.Errorf("SDK %s Materialize for %s: %w", manifest.Version, platform, err)
	}

	return spec, nil
}

// EnsureMaterialize selects only SDK-pinned release binaries or an explicit
// executable override. It never searches PATH, builds source or executes tools.
func EnsureMaterialize(ctx context.Context, manifest sdk.Manifest, override string) (string, error) {
	if override != "" {
		info, err := os.Stat(override)
		if err != nil {
			return "", fmt.Errorf("materialize override: %w", err)
		}

		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("materialize override is not a regular file: %w", os.ErrInvalid)
		}

		return executable("Materialize override", override)
	}

	spec, err := materializeSpecForPlatform(manifest, runtime.GOOS+"-"+runtime.GOARCH)
	if err != nil {
		return "", err
	}

	path, err := InstallMaterialTool(ctx, InstallMaterialToolOptions{Spec: spec})
	if err != nil {
		return "", fmt.Errorf("install Materialize: %w", err)
	}

	return path, nil
}
