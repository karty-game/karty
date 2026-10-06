package actionbuild

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const declarationsSource = `package main
import game "example.com/game/.karty/engine"
//karty:action door.open
func OpenDoor(ctx game.ActionContext, target game.WorldActorRef, speed float32) error { return nil }
//karty:condition door.unlocked
func Unlocked(ctx game.ActionContext, key bool) (bool,error) { return key,nil }
`

func fixture(t *testing.T) (string, []Declaration, []Level, []byte) {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte(declarationsSource), 0600); err != nil {
		t.Fatal(err)
	}

	level := filepath.Join(root, "levels", "room")
	if err := os.MkdirAll(level, 0750); err != nil {
		t.Fatal(err)
	}

	declarations, err := Discover(root, "example.com/game")
	if err != nil {
		t.Fatal(err)
	}

	schema, err := os.ReadFile("testdata/actions-v1.generated.json")
	if err != nil {
		t.Fatal(err)
	}

	return root, declarations, []Level{{Name: "room", Directory: level, Actors: map[string]bool{"tower/door": true}}}, schema
}
func TestCatalogSchemaAndTypedInvocation(t *testing.T) {
	t.Parallel()

	root, declarations, levels, schema := fixture(t)

	script := `{"version":1,"sequences":[{"name":"enter","onRepeat":"restart","steps":[{"condition":"door.unlocked","args":{"key":true},"then":[{"action":"door.open","args":{"target":{"actor":"tower/door"},"speed":2.5},"onFailure":[{"waitFrames":1}]}],"else":[{"waitFrames":3}]}]}]}`
	if err := os.WriteFile(filepath.Join(levels[0].Directory, "actions.json"), []byte(script), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := Compile(root, "example.com/game", declarations, levels, schema)
	if err != nil {
		t.Fatal(err)
	}

	for _, text := range []string{"StartAuthoredSequence", "context.Actor(\"tower/door\")", "OpenDoor(context, actor0, float32(2.5))", "Unlocked(context, true)", "SequenceRestart", "OnFailure:", "WaitFrames: 3"} {
		if !bytes.Contains(result.Source, []byte(text)) {
			t.Fatalf("missing %q in %s", text, result.Source)
		}
	}

	parsed, err := readJSON(result.Schema)
	if err != nil {
		t.Fatal(err)
	}

	document, err := readJSON([]byte(script))
	if err != nil {
		t.Fatal(err)
	}

	parsedSchema, validSchema := parsed.(map[string]any)
	if !validSchema {
		t.Fatal("expected schema object")
	}

	if err = validateSchema(document, parsedSchema, parsedSchema, "test", 0); err != nil {
		t.Fatal(err)
	}

	again, err := Compile(root, "example.com/game", declarations, levels, schema)
	if err != nil || !bytes.Equal(again.Source, result.Source) || !bytes.Equal(again.Catalog, result.Catalog) ||
		!bytes.Equal(again.Schema, result.Schema) {
		t.Fatal("nondeterministic catalog/adapters", err)
	}

	var catalog struct {
		Actions []Declaration `json:"actions"`
	}
	if err = json.Unmarshal(result.Catalog, &catalog); err != nil || len(catalog.Actions) != 2 {
		t.Fatal(catalog, err)
	}
}
func TestInvalidAuthoredArgumentsFailBeforeCompilation(t *testing.T) {
	t.Parallel()

	for _, script := range []string{
		`{"version":2,"sequences":[]}`,
		`{"version":1,"sequences":[{"name":"enter","steps":[{"action":"missing","args":{}}]}]}`,
		`{"version":1,"sequences":[{"name":"enter","steps":[{"action":"door.open","args":{"target":{"actor":"missing"},"speed":2}}]}]}`,
		`{"version":1,"sequences":[{"name":"enter","steps":[{"action":"door.open","args":{"target":{"actor":"tower/door"},"speed":"fast"}}]}]}`,
		`{"version":1,"sequences":[{"name":"enter","steps":[{"action":"door.open","args":{"target":{"actor":"tower/door"},"speed":1e40}}]}]}`,
		`{"version":1,"sequences":[{"name":"enter","steps":[{"waitFrames":0}]}]}`,
		`{"version":1,"sequences":[{"name":"enter","steps":[]},{"name":"enter","steps":[]}]}`,
		`{"version":1,"version":1,"sequences":[]}`,
		`{"version":1,"sequences":[],"extra":true}`,
	} {
		t.Run(script, func(t *testing.T) {
			t.Parallel()

			root, declarations, levels, schema := fixture(t)
			if err := os.WriteFile(filepath.Join(levels[0].Directory, "actions.json"), []byte(script), 0600); err != nil {
				t.Fatal(err)
			}

			if _, err := Compile(root, "example.com/game", declarations, levels, schema); err == nil {
				t.Fatal("invalid actions accepted")
			}
		})
	}
}
func TestDeclarationsRejectAmbiguityAndUnsupportedSignatures(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		strings.Replace(declarationsSource, "speed float32", "speed []float32", 1),
		strings.Replace(declarationsSource, "ctx game.ActionContext", "ctx string", 1),
		strings.Replace(declarationsSource, "error { return nil }", "bool { return true }", 1),
		strings.Replace(declarationsSource, "door.unlocked", "door.open", 1),
		"//go:build linux\n\n" + declarationsSource,
		strings.Replace(declarationsSource, "//karty:action door.open", "//karty:action door.open extra", 1),
	} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "src"), 0750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}

		if _, err := Discover(root, "example.com/game"); err == nil {
			t.Fatal("invalid declaration accepted", source)
		}
	}
}
func TestActionDiscoveryConfinement(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0750); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(t.TempDir(), "secret.go")
	if err := os.WriteFile(outside, []byte(declarationsSource), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outside, filepath.Join(root, "src", "linked.go")); err != nil {
		t.Fatal(err)
	}

	if _, err := Discover(root, "example.com/game"); err == nil {
		t.Fatal("symlink escaped discovery")
	}
}

