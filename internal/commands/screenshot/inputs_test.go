package screenshot

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/levelbuild"
)

func TestCaptureInputsRejectsBadChildBeforeCreatingContent(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	data := []byte("root")

	artifact := levelbuild.Artifact{Bytes: data, ContentSHA256: fmt.Sprintf("%x", sha256.Sum256(data)),
		Chunks: []levelbuild.ChunkArtifact{{ContentSHA256: "../outside", Bytes: []byte("invalid")}}}
	if _, err := StageCaptureInputs(directory, artifact); err == nil {
		t.Fatal("unconfined or corrupt child accepted")
	}

	if _, err := os.Stat(filepath.Join(directory, "content")); !os.IsNotExist(err) {
		t.Fatal("invalid batch created content output", err)
	}
}
