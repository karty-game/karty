package levelbuild

import (
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
)

func binaryMetadata(tb testing.TB, metadata level.Metadata) []byte {
	tb.Helper()

	data, err := level.EncodeMetadata(metadata)
	if err != nil {
		tb.Fatal(err)
	}

	return data
}
