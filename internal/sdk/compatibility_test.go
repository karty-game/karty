package sdk

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io/fs"
	"testing"

	"github.com/karty-game/karty/internal/release"
)

func TestBundleExposesDeclaredCompatibility(t *testing.T) {
	t.Parallel()

	data, err := currentTestBundle(t)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := fs.ReadFile(archive, "bundle.json")
	if err != nil {
		t.Fatal(err)
	}

	var expected Compatibility

	if err := json.Unmarshal(metadata, &expected); err != nil {
		t.Fatal(err)
	}

	manifest, err := readBundle(release.SDKVersion(), data)
	if err != nil {
		t.Fatal(err)
	}

	if manifest.Compatibility != expected {
		t.Fatalf("declared compatibility lost: got %+v, want %+v", manifest.Compatibility, expected)
	}

	if !manifest.SupportsAuthoredActions() {
		t.Fatal("current SDK must retain authored action support")
	}
}
