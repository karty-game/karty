package devserver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWatcherRevalidatesContentWhenAnEditPreservesMetadata(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWatchFile(t, root, "source.go", "before")
	watcher := fixtureWatcher(t, root)
	now := time.Unix(100, 0)
	watcher.now = func() time.Time { return now }
	before := scanDigest(t, watcher)
	file := filepath.Join(root, "source.go")

	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	writeWatchFile(t, root, "source.go", "edited")

	if err := os.Chtimes(file, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	now = now.Add(maxWatchHashAge - time.Nanosecond)

	if digest := scanDigest(t, watcher); digest != before {
		t.Fatal("unchanged metadata should retain the cheap cached path before expiry")
	}

	now = now.Add(time.Nanosecond)

	after := scanDigest(t, watcher)
	if after == before {
		t.Fatal("expired cached hash missed a metadata-preserving edit")
	}

	if !watcher.files["source.go"].hashedAt.Equal(now) {
		t.Fatal("content verification age did not advance")
	}

	now = now.Add(maxWatchHashAge)

	if digest := scanDigest(t, watcher); digest != after {
		t.Fatal("a timestamp-only verification created a false source edit")
	}
}

func TestCanceledPeriodicVerificationDoesNotRefreshCachedHashAge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWatchFile(t, root, "asset.dat", strings.Repeat("x", contextChunkBytes*4))
	watcher := fixtureWatcher(t, root)
	now := time.Unix(100, 0)
	watcher.now = func() time.Time { return now }
	scanDigest(t, watcher)
	verified := watcher.files["asset.dat"].hashedAt
	now = now.Add(maxWatchHashAge)

	if _, err := watcher.Scan(canceledDuringChecks(10)); !errors.Is(err, context.Canceled) {
		t.Fatalf("periodic hash cancellation: %v", err)
	}

	if !watcher.files["asset.dat"].hashedAt.Equal(verified) {
		t.Fatal("canceled scan refreshed unverified cached metadata")
	}

	scanDigest(t, watcher)

	if !watcher.files["asset.dat"].hashedAt.Equal(now) {
		t.Fatal("completed verification did not refresh cached hash age")
	}
}

func writeWatchFile(t *testing.T, root, name, contents string) {
	t.Helper()

	file := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(file, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureWatcher(t *testing.T, root string) *Watcher {
	t.Helper()

	watcher, err := NewWatcher(root)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = watcher.Close() })

	return watcher
}

func scanDigest(t *testing.T, watcher *Watcher) string {
	t.Helper()

	digest, err := watcher.Scan(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return digest
}

func TestWatcherTracksContentNamesAndInternalGoSources(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWatchFile(t, root, "src/main.go", "package main")
	writeWatchFile(t, root, "internal/game/settings.go", "package game")
	watcher := fixtureWatcher(t, root)
	previous := scanDigest(t, watcher)
	path := filepath.Join(root, "src", "main.go")
	now := time.Now().Add(time.Hour)

	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}

	if got := scanDigest(t, watcher); got != previous {
		t.Fatal("timestamp-only save triggered a build")
	}

	for _, name := range []string{
		".git/config", ".karty/engine/config.go", "dist/web/game.kart", ".codex/config.toml", ".agents/task.md",
		"node_modules/module/index.js", ".idea/config", ".vscode/settings.json", "src/api_codegen.go", "src/file.swp",
	} {
		writeWatchFile(t, root, name, "generated")
	}

	if got := scanDigest(t, watcher); got != previous {
		t.Fatal("generated output triggered a build")
	}

	for _, change := range []func(){
		func() { writeWatchFile(t, root, "internal/game/settings.go", "package game\nconst speed = 2") },
		func() { writeWatchFile(t, root, "levels/room/world.yaml", "version: 4") },
		func() {
			if err := os.Rename(filepath.Join(root, "levels/room/world.yaml"), filepath.Join(root, "levels/room/renamed.yaml")); err != nil {
				t.Fatal(err)
			}
		},
		func() {
			if err := os.Remove(filepath.Join(root, "levels/room/renamed.yaml")); err != nil {
				t.Fatal(err)
			}
		},
	} {
		change()

		next := scanDigest(t, watcher)
		if next == previous {
			t.Fatal("source edit, addition, rename or deletion was missed")
		}

		previous = next
	}
}

func TestWatcherDetectsAtomicReplacementWithPreservedMetadata(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWatchFile(t, root, "src/main.go", "before")
	watcher := fixtureWatcher(t, root)
	before := scanDigest(t, watcher)
	path := filepath.Join(root, "src", "main.go")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	writeWatchFile(t, root, "src/replacement.tmp", "after!")

	replacement := filepath.Join(root, "src", "replacement.tmp")
	if err := os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}

	if got := scanDigest(t, watcher); got == before {
		t.Fatal("atomic replacement reused the removed file's digest")
	}
}

func TestWatcherConfinesSymlinksAndRejectsDirectoryCycles(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"file escape", "directory escape", "directory cycle", "link cycle"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeWatchFile(t, root, "src/main.go", "package main")
			outside := t.TempDir()
			writeWatchFile(t, outside, "secret.go", "outside project")
			target := map[string]string{
				"file escape": filepath.Join(outside, "secret.go"), "directory escape": outside,
				"directory cycle": "..", "link cycle": "link",
			}[kind]

			if err := os.Symlink(target, filepath.Join(root, "src", "link")); err != nil {
				t.Fatal(err)
			}

			if _, err := fixtureWatcher(t, root).Scan(t.Context()); err == nil {
				t.Fatal("escaping or cyclic link was accepted")
			}
		})
	}
}

func TestWatcherAllowsConfinedLinksAndExcludesGeneratedTargets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWatchFile(t, root, "src/main.go", "package main")
	writeWatchFile(t, root, ".karty/generated/config.go", "generated")
	watcher := fixtureWatcher(t, root)
	before := scanDigest(t, watcher)

	for link, target := range map[string]string{"generated-link.go": ".karty/generated/config.go", "generated-directory": ".karty/generated"} {
		if err := os.Symlink(filepath.FromSlash(target), filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}

	if got := scanDigest(t, watcher); got != before {
		t.Fatal("symlink aliases leaked generated output into watched sources")
	}

	for link, target := range map[string]string{"source-alias": "src", "file-alias.go": "src/main.go"} {
		if err := os.Symlink(filepath.FromSlash(target), filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}

		got := scanDigest(t, watcher)
		if got == before {
			t.Fatal("confined source alias was omitted")
		}

		before = got
	}
}

func TestWatcherCancellationAndFileSizeBounds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWatchFile(t, root, "asset.bin", "small")
	watcher := fixtureWatcher(t, root)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := watcher.Scan(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled scan: %v", err)
	}

	if err := os.Truncate(filepath.Join(root, "asset.bin"), maxWatchFileBytes+1); err != nil {
		t.Fatal(err)
	}

	if _, err := watcher.Scan(t.Context()); !errors.Is(err, errWatchBounds) {
		t.Fatalf("oversized sparse source accepted: %v", err)
	}
}
