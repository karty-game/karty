package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/project"
)

func TestModulePathAcceptsGoModuleSyntax(t *testing.T) {
	t.Parallel()

	for name, source := range map[string]string{
		"plain":          "module example.com/game\n",
		"comment":        "module example.com/game // project identity\n",
		"quoted":         "module \"example.com/game\"\n",
		"quoted comment": "module \"example.com/game\" // project identity\n",
		"leading comment": "// module example.com/ignored\n\n" +
			"module example.com/game\n\ngo 1.27.0\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}

			module, err := project.ModulePath(directory)
			if err != nil || module != "example.com/game" {
				t.Fatalf("ModulePath() = %q, %v", module, err)
			}
		})
	}
}