func TestStringAndIntegerArgumentBounds(t *testing.T) {
	t.Parallel()

	root, _, levels, schema := fixture(t)

	declaration := `package main
import game "example.com/game/.karty/engine"
//karty:action actor.configure
func Configure(ctx game.ActionContext, text string, signed int32, unsigned uint32) error { return nil }
`
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte(declaration), 0600); err != nil {
		t.Fatal(err)
	}

	declarations, err := Discover(root, "example.com/game")
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		text, signed, unsigned string
		valid                  bool
	}{
		{`"hello"`, "-2147483648", "4294967295", true},
		{`""`, "2147483647", "0", true},
		{strconv.Quote(strings.Repeat("a", 1025)), "0", "0", false},
		{`"hello"`, "-2147483649", "0", false},
		{`"hello"`, "2147483648", "0", false},
		{`"hello"`, "0", "4294967296", false},
		{`"hello"`, "0", "-1", false},
		{`"hello"`, "0.5", "0", false},
	} {
		script := `{"version":1,"sequences":[{"name":"enter","steps":[{"action":"actor.configure","args":{"text":` + test.text + `,"signed":` + test.signed + `,"unsigned":` + test.unsigned + `}}]}]}`
		if err := os.WriteFile(filepath.Join(levels[0].Directory, "actions.json"), []byte(script), 0600); err != nil {
			t.Fatal(err)
		}

		_, err := Compile(root, "example.com/game", declarations, levels, schema)
		if (err == nil) != test.valid {
			t.Fatalf("%s: valid=%v, error=%v", script, test.valid, err)
		}
	}
}

func TestEditorSchemaAvailableBeforeAuthoredDocument(t *testing.T) {
	t.Parallel()

	root, declarations, levels, schema := fixture(t)

	result, err := Compile(root, "example.com/game", declarations, levels, schema)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(result.LevelSchemas["room"], []byte(`"tower/door"`)) {
		t.Fatal("missing level actor completion")
	}
}

func TestMalformedSDKSchemaFailsCleanly(t *testing.T) {
	t.Parallel()

	root, declarations, levels, _ := fixture(t)
	for _, schema := range []string{`{"$defs":false}`, `{"oneOf":[false]}`, `{"type":42}`, `{"properties":{"name":null}}`, `{"required":[5]}`, `{"items":false}`, `{"pattern":"["}`, `{}`, `{"$defs":{"step":{"oneOf":[]}}}`} {
		if _, err := Compile(root, "example.com/game", declarations, levels, []byte(schema)); err == nil {
			t.Fatal("malformed schema accepted", schema)
		}
	}
}
