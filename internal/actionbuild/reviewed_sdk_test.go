package actionbuild

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewedSDKUsesEntityLookup(t *testing.T) {
	t.Parallel()
	root, declarations, levels, schema := fixture(t)

	script := `{"version":1,"sequences":[{"name":"enter","steps":[{"action":"door.open","args":{"target":{"actor":"tower/door"},"speed":2.5}}]}]}`
	if err := os.WriteFile(filepath.Join(levels[0].Directory, "actions.yaml"), []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Compile(root, "example.com/game", declarations, levels, schema, 3)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(result.Source, []byte(`context.Entity("tower/door")`)) || bytes.Contains(result.Source, []byte("context.Actor(")) {
		t.Fatalf("wrong lookup contract: %s", result.Source)
	}
}
