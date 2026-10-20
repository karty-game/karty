package build

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWebDeliveryRejectsSymlinkOutputs(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"karty-web.js", "karty-sw.js", "karty-web-status.js"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			outside := filepath.Join(t.TempDir(), "user-file")

			if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}

			if err := os.Symlink(outside, filepath.Join(root, name)); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}

			if _, _, err := stageWebDelivery(t.Context(), root); !errors.Is(err, os.ErrInvalid) {
				t.Fatalf("symlink output accepted: %v", err)
			}

			contents, err := os.ReadFile(outside)
			if err != nil || string(contents) != "keep" {
				t.Fatalf("symlink target modified: %q, %v", contents, err)
			}
		})
	}
}
