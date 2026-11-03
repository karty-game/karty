package levelbuild

import (
	"errors"
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
)

func TestWebGPUCompleteEnvelopeBoundaryMatchesPublicEncoder(t *testing.T) {
	t.Parallel()

	metadata, err := level.EncodeMetadata(level.Metadata{"name": level.StringValue("aggregate")})
	if err != nil {
		t.Fatal(err)
	}

	entries := []level.SourceEntry{
		{Name: "@world/material-data", Kind: level.EntryData, Data: make([]byte, level.MaxEntrySize)},
		{Name: "@character/models", Kind: level.EntryData},
	}

	overhead := level.HeaderSize + len(metadata)
	for _, e := range entries {
		overhead += level.EntryHeaderSize + len(e.Name)
	}

	entries[1].Data = make([]byte, level.MaxEnvelopeSize-overhead-len(entries[0].Data))

	size, err := completeWebGPUEnvelopeSize(metadata, entries)
	if err != nil || size != level.MaxEnvelopeSize {
		t.Fatal("exact complete boundary rejected", size, err)
	}

	encoded, err := level.Encode(metadata, entries)
	if err != nil || uint64(len(encoded)) != size {
		t.Fatal("preflight diverges from public encoder", err)
	}

	entries[1].Data = append(entries[1].Data, 0)
	if _, err := completeWebGPUEnvelopeSize(metadata, entries); !errors.Is(err, level.ErrEnvelopeSize) {
		t.Fatal("aggregate overflow accepted", err)
	}

	if _, err := level.Encode(metadata, entries); !errors.Is(err, level.ErrEnvelopeSize) {
		t.Fatal("public boundary assumption invalid", err)
	}
}
