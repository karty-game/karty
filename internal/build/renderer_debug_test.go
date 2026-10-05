package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRendererDebugStagingOptInAndRemoval(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	for _, enabled := range []bool{false, true, false} {
		scripts, err := stageRendererDebugShell(directory, enabled)
		if err != nil {
			t.Fatal(err)
		}

		content, err := os.ReadFile(filepath.Join(directory, "renderer-debug.js"))
		if !enabled {
			if scripts != "" || !os.IsNotExist(err) {
				t.Fatal("disabled debug controls retained output")
			}

			continue
		}

		if err != nil || !strings.Contains(scripts, "kartyRendererDebugEnabled = true") ||
			!strings.Contains(scripts, "renderer-debug.js?v=") ||
			!strings.Contains(string(content), "karty-lightmap-open") || !strings.Contains(string(content), "karty-aa-toggle") {
			t.Fatalf("enabled browser diagnostics were not staged: %v", err)
		}
	}
}

func TestRendererDebugStagingRejectsSymlink(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	outside := filepath.Join(t.TempDir(), "owned.js")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outside, filepath.Join(directory, "renderer-debug.js")); err != nil {
		t.Fatal(err)
	}

	if _, err := stageRendererDebugShell(directory, true); err == nil {
		t.Fatal("debug staging followed a symlink")
	}

	content, err := os.ReadFile(outside)
	if err != nil || string(content) != "keep" {
		t.Fatal("debug staging changed an external file")
	}
}
