package toolchain

import "testing"

func TestHostPlatformRejectsUnsupportedOrUnsafeTargets(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"../../escape", "darwin-amd64", "linux-386", "/tmp/host"} {
		if _, err := hostPlatform(target); err == nil {
			t.Fatalf("accepted %s", target)
		}
	}
}
