//go:build unix

package process

import (
	"context"
	"sync"
	"testing"
	"time"
)

type compilerReadyWriter struct {
	once  sync.Once
	ready chan struct{}
}

func (writer *compilerReadyWriter) Write(contents []byte) (int, error) {
	writer.once.Do(func() { close(writer.ready) })

	return len(contents), nil
}

func TestCancellationClosesDescendantPipes(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// The child inherits the output pipe. Killing only the shell would leave
	// Wait blocked until its sleep finishes or the longer WaitDelay expires.
	command := CommandContext(ctx, "sh", "-c", "sleep 60 & printf ready; wait")

	writer := &compilerReadyWriter{ready: make(chan struct{})}
	command.Stdout = writer
	command.Stderr = writer
	finished := make(chan error, 1)

	go func() { finished <- command.Run() }()

	select {
	case <-writer.ready:
	case err := <-finished:
		t.Fatalf("compiler failed before becoming ready: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("compiler did not become ready")
	}

	cancel()

	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("canceled compiler succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("compiler descendant retained its output pipe after cancellation")
	}
}
