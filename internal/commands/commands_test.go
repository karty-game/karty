package commands_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/commands"
	"github.com/karty-game/karty/internal/release"
	"github.com/urfave/cli/v3"
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

func TestRootHelpCommands(t *testing.T) {
	t.Parallel()

	command := commands.New()

	names := make([]string, 0, len(command.VisibleCommands()))
	for _, child := range command.VisibleCommands() {
		names = append(names, child.Name)
	}

	want := []string{"new", "dev", "build", "serve", "fmt", "lint", "schema", "package", "render", "sdk"}
	if !slices.Equal(names, want) {
		t.Fatalf("visible root commands = %v, want %v", names, want)
	}

	var output bytes.Buffer

	command.Writer = &output
	if err := command.Run(t.Context(), []string{"karty", "--help"}); err != nil {
		t.Fatal(err)
	}

	for line := range strings.SplitSeq(output.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && slices.Contains([]string{"bake", "screenshot", "toolchain"}, fields[0]) {
			t.Errorf("legacy command in root help: %s", line)
		}
	}
}

func TestCommandGroupHelp(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		path []string
		want []string
	}{
		{[]string{"render"}, []string{"bake", "screenshot"}},
		{[]string{"sdk"}, []string{"current", "install", "toolchain"}},
		{[]string{"sdk", "toolchain"}, []string{"install"}},
		{[]string{"render", "bake"}, []string{"--samples", "--incremental"}},
		{[]string{"render", "screenshot"}, []string{"--batch", "--position"}},
	} {
		t.Run(strings.Join(test.path, "/"), func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer

			command := commands.New()
			command.Writer = &output

			args := append([]string{"karty"}, test.path...)
			if err := command.Run(t.Context(), append(args, "--help")); err != nil {
				t.Fatal(err)
			}

			for _, text := range append(test.want, "karty "+strings.Join(test.path, " ")) {
				if !strings.Contains(output.String(), text) {
					t.Errorf("help missing %q:\n%s", text, output.String())
				}
			}
		})
	}
}

func TestGroupedAndLegacyCommandRouting(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		path []string
		args []string
	}{
		{[]string{"render", "bake"}, []string{"--level", "levels.room", "--bounces", "0", "--incremental"}},
		{[]string{"bake"}, []string{"--level", "levels.room", "--bounces", "0", "--incremental"}},
		{[]string{"render", "screenshot"}, []string{"levels.room", "--batch", "views.toml", "--width", "640"}},
		{[]string{"screenshot"}, []string{"levels.room", "--batch", "views.toml", "--width", "640"}},
		{[]string{"sdk", "toolchain", "install"}, nil},
		{[]string{"toolchain", "install"}, nil},
	} {
		t.Run(strings.Join(test.path, "/"), func(t *testing.T) {
			t.Parallel()

			root := commands.New()

			leaf := root
			for _, name := range test.path {
				leaf = leaf.Command(name)
				if leaf == nil {
					t.Fatalf("missing command %q", name)
				}
			}

			called := false
			leaf.Action = func(_ context.Context, command *cli.Command) error {
				called = true

				switch command.Name {
				case "bake":
					if command.String("level") != "levels.room" || !command.IsSet("bounces") ||
						command.Int("bounces") != 0 || !command.Bool("incremental") {
						t.Error("bake flags were not retained")
					}
				case "screenshot":
					if command.Args().First() != "levels.room" || command.String("batch") != "views.toml" ||
						command.Int("width") != 640 {
						t.Error("screenshot arguments and flags were not retained")
					}
				}

				return nil
			}

			args := append([]string{"karty"}, test.path...)
			if err := root.Run(t.Context(), append(args, test.args...)); err != nil {
				t.Fatal(err)
			}

			if !called {
				t.Fatal("command action was not called")
			}
		})
	}
}

func TestSDKCurrent(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	command := commands.New()
	command.Writer = &output

	if err := command.Run(t.Context(), []string{"karty", "sdk", "current"}); err != nil {
		t.Fatal(err)
	}

	if got := strings.TrimSpace(output.String()); got != release.SDKVersion() {
		t.Fatalf("SDK version = %q, want %q", got, release.SDKVersion())
	}
}

func TestUsageErrorsMatchCommandPath(t *testing.T) {
	t.Parallel()

	for _, path := range [][]string{{"render", "screenshot"}, {"screenshot"}, {"sdk", "install"}} {
		t.Run(strings.Join(path, "/"), func(t *testing.T) {
			t.Parallel()

			err := commands.New().Run(t.Context(), append([]string{"karty"}, path...))

			want := "usage: karty " + strings.Join(path, " ")
			if !errors.Is(err, os.ErrInvalid) || !strings.Contains(err.Error(), want) {
				t.Fatalf("usage error = %v, want %q wrapping os.ErrInvalid", err, want)
			}
		})
	}
}
