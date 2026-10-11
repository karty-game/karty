package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
)

func TestBrowserChunkBridgeExecutesPassiveWASM(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node unavailable; required by mise run check-web")
	}

	metadata, err := level.EncodeMetadata(level.Metadata{"chunkID": level.UintValue(0)})
	if err != nil {
		t.Fatal(err)
	}

	envelope, err := level.Encode(metadata, []level.SourceEntry{{Name: "mesh", Kind: level.EntryData, Data: []byte("chunk fixture")}})
	if err != nil {
		t.Fatal(err)
	}

	module, err := level.WrapModule(envelope)
	if err != nil {
		t.Fatal(err)
	}

	directory := t.TempDir()

	path := filepath.Join(directory, "chunk.kld")
	if err := os.WriteFile(path, module, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(directory, "envelope.bin"), envelope, 0o600); err != nil {
		t.Fatal(err)
	}

	otherMetadata, err := level.EncodeMetadata(level.Metadata{"chunkID": level.UintValue(1)})
	if err != nil {
		t.Fatal(err)
	}

	otherEnvelope, err := level.Encode(
		otherMetadata,
		[]level.SourceEntry{{Name: "mesh", Kind: level.EntryData, Data: []byte("chunk fixture")}},
	)
	if err != nil {
		t.Fatal(err)
	}

	otherModule, err := level.WrapModule(otherEnvelope)
	if err != nil {
		t.Fatal(err)
	}

	if len(otherModule) != len(module) {
		t.Fatal("substitution fixture must preserve exact artifact size")
	}

	if err := os.WriteFile(filepath.Join(directory, "chunk-other.kld"), otherModule, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.CommandContext(t.Context(), "node", "testdata/launcher-chunks.test.mjs", "templates/launcher.js.tmpl", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("browser chunk bridge: %v\n%s", err, output)
	}
}
