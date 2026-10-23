package testpackages

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPinsAreIndependentAndCoverPublishedPackages(t *testing.T) {
	t.Parallel()

	pins, err := Pins()
	if err != nil {
		t.Fatal(err)
	}

	//nolint:misspell // "dialogue" is the exact published package name.
	names := []string{"controllers-3d", "controllers-isometric", "controllers-topdown", "dialogue", "fps-kit", "savegame", "tiled"}

	if len(pins) != len(names) {
		t.Fatalf("got %d release pins", len(pins))
	}

	for _, name := range names {
		pin, ok := pins[name]
		if !ok || !strings.HasPrefix(pin.Archive, "https://github.com/karty-game/karty-packages/releases/download/"+name+"-v") {
			t.Fatalf("missing published archive for %s", name)
		}

		if digest, err := hex.DecodeString(pin.SHA256); err != nil || len(digest) != sha256.Size {
			t.Fatalf("invalid SHA-256 for %s", name)
		}
	}

	delete(pins, "fps-kit")

	again, err := Pins()
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := again["fps-kit"]; !ok {
		t.Fatal("caller mutation changed committed pins")
	}
}

func TestSourceRequiresPreparedRegularManifest(t *testing.T) {
	project := t.TempDir()

	t.Setenv("KARTY_TEST_PACKAGES", project)

	if _, err := Source("../fps-kit"); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("accepted unknown fixture: %v", err)
	}

	if _, err := Source("fps-kit"); !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "mise run prepare-test-packages") {
		t.Fatalf("missing fixture did not explain preparation: %v", err)
	}

	directory := filepath.Join(project, ".karty", "packages", "fps-kit")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := filepath.Join(directory, "karty-package.toml")
	if err := os.Mkdir(manifest, 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := Source("fps-kit"); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("accepted nonregular manifest: %v", err)
	}

	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(manifest, []byte("name = 'fps-kit'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Source("fps-kit")
	if err != nil || got != directory {
		t.Fatalf("source = %q, %v", got, err)
	}
}

func TestProjectRequiresAbsoluteOverride(t *testing.T) {
	t.Setenv("KARTY_TEST_PACKAGES", "relative/path")

	if _, err := Project(); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("accepted relative override: %v", err)
	}

	t.Setenv("KARTY_TEST_PACKAGES", "")

	project, err := Project()
	if err != nil || !filepath.IsAbs(project) || filepath.Base(project) != "released-packages" {
		t.Fatalf("default project = %q, %v", project, err)
	}
}

func TestArchiveVerificationRejectsChangedOrNonregularBytes(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fixture.kpkg")
	contents := []byte("immutable release fixture")
	digest := sha256.Sum256(contents)
	want := hex.EncodeToString(digest[:])

	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := verifyArchive(path, want); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("changed bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := verifyArchive(path, want); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("accepted changed bytes: %v", err)
	}

	if err := verifyArchive(filepath.Dir(path), want); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("accepted directory: %v", err)
	}
}
