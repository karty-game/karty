package toolchain

import (
	"context"
	"fmt"
	"os"

	"github.com/karty-game/karty/internal/sdk"
)

// KTXSpec adapts the SDK's exact Khronos encoder release to the shared installer.
func KTXSpec(manifest sdk.Manifest) (MaterialToolSpec, error) {
	if err := sdk.ValidateKTX(manifest); err != nil {
		return MaterialToolSpec{}, fmt.Errorf("SDK %s KTX: %w", manifest.Version, err)
	}

	if manifest.Tools.KTX == "" {
		return MaterialToolSpec{}, fmt.Errorf(
			"SDK %s does not pin KTX; provide an explicit KARTY_KTX candidate executable: %w",
			manifest.Version,
			errArtifactUnavailable,
		)
	}

	result := MaterialToolSpec{Tool: MaterialToolKTX, Revision: manifest.Tools.KTXRevision,
		Artifacts: make(map[string]MaterialToolArtifact, len(manifest.Artifacts.KTX))}
	for platform, artifact := range manifest.Artifacts.KTX {
		result.Artifacts[platform] = MaterialToolArtifact{URL: artifact.URL, SHA256: artifact.SHA256,
			Format: artifact.Format, Executable: artifact.Executable}
	}

	return result, nil
}

// EnsureKTX selects only an explicitly supplied executable or a checksummed SDK
// tool archive. It never searches PATH or builds tools during a game build.
func EnsureKTX(ctx context.Context, manifest sdk.Manifest, override string) (string, error) {
	if override != "" {
		info, err := os.Stat(override)
		if err != nil {
			return "", fmt.Errorf("KTX override: %w", err)
		}

		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("KTX override is not a regular file: %w", os.ErrInvalid)
		}

		return executable("KTX override", override)
	}

	selected, err := KTXSpec(manifest)
	if err != nil {
		return "", err
	}

	return InstallMaterialTool(ctx, InstallMaterialToolOptions{Spec: selected})
}
