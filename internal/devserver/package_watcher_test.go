package devserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWatcherTracksExplicitPackageRootsWithoutRelaxingProjectConfinement(t *testing.T) {
	t.Parallel()
	project, library, other := t.TempDir(), t.TempDir(), t.TempDir()
	writeWatchFile(t, project, "src/main.go", "game")
	writeWatchFile(t, library, "controller.go", "before")
	writeWatchFile(t, library, "assets/wood.png", "before")
	writeWatchFile(t, other, "controller.go", "different package")
	watcher := fixtureWatcher(t, project)
	roots := map[string]string{"std": library}
	watcher.additionalRoots = func(context.Context) (map[string]string, error) { return roots, nil }
	before := scanDigest(t, watcher)
	writeWatchFile(t, library, "controller.go", "edited shared Go")

	after := scanDigest(t, watcher)
	if before == after {
		t.Fatal("missed external Go edit")
	}

	writeWatchFile(t, library, "assets/wood.png", "edited shared PNG")

	next := scanDigest(t, watcher)
	if after == next {
		t.Fatal("missed external asset edit")
	}

	writeWatchFile(t, project, ".karty/packages/cache", "generated")
	writeWatchFile(t, library, ".karty/generated", "generated")

	if scanDigest(t, watcher) != next {
		t.Fatal("generated output triggered a rebuild")
	}

	roots["std"] = other

	if scanDigest(t, watcher) == next {
		t.Fatal("missed package source replacement")
	}

	delete(roots, "std")
	scanDigest(t, watcher)

	if len(watcher.children) != 0 {
		t.Fatal("removed package watcher remained open")
	}

	if err := os.Symlink(library, filepath.Join(project, "escape")); err != nil {
		t.Fatal(err)
	}

	if _, err := watcher.Scan(t.Context()); err == nil {
		t.Fatal("declared package capability relaxed ordinary project confinement")
	}
}

func TestPackageRootReplacementReopensItsDirectoryAndRejectsEscapingLinks(t *testing.T) {
	t.Parallel()
	project, parent := t.TempDir(), t.TempDir()
	library := filepath.Join(parent, "kit")
	writeWatchFile(t, library, "source.go", "old inode")
	watcher := fixtureWatcher(t, project)
	watcher.additionalRoots = func(context.Context) (map[string]string, error) { return map[string]string{"kit": library}, nil }

	before := scanDigest(t, watcher)
	if err := os.Rename(library, filepath.Join(parent, "old")); err != nil {
		t.Fatal(err)
	}

	writeWatchFile(t, library, "source.go", "new inode")

	if scanDigest(t, watcher) == before {
		t.Fatal("kept watching the displaced directory")
	}

	writeWatchFile(t, parent, "outside", "private")

	if err := os.Symlink("../outside", filepath.Join(library, "escape")); err != nil {
		t.Fatal(err)
	}

	if _, err := watcher.Scan(t.Context()); err == nil {
		t.Fatal("package read escaped its own root")
	}

	if err := os.Remove(filepath.Join(library, "escape")); err != nil {
		t.Fatal(err)
	}

	scanDigest(t, watcher)
}
