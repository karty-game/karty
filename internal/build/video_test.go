package build

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty/internal/project"
)

//nolint:wsl_v5 // One scenario covers catalog generation and staging.
func TestVideoStagingAndTypedIDs(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	source := append([]byte{0, 0, 1, 0xba}, bytes.Repeat([]byte{7}, cartridge.VideoChunkSize)...)
	if err := os.WriteFile(filepath.Join(directory, "clip.mpg"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(directory, "game.kart")
	if err := os.WriteFile(artifact, []byte{0, 97, 115, 109, 1, 0, 0, 0}, 0o600); err != nil {
		t.Fatal(err)
	}
	videos := []project.Video{{Name: "z.demo", Source: "clip.mpg"}, {Name: "a.demo", Source: "clip.mpg"}}
	if err := stageProjectVideos(directory, directory, artifact, videos); err != nil {
		t.Fatal(err)
	}
	wasm, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := cartridge.ExtractSection(wasm, cartridge.VideoSectionName)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := cartridge.DecodeVideos(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if catalog[0].Name != "a.demo" || catalog[0].ID != 1 || len(catalog[0].Chunks) != 2 {
		t.Fatalf("catalog: %+v", catalog)
	}
	target := t.TempDir()
	if err := stageVideoFiles(target, artifact, directory); err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // The validated catalog path is the subject of this test.
	got, err := os.ReadFile(filepath.Join(target, catalog[0].Path()))
	if err != nil || bytes.Equal(got, source) || filepath.Ext(catalog[0].Path()) != ".kvid" {
		t.Fatalf("staged bytes: %v", err)
	}
	reader, err := cartridge.NewMediaReader(bytes.NewReader(got), cartridge.MediaKindMPEG1Video, int64(len(got)))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(payload, source) || reader.Verify() != nil {
		t.Fatalf("unwrapped video: %v", err)
	}
	generated, err := videoAssetFile(videos, "example.com/game/.karty/engine")
	if err != nil || !bytes.Contains(generated, []byte("VideoADemo engine.VideoID = 1")) {
		t.Fatalf("generated IDs: %s %v", generated, err)
	}
}

//nolint:wsl_v5 // The table keeps both rejection paths together.
func TestVideoSourceRejectsEscapeAndUnsupportedHeader(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "bad.mpg"), []byte("not an MPEG file"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, path := range []string{"bad.mpg", "../escape.mpg"} {
		if _, err := stageVideoSource(root, t.TempDir(), path); err == nil {
			t.Fatal("accepted invalid source")
		}
	}
}

//nolint:wsl_v5 // Both identifier assertions form one small scenario.
func TestVideoIdentifiersPreserveSoundInName(t *testing.T) {
	t.Parallel()

	generated, err := videoAssetFile([]project.Video{{Name: "soundtrack.intro"}}, "example.com/game/.karty/engine")
	if err != nil || !bytes.Contains(generated, []byte("VideoSoundtrackIntro engine.VideoID = 1")) {
		t.Fatalf("generated: %s %v", generated, err)
	}
	if _, err := videoAssetFile([]project.Video{{Name: "demo.clip"}, {Name: "demo-clip"}}, "example.com/game/.karty/engine"); err == nil {
		t.Fatal("accepted colliding video identifiers")
	}
}
