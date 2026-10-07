package sdk

import (
	"context"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
)

func TestUnsupportedSDKRejectedBeforeIO(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"0.0.1", "0.0.2", "0.0.3", "0.0.4", "0.0.5-rc1"} {
		if _, err := Resolve(version); err == nil || !strings.Contains(err.Error(), MinimumVersion) {
			t.Fatalf("Resolve(%s): %v", version, err)
		}

		if err := InstallBundle(version, nil, ""); err == nil || !strings.Contains(err.Error(), MinimumVersion) {
			t.Fatalf("InstallBundle(%s): %v", version, err)
		}

		if _, err := ResolvePublished(context.Background(), version); err == nil || !strings.Contains(err.Error(), MinimumVersion) {
			t.Fatalf("ResolvePublished(%s): %v", version, err)
		}
	}

	for _, version := range []string{MinimumVersion, release.SDKVersion(), "0.1.0", "1.0.0", "1.0.0-rc1"} {
		if err := validateVersion(version); err != nil {
			t.Fatalf("supported %s: %v", version, err)
		}
	}
}

func TestSDKVersionRejectsMalformedHigherVersions(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"1.00.0", "1.0.0007", "1.0.18446744073709551616", "v0.0.7", "../0.0.7"} {
		if err := validateVersion(version); err == nil {
			t.Fatalf("accepted invalid %q", version)
		}
	}
}
