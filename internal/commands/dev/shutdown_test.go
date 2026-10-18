package dev

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/urfave/cli/v3"
)

type previewAddressWriter struct {
	once    sync.Once
	address chan string
}

func (writer *previewAddressWriter) Write(contents []byte) (int, error) {
	if address, found := strings.CutPrefix(string(contents), "Karty dev watching project files at "); found {
		writer.once.Do(func() { writer.address <- strings.TrimSpace(address) })
	}

	return len(contents), nil
}

func awaitShutdownSignal(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()

	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func TestCancellationClosesHTTPAndSSEBeforeJoiningBuildCleanup(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	release := make(chan struct{})

	var releaseOnce sync.Once

	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })

	started, canceled := make(chan struct{}), make(chan struct{})
	writer := &previewAddressWriter{address: make(chan string, 1)}
	command := &cli.Command{Writer: writer, ErrWriter: io.Discard}
	config := configuration{directory: t.TempDir(), address: "127.0.0.1:0"}

	finished := make(chan error, 1)
	go func() {
		finished <- serve(ctx, command, config, func(buildCtx context.Context, _ func(string)) error {
			close(started)
			<-buildCtx.Done()
			close(canceled)
			<-release

			return buildCtx.Err()
		})
	}()

	var address string
	select {
	case address = <-writer.address:
	case err := <-finished:
		t.Fatalf("preview exited before listening: %v", err)
	case <-time.After(time.Second):
		t.Fatal("preview did not start listening")
	}

	awaitShutdownSignal(t, started, "build start")

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address+"__karty_dev/events", nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	reader := bufio.NewReader(response.Body)
	if first, readErr := reader.ReadString('\n'); readErr != nil || first != "event: snapshot\n" {
		t.Fatalf("stream did not start with a snapshot: %q, %v", first, readErr)
	}

	streamClosed := make(chan struct{})

	go func() { _, _ = io.Copy(io.Discard, reader); close(streamClosed) }()

	cancel()
	awaitShutdownSignal(t, canceled, "build cancellation")
	awaitShutdownSignal(t, streamClosed, "event stream shutdown")
	assertPreviewListenerClosed(t, strings.TrimSuffix(strings.TrimPrefix(address, "http://"), "/"))

	select {
	case err := <-finished:
		t.Fatalf("preview returned before joining build cleanup: %v", err)
	default:
	}

	releaseOnce.Do(func() { close(release) })

	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("preview did not join shutdown goroutines")
	}
}

func assertPreviewListenerClosed(t *testing.T, address string) {
	t.Helper()

	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()

	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()

	for {
		dialer := net.Dialer{Timeout: 100 * time.Millisecond}

		connection, err := dialer.DialContext(t.Context(), "tcp", address)
		if err != nil {
			if t.Context().Err() != nil {
				t.Fatal("test context expired before the listener closed")
			}

			return
		}

		_ = connection.Close()

		select {
		case <-deadline.C:
			t.Fatal("preview listener survived cancellation")
		case <-tick.C:
		}
	}
}

func TestCanceledDevelopmentDoesNotListenOrStartBuild(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	writer := &previewAddressWriter{address: make(chan string, 1)}
	command := &cli.Command{Writer: writer, ErrWriter: io.Discard}

	config := configuration{directory: t.TempDir(), address: "127.0.0.1:0"}
	if err := serve(ctx, command, config, func(context.Context, func(string)) error {
		t.Error("canceled preview started a build")

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case <-writer.address:
		t.Fatal("canceled preview opened a listener")
	default:
	}
}
