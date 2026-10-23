package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/karty-game/karty/internal/testpackages"
)

const guideControllerPath = "example.com/my-game/.karty/processed/code/controllers-3d.sdk/controller"

// Type-check documentation with the package's actual pinned adapter, using the
// same SDK package identity as the surrounding examples.
func controllerExampleSources(t *testing.T) map[string][]byte {
	t.Helper()

	source, err := testpackages.Source("controllers-3d")
	if err != nil {
		t.Fatal(err)
	}

	files, err := filepath.Glob(filepath.Join(source, "bridge/templates/*.go.tmpl"))
	if err != nil || len(files) == 0 {
		t.Fatalf("controller adapter templates: %v", err)
	}

	result := make(map[string][]byte, len(files))

	for _, name := range files {
		contents, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}

		parsed, err := template.New(name).Parse(string(contents))
		if err != nil {
			t.Fatal(err)
		}

		var output bytes.Buffer
		if err := parsed.Execute(&output, struct{ Module string }{"example.com/my-game"}); err != nil {
			t.Fatal(err)
		}

		result[strings.TrimSuffix(filepath.Base(name), ".tmpl")] = output.Bytes()
	}

	return result
}
