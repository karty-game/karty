package build

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoa"
	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/project"
)

//nolint:wsl_v5 // One scenario covers generation, catalog embedding, and target staging.
func TestAudioStreamStagingAndTypedIDs(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	encoded, metadata, err := qoa.Encode([]int16{1, -1, 2, -2}, 1, 48_000)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(directory, "cached.qoa")
	if err := os.WriteFile(source, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(directory, "game.kart")
	if err := os.WriteFile(artifact, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(encoded))
	streams := []assetpipeline.AudioStream{
		{
			ID:           1,
			Name:         "theme",
			Kind:         "music",
			SourcePath:   source,
			OutputSHA256: digest,
			OutputBytes:  int64(len(encoded)),
			Metadata: assetpipeline.AudioMetadata{
				Channels:   metadata.Channels,
				SampleRate: metadata.SampleRate,
				Frames:     metadata.Frames,
			},
		},
	}
	if err := stageProjectAudioStreams(directory, artifact, streams); err != nil {
		t.Fatal(err)
	}
	wasm, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := cartridge.ExtractSection(wasm, cartridge.AudioStreamSectionName)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := cartridge.DecodeAudioStreams(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 1 || catalog[0].Name != "theme" || len(catalog[0].Chunks) != 1 {
		t.Fatalf("catalog = %+v", catalog)
	}
	target := t.TempDir()
	if err := stageAudioStreamFiles(target, artifact, directory); err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // The validated catalog path is the subject of the staging test.
	got, err := os.ReadFile(filepath.Join(target, catalog[0].Path()))
	if err != nil || bytes.Equal(got, encoded) || filepath.Ext(catalog[0].Path()) != ".kaud" {
		t.Fatalf("staged bytes: %v", err)
	}
	reader, err := cartridge.NewMediaReader(bytes.NewReader(got), cartridge.MediaKindQOAAudio, int64(len(got)))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(payload, encoded) || reader.Verify() != nil {
		t.Fatalf("unwrapped audio: %v", err)
	}
	generated, err := audioStreamAssetFile(
		"MusicID",
		"Music",
		"music",
		"example.com/game/.karty/engine",
		[]project.AudioStream{{Name: "battle.theme"}},
	)
	if err != nil || !bytes.Contains(generated, []byte("MusicBattleTheme engine.MusicID = 1")) {
		t.Fatalf("generated IDs: %s %v", generated, err)
	}
}

func TestAudioStreamAssetFileEmptyHasNoUnusedImport(t *testing.T) {
	t.Parallel()

	generated, err := audioStreamAssetFile("MusicID", "Music", "music", "example.com/game/.karty/engine", nil)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(generated, []byte("import engine")) {
		t.Fatalf("empty generated file has unused import: %q", generated)
	}
}
