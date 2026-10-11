package actionbuild

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/actions"
)

func TestYAMLActionsEmitTypedBinary(t *testing.T) {
	t.Parallel()

	const source = "version: 1\nsequences:\n  - name: enter\n    steps:\n      - action: door.open\n        args:\n          target: {actor: tower/door}\n          speed: 2.5\n      - waitFrames: 1\n"

	data, err := CompileDocument([]byte(source))
	if err != nil {
		t.Fatal(err)
	}

	document, err := actions.Decode(data)
	if err != nil || !bytes.HasPrefix(data, []byte("KTYA")) || len(document.Sequences) != 1 {
		t.Fatal("authored source was not compiled to binary", err)
	}

	step := document.Sequences[0].Steps[0]
	actor, _ := step.Args.Object("target")
	name, _ := actor.String("actor")
	speed, _ := step.Args["speed"].AsFloat()

	if name != "tower/door" || speed != 2.5 {
		t.Fatal("typed arguments lost")
	}

	for _, malformed := range []string{"version: 1\nversion: 1\nsequences: []\n", "version: 1\nsequences: []\nextra: true\n", "version: 1\nsequences: &list []\nother: *list\n"} {
		if _, err := CompileDocument([]byte(malformed)); err == nil {
			t.Fatal("accepted malformed YAML")
		}
	}
}
func TestActionsJSONFileMustBeMigrated(t *testing.T) {
	t.Parallel()

	root, declarations, levels, schema := fixture(t)
	if err := os.WriteFile(filepath.Join(levels[0].Directory, "actions.json"), []byte(`{"version":1,"sequences":[]}`), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := Compile(root, "example.com/game", declarations, levels, schema); err == nil {
		t.Fatal("accepted legacy JSON source file")
	}
}
