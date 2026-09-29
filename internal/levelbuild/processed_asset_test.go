package levelbuild

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestReadVerifiedProcessedAssetRejectsReplacement(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "payload")
	original := []byte("verified output")

	digest := sha256.Sum256(original)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := readVerifiedProcessedAsset(path, 1024, hex.EncodeToString(digest[:])); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("replaced output"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := readVerifiedProcessedAsset(path, 1024, hex.EncodeToString(digest[:])); err == nil {
		t.Fatal("readVerifiedProcessedAsset() accepted a replaced cache payload")
	}
}
