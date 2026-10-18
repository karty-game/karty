package devserver

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

var (
	errFixtureBuild = errors.New("fixture compiler error")
	errFixtureWatch = errors.New("fixture watch error")
)

func TestCanceledSchedulerDoesNotStartDebouncedBuild(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	loop := &buildLoop{pending: &sourceChange{digest: "pending"}, options: RunOptions{Build: func(context.Context, func(string)) error {
		t.Error("canceled scheduler started a compiler")

		return nil
	}}}
	loop.start(ctx)

	if loop.active != nil || loop.revision != 0 {
		t.Fatal("canceled debounce launched a build goroutine")
	}

	results := make(chan buildResult, 1)
	buildGeneration(ctx, loop.options, "pending", 1, results, make(chan buildProgress, 1))

	if result := <-results; !errors.Is(result.err, context.Canceled) || result.snapshot != nil {
		t.Fatal("canceled generation called build or captured staged files")
	}
}

type testBuild struct {
	finish   chan error
	canceled chan struct{}
	release  chan struct{}
}

type runnerFixture struct {
	root     string
	watcher  *Watcher
	server   *Server
	changes  chan sourceChange
	started  chan *testBuild
	finished chan error
	cancel   context.CancelFunc
	shutdown chan struct{}
}

func newRunnerFixture(t *testing.T) *runnerFixture {
	t.Helper()
	root := t.TempDir()
	writeWatchFile(t, root, "src/main.go", "initial game")
	fixtureBuild(t, filepath.Join(root, "dist", "web"), []byte("level"), []byte("code"), nil)
	fixture := &runnerFixture{
		root: root, watcher: fixtureWatcher(t, root), server: fixtureServer(t), changes: make(chan sourceChange, 8),
		started: make(chan *testBuild, 8), finished: make(chan error, 1), shutdown: make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(t.Context())
	fixture.cancel = cancel
	options := RunOptions{Directory: root, Debounce: time.Millisecond, Build: fixture.build}

	go func() { fixture.finished <- runBuilds(ctx, fixture.server, fixture.watcher, options, fixture.changes) }()

	t.Cleanup(func() {
		cancel()
		close(fixture.shutdown)

		select {
		case err := <-fixture.finished:
			if err != nil {
				t.Errorf("runner shutdown: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("runner did not join its build")
		}
	})

	return fixture
}

func (fixture *runnerFixture) build(ctx context.Context, _ func(string)) error {
	attempt := &testBuild{finish: make(chan error, 1), canceled: make(chan struct{}), release: make(chan struct{})}
	fixture.started <- attempt

	select {
	case err := <-attempt.finish:
		return err
	case <-ctx.Done():
		close(attempt.canceled)

		select {
		case <-attempt.release:
		case <-fixture.shutdown:
		}

		return ctx.Err()
	}
}

func (fixture *runnerFixture) change(t *testing.T, text string) {
	t.Helper()
	writeWatchFile(t, fixture.root, "src/main.go", text)

	current := scanDigest(t, fixture.watcher)
	fixture.changes <- sourceChange{digest: current}
}

func nextBuild(t *testing.T, fixture *runnerFixture) *testBuild {
	t.Helper()

	select {
	case attempt := <-fixture.started:
		return attempt
	case <-time.After(time.Second):
		t.Fatal("queued development build did not start")
	}

	return nil
}

func waitForRunnerStatus(t *testing.T, server *Server, state string, revision uint64) {
	t.Helper()

	updates := make(chan []byte, 32)

	server.mu.Lock()
	server.clients[updates] = struct{}{}
	server.mu.Unlock()

	defer func() {
		server.mu.Lock()
		delete(server.clients, updates)
		server.mu.Unlock()
	}()

	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()

	for {
		server.mu.Lock()
		status := server.status
		server.mu.Unlock()

		if status.State == state && (revision == 0 || status.Generation != nil && status.Generation.Revision == revision) {
			return
		}

		select {
		case <-updates:
		case <-deadline.C:
			t.Fatalf("wanted %s build %d; got %+v", state, revision, status)
		}
	}
}

func TestRunnerJoinsSupersededBuildBeforeStartingNext(t *testing.T) {
	t.Parallel()
	fixture := newRunnerFixture(t)
	fixture.change(t, "first")
	first := nextBuild(t, fixture)
	fixture.change(t, "second")

	select {
	case <-first.canceled:
	case <-time.After(time.Second):
		t.Fatal("superseded build was not canceled")
	}

	select {
	case <-fixture.started:
		t.Fatal("new build started before old cleanup finished")
	default:
	}

	close(first.release)

	second := nextBuild(t, fixture)
	second.finish <- nil

	waitForRunnerStatus(t, fixture.server, "ready", 2)
}

func TestRunnerRetainsLastGoodBuildOnFailureAndRecoversWatchErrors(t *testing.T) {
	t.Parallel()
	fixture := newRunnerFixture(t)
	fixture.change(t, "working game")

	first := nextBuild(t, fixture)
	first.finish <- nil

	waitForRunnerStatus(t, fixture.server, "ready", 1)

	fixture.change(t, "broken game")

	second := nextBuild(t, fixture)
	second.finish <- errFixtureBuild

	waitForRunnerStatus(t, fixture.server, "error", 1)

	fixture.changes <- sourceChange{err: errFixtureWatch}

	waitForRunnerStatus(t, fixture.server, "error", 1)

	fixture.changes <- sourceChange{digest: scanDigest(t, fixture.watcher)}

	third := nextBuild(t, fixture)
	third.finish <- nil

	waitForRunnerStatus(t, fixture.server, "ready", 3)
}

func TestRunnerSuppressesStaleCaptureAndDeduplicatesFreshnessScan(t *testing.T) {
	t.Parallel()
	fixture := newRunnerFixture(t)
	fixture.change(t, "old source")
	first := nextBuild(t, fixture)

	// No poll event arrives until after completion; the freshness scan must
	// discard the otherwise successful captured build and schedule the edit.
	writeWatchFile(t, fixture.root, "src/main.go", "new source")

	first.finish <- nil

	second := nextBuild(t, fixture)
	fixture.server.mu.Lock()
	published := fixture.server.status.Generation
	fixture.server.mu.Unlock()

	if published != nil {
		t.Fatal("obsolete captured build was published")
	}

	// The poller's later identical digest must not cancel this replacement.
	loop := &buildLoop{latestDigest: scanDigest(t, fixture.watcher), active: func() { t.Error("duplicate source canceled active build") }}
	loop.changed(sourceChange{digest: loop.latestDigest})

	fixture.changes <- sourceChange{digest: loop.latestDigest}

	second.finish <- nil

	waitForRunnerStatus(t, fixture.server, "ready", 2)
}

func TestRunnerShutdownWaitsForBuildCleanup(t *testing.T) {
	t.Parallel()
	fixture := newRunnerFixture(t)
	fixture.change(t, "building")
	attempt := nextBuild(t, fixture)
	fixture.cancel()

	select {
	case <-attempt.canceled:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel active build")
	}

	select {
	case err := <-fixture.finished:
		t.Fatalf("runner returned before cancellation cleanup: %v", err)
	default:
	}

	close(attempt.release)
}

func TestRunnerProgressCannotOverwriteWatchErrorsOrNewerBuilds(t *testing.T) {
	t.Parallel()
	server := fixtureServer(t)
	loop := &buildLoop{server: server, latestDigest: "working", revision: 1, active: func() {}}
	loop.progressed(buildProgress{revision: 1, digest: "working", message: "current progress"})
	server.mu.Lock()
	message := server.status.Message
	server.mu.Unlock()

	if message != "current progress" {
		t.Fatal("active build progress was discarded")
	}

	// Model a callback that passed its context check before cancellation but
	// enqueued its progress only after the source error was handled.
	loop.changed(sourceChange{err: errFixtureWatch})
	loop.progressed(buildProgress{revision: 1, digest: "working", message: "late progress"})
	server.mu.Lock()
	state, message := server.status.State, server.status.Message
	server.mu.Unlock()

	if state != "error" || message != errFixtureWatch.Error() {
		t.Fatal("obsolete progress replaced the watch diagnostic")
	}

	loop.latestDigest, loop.revision = "working", 2

	server.SetStatus("building", "newer build")
	loop.progressed(buildProgress{revision: 1, digest: "working", message: "old build"})
	server.mu.Lock()
	message = server.status.Message
	server.mu.Unlock()

	if message != "newer build" {
		t.Fatal("older revision replaced current build progress")
	}
}

func TestRunnerDoesNotPublishIncompleteBuildArtifacts(t *testing.T) {
	t.Parallel()
	fixture := newRunnerFixture(t)
	fixture.change(t, "working")

	first := nextBuild(t, fixture)
	first.finish <- nil

	waitForRunnerStatus(t, fixture.server, "ready", 1)
	writeWatchFile(t, fixture.root, "dist/web/game.kart", "incomplete cartridge")
	fixture.change(t, "incomplete output")

	second := nextBuild(t, fixture)
	second.finish <- nil

	waitForRunnerStatus(t, fixture.server, "error", 1)
	fixtureBuild(t, filepath.Join(fixture.root, "dist", "web"), []byte("updated level"), []byte("code"), nil)
	fixture.change(t, "valid output")

	third := nextBuild(t, fixture)
	third.finish <- nil

	waitForRunnerStatus(t, fixture.server, "ready", 3)
}

func TestRunWatchesSourceAndStopsCleanly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWatchFile(t, root, "internal/game/logic.go", "initial")
	fixtureBuild(t, filepath.Join(root, "dist", "web"), []byte("level"), []byte("code"), nil)
	server := fixtureServer(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	finished := make(chan error, 1)
	go func() {
		finished <- Run(ctx, server, RunOptions{
			Directory: root, PollInterval: time.Millisecond, Debounce: time.Millisecond,
			Build: func(context.Context, func(string)) error { return nil },
		})
	}()

	waitForRunnerStatus(t, server, "ready", 1)
	writeWatchFile(t, root, "internal/game/logic.go", "edited internal game code")
	waitForRunnerStatus(t, server, "ready", 2)
	cancel()

	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run left its watcher alive after shutdown")
	}
}
