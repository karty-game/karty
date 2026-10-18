package contextio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type cancelReader struct {
	input  io.Reader
	cancel context.CancelFunc
	reads  int
}

func (reader *cancelReader) Read(contents []byte) (int, error) {
	reader.reads++
	n, err := reader.input.Read(contents)
	reader.cancel()

	return n, err
}

// WriterTo must not bypass chunked context checks.
func (*cancelReader) WriteTo(io.Writer) (int64, error) { panic("uncancellable fast path") }

func TestCanceledCopyStopsBetweenChunksAndDoesNotUseFastPath(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	input := &cancelReader{input: bytes.NewReader(make([]byte, 4*chunkSize)), cancel: cancel}

	var output bytes.Buffer

	n, err := Copy(ctx, &output, input)
	if !errors.Is(err, context.Canceled) || n > chunkSize || input.reads != 1 || output.Len() > chunkSize {
		t.Fatalf("copy continued after cancellation: n=%d reads=%d output=%d err=%v", n, input.reads, output.Len(), err)
	}
}

func TestExactLengthCopyDoesNotSwallowCancellationWithFinalBytes(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	input := &cancelReader{input: bytes.NewReader([]byte("payload")), cancel: cancel}

	_, err := CopyN(ctx, io.Discard, input, 7)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("final-byte cancellation was lost: %v", err)
	}
}

func TestCanceledReadDoesNotTouchSource(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	input := &cancelReader{input: bytes.NewReader([]byte("payload")), cancel: cancel}

	if _, err := ReadAll(ctx, input); !errors.Is(err, context.Canceled) || input.reads != 0 {
		t.Fatalf("canceled reader touched source: reads=%d err=%v", input.reads, err)
	}
}
