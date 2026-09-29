package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareMediaDirectoryRemovesOnlyOwnedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	content := filepath.Join(root, mediaContentDirectory)
	if err := os.Mkdir(content, 0o750); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"old.kaud", ".audio-incomplete", "movie.kvid", "level.kld"} {
		if err := os.WriteFile(filepath.Join(content, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := prepareMediaDirectory(root, ".kaud", ".audio-"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"old.kaud", ".audio-incomplete"} {
		if _, err := os.Stat(filepath.Join(content, name)); !os.IsNotExist(err) {
			t.Fatalf("stale audio file %q remains", name)
		}
	}

	for _, name := range []string{"movie.kvid", "level.kld"} {
		if _, err := os.Stat(filepath.Join(content, name)); err != nil {
			t.Fatalf("unrelated content %q was removed: %v", name, err)
		}
	}
}
