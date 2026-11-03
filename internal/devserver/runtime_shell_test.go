package devserver

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWebGPUSnapshotUsesModulesAndPreservesLevelOnlyIdentity(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	write := func(level []byte) {
		t.Helper()
		fixtureBuild(t, directory, level, []byte("unchanged-code"), nil)

		if err := os.Remove(filepath.Join(directory, "karty.js")); err != nil {
			t.Fatal(err)
		}

		for name, data := range map[string]string{
			"index.html":    "<html><head></head><body><canvas id=world></canvas><script type=module src=bootstrap.mjs></script></body></html>",
			"bootstrap.mjs": "import './runtime.mjs';", "runtime.mjs": "export const runtime=1;",
			"client.mjs": "export const client=1;", "runtime.bin": "host",
		} {
			if err := os.WriteFile(filepath.Join(directory, name), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	write([]byte("level-first"))

	first := captureFixture(t, directory, 1)

	write([]byte("level-other"))

	second := captureFixture(t, directory, 2)
	if first.Generation.ID == second.Generation.ID || first.Generation.RuntimeIdentity != second.Generation.RuntimeIdentity {
		t.Fatal("WebGPU level-only edit lost its stable runtime identity")
	}

	if err := os.WriteFile(filepath.Join(directory, "runtime.mjs"), []byte("export const runtime=2;"), 0o600); err != nil {
		t.Fatal(err)
	}

	third := captureFixture(t, directory, 3)
	if second.Generation.RuntimeIdentity == third.Generation.RuntimeIdentity {
		t.Fatal("WebGPU runtime module change did not require restart")
	}

	if err := os.Remove(filepath.Join(directory, "client.mjs")); err != nil {
		t.Fatal(err)
	}

	if _, err := Capture(t.Context(), directory, 4); !errors.Is(err, ErrSnapshot) {
		t.Fatalf("incomplete WebGPU runtime accepted: %v", err)
	}
}
