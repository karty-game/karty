package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldreflection"
)

const gpuCellRecordProbeOffset = 77 * 4

// Bind final slots after capture, leaving every content-addressed cell untouched.
func buildReflectionDrawProbes(ctx context.Context, root level.Envelope, chunks []ChunkArtifact,
	capture worldreflection.Bundle,
) ([][]byte, error) {
	encoded, _, found := root.Read(worldgpuchunk.CatalogEntry, 0, level.MaxEntrySize)
	if !found {
		return nil, worldreflection.ErrCubemaps
	}

	catalog, err := worldgpuchunk.DecodeCatalog(encoded)
	if err != nil {
		return nil, err
	}

	artifacts := make(map[string][]byte, len(chunks))

	for _, chunk := range chunks {
		artifacts[chunk.ContentSHA256] = chunk.Bytes
	}

	probes := make(map[int]uint32, len(capture.Probes))

	for index, probe := range capture.Probes {
		// #nosec G115 -- The validated capture has at most 256 probes.
		probes[probe.ID] = uint32(index + 1)
	}

	tables := make([][]byte, len(catalog.Cells))

	for index, cell := range catalog.Cells {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		module := artifacts[hex.EncodeToString(cell.ContentSHA256[:])]
		if uint64(len(module)) != cell.Size || sha256.Sum256(module) != cell.ContentSHA256 {
			return nil, worldreflection.ErrCubemaps
		}

		envelope, err := level.DecodePassiveModule(module)
		if err != nil {
			return nil, err
		}

		data, _, found := envelope.Read(worldgpuchunk.PayloadEntry, 0, level.MaxEntrySize)
		if !found {
			return nil, worldreflection.ErrCubemaps
		}

		payload, err := worldgpuchunk.DecodePayload(data)
		if err != nil || payload.CellID != cell.ID {
			return nil, worldreflection.ErrCubemaps
		}

		tables[index], err = packReflectionDrawProbes(payload, capture, probes)
		if err != nil {
			return nil, err
		}
	}

	return tables, nil
}

func packReflectionDrawProbes(payload worldgpuchunk.Payload, capture worldreflection.Bundle, probes map[int]uint32) ([]byte, error) {
	if len(payload.Records) != len(payload.RecordRefs)*worldgpuchunk.RecordStride {
		return nil, worldreflection.ErrCubemaps
	}

	for index, ref := range payload.RecordRefs {
		if ref.OwnerKind != worldgpuchunk.OwnerSector && ref.OwnerKind != worldgpuchunk.OwnerSolid {
			return nil, worldreflection.ErrCubemaps
		}

		owners := capture.Sectors
		if ref.OwnerKind == worldgpuchunk.OwnerSolid {
			owners = capture.Solids
		}

		if uint64(ref.OwnerID) >= uint64(len(owners)) {
			return nil, worldreflection.ErrCubemaps
		}

		id := owners[ref.OwnerID]

		slot, found := probes[id]
		if id != -1 && !found {
			return nil, worldreflection.ErrCubemaps
		}

		stored := binary.LittleEndian.Uint32(payload.Records[index*worldgpuchunk.RecordStride+gpuCellRecordProbeOffset:])
		if stored != slot {
			return nil, worldreflection.ErrCubemaps
		}
	}

	// The existing context word now suppresses reflections only for capture.
	// Ordinary contexts use every immutable record's own proven probe slot.
	return make([]byte, 4*len(payload.Draws)), nil
}
