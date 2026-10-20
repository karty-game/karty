package devserver

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func startWatchChanges(t *testing.T, watcher *Watcher) (<-chan sourceChange, *atomic.Int64) {
	t.Helper()

	count := new(atomic.Int64)
	watcher.now = func() time.Time {
		count.Add(1)

		return time.Now()
	}
	ctx, cancel := context.WithCancel(t.Context())
	output := make(chan sourceChange, 1)

	done := make(chan struct{})
	go func() {
		defer close(done)

		watcher.changes(ctx, 5*time.Millisecond, output)
	}()

	t.Cleanup(func() {
		cancel()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("watch changes did not join after cancellation")
		}
	})

	return output, count
}

func nextWatchChange(t *testing.T, changes <-chan sourceChange) string {
	t.Helper()

	select {
	case change, open := <-changes:
		if !open || change.err != nil {
			t.Fatalf("watch change: open=%v error=%v", open, change.err)
		}

		return change.digest
	case <-time.After(2 * time.Second):
		t.Fatal("source change was missed")

		return ""
	}
}

func TestWatcherNotificationsSkipIdleScansAndTrackNewDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWatchFile(t, root, "src/main.go", "before")
	watcher := fixtureWatcher(t, root)
	changes, count := startWatchChanges(t, watcher)
	previous := nextWatchChange(t, changes)

	// Initial registration requires one follow-up scan; subsequent polls of
	// an idle project must not traverse the directory tree.
	time.Sleep(80 * time.Millisecond)

	if scans := count.Load(); scans > 3 {
		t.Fatalf("idle notification loop performed %d scans", scans)
	}

	for _, name := range []string{"src/main.go", "new/nested/source.go", "new/nested/source.go"} {
		writeWatchFile(t, root, name, previous)

		next := nextWatchChange(t, changes)
		if next == previous {
			t.Fatal("notification did not change source digest")
		}

		previous = next
	}

	if err := os.Remove(filepath.Join(root, "new", "nested", "source.go")); err != nil {
		t.Fatal(err)
	}

	nextWatchChange(t, changes)
}

func TestWatcherNotificationsExcludeGeneratedAndUnrelatedParentEvents(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	writeWatchFile(t, root, "src/main.go", "before")
	watcher := fixtureWatcher(t, root)
	changes, count := startWatchChanges(t, watcher)
	nextWatchChange(t, changes)
	time.Sleep(30 * time.Millisecond)

	before := count.Load()

	for _, name := range []string{".karty/output.go", "dist/game.kart", ".codex/config.toml", "src/editor.tmp"} {
		writeWatchFile(t, root, name, "generated")
	}

	writeWatchFile(t, parent, "sibling/file.go", "unrelated")
	time.Sleep(40 * time.Millisecond)

	if after := count.Load(); after != before {
		t.Fatalf("excluded notifications triggered scans: before=%d after=%d", before, after)
	}
}

func TestWatcherNotificationsTrackPackageDirectoryReplacement(t *testing.T) {
	t.Parallel()
	root, parent := t.TempDir(), t.TempDir()
	packageRoot := filepath.Join(parent, "kit")

	writeWatchFile(t, root, "src/main.go", "before")
	writeWatchFile(t, packageRoot, "lib.go", "before")
	watcher := fixtureWatcher(t, root)
	watcher.additionalRoots = func(context.Context) (map[string]string, error) {
		return map[string]string{"kit": packageRoot}, nil
	}
	changes, _ := startWatchChanges(t, watcher)

	before := nextWatchChange(t, changes)
	if err := os.Rename(packageRoot, filepath.Join(parent, "old-kit")); err != nil {
		t.Fatal(err)
	}

	writeWatchFile(t, packageRoot, "lib.go", "after replacement")

	if after := nextWatchChange(t, changes); after == before {
		t.Fatal("package directory replacement was missed")
	}
}

type fakeWatchNotifications struct {
	mutex  sync.Mutex
	paths  map[string]bool
	addErr error
}

func (fake *fakeWatchNotifications) Add(name string) error {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()

	fake.paths[name] = true

	return fake.addErr
}

func (fake *fakeWatchNotifications) Remove(name string) error {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()

	delete(fake.paths, name)

	return nil
}

func (fake *fakeWatchNotifications) WatchList() []string {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()

	paths := make([]string, 0, len(fake.paths))
	for name := range fake.paths {
		paths = append(paths, name)
	}

	return paths
}

func (*fakeWatchNotifications) Close() error { return nil }

