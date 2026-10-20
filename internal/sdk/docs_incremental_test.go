package sdk

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestSyncDocsRefreshesChangedCandidateWithSameVersion(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	resources := fstest.MapFS{"docs/1/README.md": {Data: []byte("original reference")}}
	manifest := Manifest{Version: "9.9.9", resources: resources}

	manifest.Docs.Version = "1"
	if err := SyncDocsContext(t.Context(), project, manifest); err != nil {
		t.Fatal(err)
	}

	resources["docs/1/README.md"] = &fstest.MapFile{Data: []byte("updated reference")}

	if err := SyncDocsContext(t.Context(), project, manifest); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(project, ".karty", "docs", "README.md"))
	if err != nil || string(data) != "updated reference" {
		t.Fatalf("candidate documentation = %q, %v", data, err)
	}
}
