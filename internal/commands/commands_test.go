package commands_test

import (
	"testing"

	"github.com/karty-game/karty/internal/commands"
)

func TestNewIncludesTopLevelCommands(t *testing.T) {
	t.Parallel()

	command := commands.New()

	names := make(map[string]bool, len(command.Commands))
	for _, child := range command.Commands {
		names[child.Name] = true
	}

	for _, name := range []string{"new", "build", "bake", "screenshot", "schema", "dev", "serve", "toolchain"} {
		if !names[name] {
			t.Errorf("root command missing %q", name)
		}
	}
}
