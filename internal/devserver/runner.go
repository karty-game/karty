package devserver

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

// RunOptions keeps the watcher independent of compilers and the build service.
// Build must return only after cancellation cleanup has completed.
type RunOptions struct {
	Directory    string
	Build        func(context.Context, func(string)) error
	PollInterval time.Duration
	Debounce     time.Duration
	Log          func(string)
	// WatchRoots supplies explicitly declared development package roots. Each
	// is watched independently without permitting project symlinks to escape.
	WatchRoots func(context.Context) (map[string]string, error)
}

const (
	defaultPollInterval = 500 * time.Millisecond
	defaultDebounce     = 150 * time.Millisecond
)

var errBuildRequired = errors.New("development build callback is required")

type buildResult struct {
	snapshot *Snapshot
	digest   string
	err      error
}

type buildProgress struct {
	revision uint64
	digest   string
	message  string
}

// Run serializes builds and snapshots while keeping the HTTP server responsive.
// Failed and obsolete builds never replace the last successful generation.
func Run(ctx context.Context, server *Server, options RunOptions) error {
	if ctx.Err() != nil {
		return nil
	}

	if options.Build == nil {
		return errBuildRequired
	}

	if options.PollInterval <= 0 {
		options.PollInterval = defaultPollInterval
	}

	if options.Debounce <= 0 {
		options.Debounce = defaultDebounce
	}

	watcher, err := NewWatcher(options.Directory)
	if err != nil {
		return err
	}

	watcher.additionalRoots = options.WatchRoots

	if ctx.Err() != nil {
		_ = watcher.Close()

		return nil
	}

	watchCtx, stop := context.WithCancel(ctx)
	changes := make(chan sourceChange, 1)

	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)

		watcher.changes(watchCtx, options.PollInterval, changes)
	}()

	defer func() { stop(); <-watchDone; _ = watcher.Close() }()

	return runBuilds(ctx, server, watcher, options, changes)
}

type buildLoop struct {
	server       *Server
	watcher      *Watcher
	options      RunOptions
	pending      *sourceChange
	latestDigest string
	active       context.CancelFunc
	settled      <-chan time.Time
	timer        *time.Timer
	revision     uint64
	results      chan buildResult
	updates      chan buildProgress
}

func runBuilds(ctx context.Context, server *Server, watcher *Watcher, options RunOptions, changes <-chan sourceChange) error {
	loop := &buildLoop{
		server: server, watcher: watcher, options: options,
		results: make(chan buildResult, 1), updates: make(chan buildProgress, 1),
	}
	defer loop.close()

	for {
		if ctx.Err() != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case change, open := <-changes:
			if !open {
				return nil
			}

			loop.changed(change)
		case <-loop.settled:
			loop.settled = nil
			loop.start(ctx)
		case result := <-loop.results:
			loop.completed(ctx, result)
		case progress := <-loop.updates:
			loop.progressed(progress)
		}
	}
}

func (loop *buildLoop) close() {
	if loop.timer != nil {
		loop.timer.Stop()
	}

	if loop.active != nil {
		loop.active()
		<-loop.results
	}
}

func (loop *buildLoop) queue(change sourceChange) {
	loop.pending = &change

	if loop.timer != nil {
		loop.timer.Stop()
	}

	loop.timer = time.NewTimer(loop.options.Debounce)
	loop.settled = loop.timer.C
}

func (loop *buildLoop) changed(change sourceChange) {
	key := change.digest
	if change.err != nil {
		key = "error:" + change.err.Error()
	}

	if key == loop.latestDigest {
		return
	}

	loop.latestDigest = key
	if loop.active != nil {
		loop.active()
	}

	if change.err != nil {
		loop.pending = nil
		loop.server.SetStatus("error", change.err.Error())

		return
	}

	loop.server.SetStatus("building", "Waiting for source changes to settle…")
	loop.queue(change)
}

func (loop *buildLoop) start(ctx context.Context) {
	if ctx.Err() != nil || loop.active != nil || loop.pending == nil {
		return
	}

	loop.revision++
	job := *loop.pending
	loop.pending = nil
	workCtx, cancel := context.WithCancel(ctx)
	loop.active = cancel
	loop.server.SetStatus("building", "Building game and updating assets…")

	go func() {
		defer cancel()

		buildGeneration(workCtx, loop.options, job.digest, loop.revision, loop.results, loop.updates)
	}()
}

func (loop *buildLoop) progressed(progress buildProgress) {
	// Progress shares the publication owner. A callback that raced cancellation
	// can never replace the watcher diagnostic or a later build's status.
	if loop.active != nil && loop.revision == progress.revision && loop.latestDigest == progress.digest {
		loop.server.SetStatus("building", progress.message)
	}
}

func (loop *buildLoop) completed(ctx context.Context, result buildResult) {
	if loop.active != nil {
		loop.active()
	}

	loop.active = nil

	if loop.pending != nil {
		// The debounce may already have elapsed while cancellation cleanup ran.
		if loop.settled == nil {
			loop.queue(*loop.pending)
		}

		return
	}

	if errors.Is(result.err, context.Canceled) || ctx.Err() != nil {
		return
	}

	current, scanErr := loop.watcher.Scan(ctx)
	if scanErr != nil {
		loop.latestDigest = "error:" + scanErr.Error()
		loop.server.SetStatus("error", scanErr.Error())

		return
	}

	if current != result.digest {
		loop.latestDigest = current
		loop.server.SetStatus("building", "Source changed while building; rebuilding…")
		loop.queue(sourceChange{digest: current})

		return
	}

	if ctx.Err() != nil {
		return
	}

	if result.err != nil {
		loop.server.SetStatus("error", result.err.Error())

		if loop.options.Log != nil {
			loop.options.Log(result.err.Error())
		}

		return
	}

	loop.server.Publish(result.snapshot)

	if loop.options.Log != nil {
		loop.options.Log(fmt.Sprintf("Preview updated (build %d)", result.snapshot.Generation.Revision))
	}
}

func buildGeneration(ctx context.Context, options RunOptions, digest string, revision uint64,
	output chan<- buildResult, updates chan<- buildProgress) {
	err := ctx.Err()
	if err == nil {
		err = options.Build(ctx, func(message string) {
			if ctx.Err() == nil {
				select {
				case updates <- buildProgress{revision: revision, digest: digest, message: message}:
				default:
				}
			}
		})
	}

	var snapshot *Snapshot

	if err == nil {
		err = ctx.Err()
	}

	if err == nil {
		snapshot, err = Capture(ctx, filepath.Join(options.Directory, "dist", "web"), revision)
	}

	if ctx.Err() != nil {
		err = ctx.Err()
	}

	output <- buildResult{snapshot: snapshot, digest: digest, err: err}
}
