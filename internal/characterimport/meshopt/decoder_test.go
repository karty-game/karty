package meshopt

import (
	"bytes"
	"context"
	"testing"
)

func TestDecodeSyntheticVertexStreamAndReuseAfterFailure(t *testing.T) {
	t.Parallel()

	decoder, err := New(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close(context.Background())
	// Meshopt v0 vertex stream: zero deltas for three vertices, four byte
	// channels, and the repeated vertex in the final four bytes. This fixture
	// is synthetic and does not depend on any project artwork.
	source := make([]byte, 37)
	source[0] = 0xa0
	copy(source[33:], []byte{1, 2, 3, 4})

	for range 3 {
		if _, err = decoder.Decode(t.Context(), []byte{0}, 3, 4, "ATTRIBUTES", ""); err == nil {
			t.Fatal("accepted truncated stream")
		}

		decoded, decodeErr := decoder.Decode(t.Context(), source, 3, 4, "ATTRIBUTES", "NONE")
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		if !bytes.Equal(decoded, bytes.Repeat([]byte{1, 2, 3, 4}, 3)) {
			t.Fatalf("incorrect decoded bytes: %v", decoded)
		}

		decoded[0] = 99
	}
}

func TestRejectsDecoderBoundsAndIncompatibleFilters(t *testing.T) {
	t.Parallel()

	decoder, err := New(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close(context.Background())

	for _, input := range []struct {
		count, stride int
		mode, filter  string
	}{{0, 4, "ATTRIBUTES", ""}, {MaxBytes, 256, "ATTRIBUTES", ""}, {3, 3, "ATTRIBUTES", ""}, {2, 2, "TRIANGLES", ""}, {3, 4, "INDICES", "OCTAHEDRAL"}, {3, 4, "ATTRIBUTES", "QUATERNION"}, {3, 4, "ATTRIBUTES", "unknown"}} {
		if _, err = decoder.Decode(t.Context(), []byte{0}, input.count, input.stride, input.mode, input.filter); err == nil {
			t.Fatalf("accepted invalid decoder input: %+v", input)
		}
	}
}
