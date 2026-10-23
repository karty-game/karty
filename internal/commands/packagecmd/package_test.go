package packagecmd_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/commands/packagecmd"
	"github.com/karty-game/karty/internal/packages"
	"github.com/urfave/cli/v3"
)

func TestUpgradeRequiresOnePackageName(t *testing.T) {
	t.Parallel()

	for _, arguments := range [][]string{nil, {"fps-kit", "savegame"}} {
		t.Run(strings.Join(arguments, ","), func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer

			command := &cli.Command{Name: "karty", Writer: &output, ErrWriter: &output, Commands: []*cli.Command{packagecmd.Command()}}

			args := append([]string{"karty", "package", "upgrade"}, arguments...)

			if err := command.Run(
				t.Context(),
				args,
			); !errors.Is(err, packages.ErrPackage) ||
				!strings.Contains(err.Error(), "one installed package name") {
				t.Fatalf("argument error = %v", err)
			}
		})
	}
}

func TestUpgradeHelpExplainsLatestStableSelection(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	command := &cli.Command{Name: "karty", Writer: &output, ErrWriter: &output, Commands: []*cli.Command{packagecmd.Command()}}
	if err := command.Run(t.Context(), []string{"karty", "package", "upgrade", "--help"}); err != nil {
		t.Fatal(err)
	}

	for _, text := range []string{"latest stable catalog release", "karty package upgrade", "<name>"} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("upgrade help is missing %q: %s", text, output.String())
		}
	}
}
