package levelbuild

import (
	"os"
	"testing"
)

func writePrebakeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	// #nosec G703 -- all callers construct paths inside isolated t.TempDir fixtures.
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
