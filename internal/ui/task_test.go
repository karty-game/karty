package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

//nolint:paralleltest // Bubble Tea programs share terminal output and install process-wide signal handlers.
func TestTaskCancellationWaitsForWorkerCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	started := make(chan struct{})
	releaseCleanup := make(chan struct{})
	workerDone := make(chan struct{})
	returned := make(chan error, 1)

	defer close(releaseCleanup)

	go func() {
		returned <- RunTaskWithProgress(ctx, "Building", func(ctx context.Context, progress func(string)) error {
			defer close(workerDone)

			close(started)
			<-ctx.Done()
			progress("Cleaning up")
			<-releaseCleanup

			return ctx.Err()
		})
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("task worker did not start")
	}

	cancel()

	select {
	case err := <-returned:
		t.Fatalf("task returned before cleanup completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	releaseCleanup <- struct{}{}

	select {
	case err := <-returned:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("task did not return after cleanup completed")
	}

	select {
	case <-workerDone:
	default:
		t.Fatal("task returned while worker was still running")
	}
}

//nolint:paralleltest // Bubble Tea programs share terminal output and install process-wide signal handlers.
func TestTaskReturnsWorkerError(t *testing.T) {
	want := staticError("build failed")

	err := RunTaskWithProgress(t.Context(), "Building", func(_ context.Context, progress func(string)) error {
		progress("Compiling")

		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("task error = %v, want %v", err, want)
	}
}

func TestTaskModelViewIncludesProgressBar(t *testing.T) {
	t.Parallel()

	view := (taskModel{label: "Building client"}).View()
	if !strings.Contains(view, "Building client [") {
		t.Fatalf("task view = %q, want label and progress bar", view)
	}
}

func TestTaskModelReturnsTaskError(t *testing.T) {
	t.Parallel()

	want := staticError("task failed")

	model, command := (taskModel{}).Update(taskFinishedMsg{err: want})
	if command == nil {
		t.Fatal("task completion did not quit the program")
	}

	result, ok := model.(taskModel)
	if !ok {
		t.Fatal("task completion returned an unexpected model")
	}

	if !errors.Is(result.err, want) {
		t.Fatalf("task error = %v, want %v", result.err, want)
	}
}

func TestTaskProgressRetainsAnimationAndFailureActivity(t *testing.T) {
	t.Parallel()

	model, command := (taskModel{label: "Building game", frame: 3}).Update(taskProgressMsg("Compiling Go game"))

	result, valid := model.(taskModel)
	if !valid || command != nil || result.frame != 3 || !strings.Contains(result.View(), "Compiling Go game [") {
		t.Fatalf("progress lost the activity or animation: %+v", model)
	}

	model, command = result.Update(taskFinishedMsg{err: staticError("compiler failed")})

	result, valid = model.(taskModel)
	if !valid || command == nil || result.View() != "Compiling Go game: failed" {
		t.Fatalf("failure lost its build activity: %+v", model)
	}
}