func TestWatcherNotificationsFallBackOnSetupRegistrationAndRuntimeFailure(t *testing.T) {
	t.Parallel()

	for _, failure := range []string{"setup", "registration", "runtime", "closed events"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeWatchFile(t, root, "source.go", "before")
			watcher := fixtureWatcher(t, root)
			failures := make(chan error, 1)
			events := make(chan fsnotify.Event)
			watcher.newNotifications = func() (*watchNotifications, error) {
				if failure == "setup" {
					return nil, os.ErrInvalid
				}

				backend := &fakeWatchNotifications{paths: make(map[string]bool)}
				if failure == "registration" {
					backend.addErr = os.ErrInvalid
				}

				return &watchNotifications{watchNotificationBackend: backend, events: events, errors: failures}, nil
			}
			changes, _ := startWatchChanges(t, watcher)
			before := nextWatchChange(t, changes)

			switch failure {
			case "runtime":
				failures <- os.ErrInvalid
			case "closed events":
				close(events)
			}

			writeWatchFile(t, root, "source.go", "after fallback")

			if after := nextWatchChange(t, changes); after == before {
				t.Fatal("polling fallback missed source change")
			}
		})
	}
}

func TestWatcherPeriodicScanRecoversMissedNotifications(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWatchFile(t, root, "source.go", "before")
	watcher := fixtureWatcher(t, root)
	watcher.verificationInterval = 30 * time.Millisecond
	watcher.newNotifications = func() (*watchNotifications, error) {
		return &watchNotifications{watchNotificationBackend: &fakeWatchNotifications{paths: make(map[string]bool)}}, nil
	}
	changes, _ := startWatchChanges(t, watcher)
	before := nextWatchChange(t, changes)

	time.Sleep(15 * time.Millisecond)
	writeWatchFile(t, root, "source.go", "missed event")

	if after := nextWatchChange(t, changes); after == before {
		t.Fatal("periodic verification missed the unreported edit")
	}
}

func TestWatcherCachedScansDoNotPostponeFullVerification(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWatchFile(t, root, "source.go", "before")
	writeWatchFile(t, root, "trigger.go", "before")

	info, err := os.Stat(filepath.Join(root, "source.go"))
	if err != nil {
		t.Fatal(err)
	}

	watcher := fixtureWatcher(t, root)
	watcher.verificationInterval = 200 * time.Millisecond
	events := make(chan fsnotify.Event, 1)
	watcher.newNotifications = func() (*watchNotifications, error) {
		return &watchNotifications{
			watchNotificationBackend: &fakeWatchNotifications{paths: make(map[string]bool)},
			events:                   events,
		}, nil
	}
	changes, _ := startWatchChanges(t, watcher)
	nextWatchChange(t, changes)

	started := time.Now()

	time.Sleep(120 * time.Millisecond)
	writeWatchFile(t, root, "trigger.go", "trigger cached scan")

	events <- fsnotify.Event{Name: filepath.Join(root, "trigger.go"), Op: fsnotify.Write}

	before := nextWatchChange(t, changes)
	writeWatchFile(t, root, "source.go", "edited")

	if err := os.Chtimes(filepath.Join(root, "source.go"), info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	// The first periodic verification is due at 200 ms, even though the
	// notification-triggered scan reused this file's earlier cached hash.
	deadline := time.NewTimer(time.Until(started.Add(280 * time.Millisecond)))
	defer deadline.Stop()

	select {
	case change := <-changes:
		if change.err != nil || change.digest == before {
			t.Fatalf("periodic verification failed: %+v", change)
		}
	case <-deadline.C:
		t.Fatal("cached scan postponed full verification of an unchanged metadata file")
	}
}

func TestWatcherNotificationsRefreshReplacedAncestorDirectories(t *testing.T) {
	t.Parallel()
	root, parent := t.TempDir(), t.TempDir()
	ancestor := filepath.Join(parent, "repo")
	packageRoot := filepath.Join(ancestor, "kit")

	writeWatchFile(t, root, "source.go", "project")
	writeWatchFile(t, packageRoot, "lib.go", "before")
	watcher := fixtureWatcher(t, root)
	watcher.additionalRoots = func(context.Context) (map[string]string, error) {
		return map[string]string{"kit": packageRoot}, nil
	}
	changes, _ := startWatchChanges(t, watcher)
	before := nextWatchChange(t, changes)

	if err := os.Rename(ancestor, filepath.Join(parent, "old-repo")); err != nil {
		t.Fatal(err)
	}

	writeWatchFile(t, packageRoot, "lib.go", "replacement")

	if after := nextWatchChange(t, changes); after == before {
		t.Fatal("ancestor replacement did not trigger a source scan")
	}

	// Reconciliation must repair the descendant watch so later edits do not
	// wait for the ten-second periodic verification.
	writeWatchFile(t, packageRoot, "new.go", "new source")
	nextWatchChange(t, changes)
}
