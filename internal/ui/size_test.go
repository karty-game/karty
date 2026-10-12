package ui

import "testing"

func TestSizeUsesBinaryUnits(t *testing.T) {
	t.Parallel()

	for _, example := range []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"}, {1, "1 B"}, {1023, "1023 B"}, {1024, "1.0 KiB"},
		{1536, "1.5 KiB"}, {1 << 20, "1.0 MiB"}, {1 << 30, "1.0 GiB"}, {1 << 40, "1.0 TiB"},
	} {
		if got := Size(example.bytes); got != example.want {
			t.Errorf("Size(%d) = %q, want %q", example.bytes, got, example.want)
		}
	}
}
