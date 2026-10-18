package commands_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/commands"
	"github.com/karty-game/karty/internal/release"
)

func TestCLIReportsBuildVersion(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	command := commands.New()
	command.Writer = &output

	if err := command.Run(t.Context(), []string{"karty", "--version"}); err != nil {
		t.Fatal(err)
	}

	if got, want := strings.TrimSpace(output.String()), "karty version "+release.Version; got != want {
		t.Fatalf("CLI version = %q, want %q", got, want)
	}
}

func TestNewIncludesTopLevelCommands(t *testing.T) {
	t.Parallel()

	command := commands.New()

	names := make(map[string]bool, len(command.Commands))
	for _, child := range command.Commands {
		names[child.Name] = true
	}

	for _, name := range []string{"new", "package", "fmt", "lint", "build", "bake", "screenshot", "schema", "dev", "serve", "toolchain"} {
		if !names[name] {
			t.Errorf("root command missing %q", name)
		}
	}
}
