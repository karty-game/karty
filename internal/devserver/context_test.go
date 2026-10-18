package devserver

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/karty-game/karty-sdk/format/cartridge"
)

type cancelingContents struct {
	cancel  context.CancelFunc
	reads   int
	maxRead int
}

func (reader *cancelingContents) Read(buffer []byte) (int, error) {
	reader.reads++

	reader.maxRead = max(reader.maxRead, len(buffer))
	for index := range buffer {
		buffer[index] = 'x'
	}

	if reader.reads == 2 {
		reader.cancel()
	}

	return len(buffer), nil
}

func TestSnapshotAndWatcherCancelBetweenBoundedReads(t *testing.T) {
	t.Parallel()

	for _, watch := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		reader := &cancelingContents{cancel: cancel}

		var err error
		if watch {
			_, _, err = hashWatchContents(ctx, reader)
		} else {
			_, err = readSnapshotContents(ctx, reader, maxArtifactBytes)
		}

		cancel()

		if !errors.Is(err, context.Canceled) || reader.reads != 2 || reader.maxRead > contextChunkBytes {
			t.Fatalf("cancel after read: watch=%v reads=%d maxRead=%d err=%v", watch, reader.reads, reader.maxRead, err)
		}
	}
}

// This context cancels deterministically during CPU-only loops, without relying
// on scheduling a timer against a fast warmed hash or memory copy.
type checkCancellation struct {
	checks int
	after  int
	done   chan struct{}
}

func (ctx *checkCancellation) Deadline() (time.Time, bool) { return time.Time{}, false }
func (ctx *checkCancellation) Done() <-chan struct{}       { return ctx.done }
func (ctx *checkCancellation) Value(any) any               { return nil }
func (ctx *checkCancellation) Err() error {
	ctx.checks++
	if ctx.checks >= ctx.after {
		select {
		case <-ctx.done:
		default:
			close(ctx.done)
		}

		return context.Canceled
	}

	return nil
}

func canceledDuringChecks(after int) *checkCancellation {
	return &checkCancellation{after: after, done: make(chan struct{})}
}

func TestSnapshotHashAndCloneCancelDuringCPUWork(t *testing.T) {
	t.Parallel()

	data := bytes.Repeat([]byte("x"), contextChunkBytes*4)

	checksum, err := digestContext(canceledDuringChecks(3), data)
	if !errors.Is(err, context.Canceled) || checksum != "" {
		t.Fatal("hash returned a result after cancellation")
	}

	cloned, err := cloneContext(canceledDuringChecks(3), data)
	if !errors.Is(err, context.Canceled) || cloned != nil {
		t.Fatal("clone returned a snapshot after cancellation")
	}
}

func TestManifestSectionWalkCancelsAndKeepsCompleteValidation(t *testing.T) {
	t.Parallel()

	header := []byte("\x00asm\x01\x00\x00\x00")
	wasm := append(bytes.Clone(header), bytes.Repeat([]byte{0, 2, 1, 'x'}, 64)...)

	wasm, err := cartridge.EmbedSection(wasm, cartridge.ManifestSectionName, []byte("manifest"))
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := manifestSection(canceledDuringChecks(5), wasm); !errors.Is(err, context.Canceled) {
		t.Fatal("section walk ignored cancellation")
	}

	valid, err := cartridge.EmbedSection(header, cartridge.ManifestSectionName, []byte("manifest"))
	if err != nil {
		t.Fatal(err)
	}

	for _, invalid := range [][]byte{append(bytes.Clone(valid), valid[len(header):]...), append(bytes.Clone(valid), 0, 128)} {
		if _, _, err := manifestSection(t.Context(), invalid); !errors.Is(err, cartridge.ErrSection) {
			t.Fatal("incomplete or duplicated section accepted")
		}
	}
}

func TestCanceledIdentificationDoesNotPublishGeneration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fixtureBuild(t, root, []byte("level"), []byte("code"), nil)
	snapshot := captureFixture(t, root, 1)

	snapshot.Generation = Generation{}
	if err := snapshot.identify(canceledDuringChecks(5), 2); !errors.Is(err, context.Canceled) {
		t.Fatalf("identification cancellation: %v", err)
	}

	if snapshot.Generation.ID != "" {
		t.Fatal("canceled identification published a generation")
	}
}
