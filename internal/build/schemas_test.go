package build_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/build"
	worldschema "github.com/karty-game/karty/internal/worldbuild/schema"
)

func TestInvalidWorldFailsBeforeSDKOrToolchainPreparation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "levels", "tiny"), 0750); err != nil {
		t.Fatal(err)
	}

	for name, contents := range map[string]string{
		"karty.toml":             "[project]\nname='tiny'\n[project.resolution]\nwidth=64\nheight=64\n[sdk]\nversion='99.0.0'\n",
		"go.mod":                 "module example.com/tiny\n\ngo 1.27.0\n",
		"levels/tiny/level.toml": "[level]\nname='levels.tiny'\nkind='world'\n[world]\nsource='world.yaml'\n",
		"levels/tiny/world.yaml": "version: 6\nunknown_field: true\n",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}

	if err := build.RunWithOptions(t.Context(), root, build.Options{}); !errors.Is(err, worldschema.ErrValidation) {
		t.Fatalf("prepared nonexistent SDK before checking YAML: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".karty", "schemas", "levels", "tiny.schema.json")); err != nil {
		t.Fatal("invalid world must still receive an editor schema:", err)
	}
}
