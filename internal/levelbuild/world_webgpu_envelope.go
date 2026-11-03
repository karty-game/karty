package levelbuild

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/karty-game/karty-sdk/format/level"
)

// Account for the exact completed root, after source omission and sprite atlas,
// character, world, UI, actions and chunk catalog preparation. The public
// envelope has no padding: one header, metadata, entry headers/names and bytes.
// Encode still performs canonical-name/kind/duplicate validation afterward.
func completeWebGPUEnvelopeSize(metadata []byte, entries []level.SourceEntry) (uint64, error) {
	if len(metadata) > level.MaxMetadataSize || len(entries) > level.MaxEntryCount {
		return 0, level.ErrEnvelopeSize
	}

	total := uint64(level.HeaderSize) + uint64(len(metadata))
	for _, entry := range entries {
		if len(entry.Name) == 0 || len(entry.Name) > level.MaxEntryNameSize || !utf8.ValidString(entry.Name) || len(entry.Data) == 0 ||
			len(entry.Data) > level.MaxEntrySize {
			return 0, fmt.Errorf("entry %q (%d bytes): %w", entry.Name, len(entry.Data), level.ErrEntry)
		}

		total += uint64(level.EntryHeaderSize) + uint64(len(entry.Name)) + uint64(len(entry.Data))
	}

	if total > level.MaxEnvelopeSize {
		largest := slices.Clone(entries)
		slices.SortFunc(largest, func(a, b level.SourceEntry) int { return len(b.Data) - len(a.Data) })

		parts := make([]string, 0, min(8, len(largest)))
		for _, entry := range largest[:min(8, len(largest))] {
			parts = append(parts, fmt.Sprintf("%s=%d", entry.Name, len(entry.Data)))
		}

		return total, fmt.Errorf(
			"complete root requires %d bytes, limit %d (largest entries: %s): %w",
			total,
			level.MaxEnvelopeSize,
			strings.Join(parts, ", "),
			level.ErrEnvelopeSize,
		)
	}

	return total, nil
}
