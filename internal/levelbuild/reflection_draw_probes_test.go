package levelbuild

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"testing"

	"github.com/karty-game/karty-sdk/bake/reflectionplan"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldreflection"
)

func TestReflectionDrawProbesResolveSparseSectorSolidAndEmptyOwnersOffline(t *testing.T) {
	t.Parallel()

	var payload worldgpuchunk.Payload

	payload.RecordRefs = []worldgpuchunk.RecordRef{
		{OwnerKind: worldgpuchunk.OwnerSector, OwnerID: 0},
		{OwnerKind: worldgpuchunk.OwnerSolid, OwnerID: 0},
		{OwnerKind: worldgpuchunk.OwnerSector, OwnerID: 1},
	}
	payload.Draws = []worldgpuchunk.Draw{{Record: 0}}
	payload.Records = make([]byte, len(payload.RecordRefs)*worldgpuchunk.RecordStride)
	binary.LittleEndian.PutUint32(payload.Records[gpuCellRecordProbeOffset:], 2)
	binary.LittleEndian.PutUint32(payload.Records[worldgpuchunk.RecordStride+gpuCellRecordProbeOffset:], 1)

	capture := worldreflection.Bundle{Sectors: []int{97, -1}, Solids: []int{7}}

	slots, err := packReflectionDrawProbes(payload, capture, map[int]uint32{7: 1, 97: 2})
	if err != nil || !bytes.Equal(slots, make([]byte, 4)) {
		t.Fatal("offline sparse/sector/solid/no-environment mapping changed", slots, err)
	}

	binary.LittleEndian.PutUint32(payload.Records[worldgpuchunk.RecordStride+gpuCellRecordProbeOffset:], 2)

	if _, err := packReflectionDrawProbes(payload, capture, map[int]uint32{7: 1, 97: 2}); err == nil {
		t.Fatal("wrong probe in a nonrepresentative record accepted")
	}
}

func TestReflectionDrawProbesMatchCatalogHashAndLeaveCellsUntouched(t *testing.T) {
	encoder := os.Getenv("KARTY_TEST_KTX")
	if encoder == "" {
		t.Skip("KARTY_TEST_KTX not selected")
	}

	t.Setenv("KARTY_KTX", encoder)
	root, assets := gpuActualProfileFixture(t, "fixed")
	assets.manifest.Assets.Capabilities.Runtime = append(assets.manifest.Assets.Capabilities.Runtime,
		asset.Capability(worldreflection.GPUFeature))

	artifacts, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(err)
	}

	artifact := artifacts[0]

	envelope, document, err := reflectionSource(artifact.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	capture, err := reflectionplan.Plan(document, worldreflection.InputDigest(envelope))
	if err != nil {
		t.Fatal(err)
	}

	catalogData, _, _ := envelope.Read(worldgpuchunk.CatalogEntry, 0, level.MaxEntrySize)

	catalog, err := worldgpuchunk.DecodeCatalog(catalogData)
	if err != nil {
		t.Fatal(err)
	}

	chunks := append([]ChunkArtifact{{ID: 0, Bytes: []byte("unrelated physics"), ContentSHA256: "foreign"}}, artifact.Chunks...)
	before := make([][]byte, len(chunks))

	for index := range chunks {
		before[index] = bytes.Clone(chunks[index].Bytes)
		chunks[index].ID = 0
	}

	tables, err := buildReflectionDrawProbes(t.Context(), envelope, chunks, capture)
	if err != nil || len(tables) != len(catalog.Cells) {
		t.Fatal("catalog hash selection failed", err)
	}

	for index, cell := range catalog.Cells {
		var payload worldgpuchunk.Payload

		for _, chunk := range chunks {
			if chunk.ContentSHA256 == hex.EncodeToString(cell.ContentSHA256[:]) {
				child := decodePassiveEnvelope(t, chunk.Bytes)
				data, _, _ := child.Read(worldgpuchunk.PayloadEntry, 0, level.MaxEntrySize)

				payload, err = worldgpuchunk.DecodePayload(data)
				if err != nil {
					t.Fatal(err)
				}
			}
		}

		if len(tables[index]) != len(payload.Draws)*4 {
			t.Fatal("draw table count changed")
		}

		for offset := 0; offset < len(tables[index]); offset += 4 {
			if binary.LittleEndian.Uint32(tables[index][offset:]) != 0 {
				t.Fatal("ordinary publication suppressed per-record probes")
			}
		}
	}

	for index := range chunks {
		if !bytes.Equal(before[index], chunks[index].Bytes) {
			t.Fatal("cell bytes rewritten")
		}
	}
}
