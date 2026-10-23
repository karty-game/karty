package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLevelAssetsUseLogicalNamesWithoutReadingResources(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writeLevelNameFixture(t, directory, "room", "levels.room")
	writeLevelNameFixture(t, directory, "different-folder", "art.opening")

	generated, err := LevelAssetPackageFile(t.Context(), directory, "example.com/game/.karty/engine")
	if err != nil {
		t.Fatal(err)
	}

	for _, declaration := range []string{`LevelRoom`, `LevelArtOpening`, `engine.LevelName = "levels.room"`, `engine.LevelName = "art.opening"`} {
		if !strings.Contains(string(generated), declaration) {
			t.Fatalf("missing %q in generated levels:\n%s", declaration, generated)
		}
	}

	if strings.Index(string(generated), "LevelArtOpening") > strings.Index(string(generated), "LevelRoom") {
		t.Fatal("level constants do not follow sorted logical names")
	}
}

func TestLevelAssetsRejectDuplicateNamesAndIdentifiers(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"levels.room", "art.opening"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			writeLevelNameFixture(t, directory, "first", name)
			writeLevelNameFixture(t, directory, "second", name)

			if _, err := LevelNames(t.Context(), directory); !errors.Is(err, errDuplicateLevelName) {
				t.Fatalf("duplicate logical name accepted: %v", err)
			}
		})
	}

	for _, names := range [][]string{{"levels.art-opening", "art.opening"}, {"levels.a-b", "levels.a_b"}} {
		if _, err := levelAssetPackageFile(names, "example.com/game/.karty/engine"); !errors.Is(err, errLevelIdentifier) {
			t.Fatalf("ambiguous constants accepted for %q: %v", names, err)
		}
	}
}

func TestLevelAssetsEmptyAndCanceledDiscovery(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	generated, err := LevelAssetPackageFile(t.Context(), directory, "example.com/game/.karty/engine")
	if err != nil || strings.Contains(string(generated), "import") {
		t.Fatalf("empty level catalog: %s, %v", generated, err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := LevelNames(ctx, directory); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled discovery: %v", err)
	}
}

func TestLevelAssetsConfineManifestReads(t *testing.T) {
	t.Parallel()

	directory, outside := t.TempDir(), t.TempDir()
	writeLevelNameFixture(t, outside, "secret", "levels.secret")

	level := filepath.Join(directory, "levels", "escape")
	if err := os.MkdirAll(level, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(filepath.Join(outside, "levels", "secret", "level.toml"), filepath.Join(level, "level.toml")); err != nil {
		t.Fatal(err)
	}

	if _, err := LevelNames(t.Context(), directory); err == nil {
		t.Fatal("read a level manifest outside the project")
	}

	link := filepath.Join(level, "level.toml")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}

	writeLevelNameFixture(t, directory, "neighbor", "levels.neighbor")

	if err := os.Symlink(filepath.Join(directory, "levels", "neighbor", "level.toml"), link); err != nil {
		t.Fatal(err)
	}

	if _, err := LevelNames(t.Context(), directory); err == nil {
		t.Fatal("read a level manifest outside its own level directory")
	}
}

func writeLevelNameFixture(t *testing.T, root, directory, name string) {
	t.Helper()

	path := filepath.Join(root, "levels", directory)
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatal(err)
	}
	// Missing world and texture sources prove discovery stays metadata-only.
	source := "[level]\nname = " + `"` + name + `"` + "\nkind = \"world\"\n[world]\nsource = \"missing.yaml\"\n"
	if err := os.WriteFile(filepath.Join(path, "level.toml"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}
