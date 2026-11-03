package toolchain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/sdk"
)

func TestEnsureKTXRequiresExplicitCandidateExecutable(t *testing.T) {
	t.Parallel()

	if _, err := EnsureKTX(t.Context(), sdk.Manifest{}, ""); err == nil {
		t.Fatal("missing release pin silently selected a PATH encoder")
	}

	directory := t.TempDir()
	if _, err := EnsureKTX(t.Context(), sdk.Manifest{}, directory); err == nil {
		t.Fatal("directory selected as encoder")
	}

	name := filepath.Join(directory, "ktx")
	if err := os.WriteFile(name, []byte("candidate fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(name, 0o700); err != nil {
		t.Fatal(err)
	}

	actual, err := EnsureKTX(t.Context(), sdk.Manifest{}, name)
	if err != nil || actual != name {
		t.Fatal("explicit candidate override rejected", actual, err)
	}
}
