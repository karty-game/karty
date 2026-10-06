package schema_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/commands/schema"
	"github.com/urfave/cli/v3"
)

//nolint:paralleltest // t.Chdir changes the process working directory and cannot run in parallel.
func TestCommandGeneratesBaseWithoutProjectOrSDK(t *testing.T) {
	t.Chdir(t.TempDir())

	var output bytes.Buffer

	command := &cli.Command{Name: "karty", Commands: []*cli.Command{schema.Command()}, Writer: &output}
	if err := command.Run(t.Context(), []string{"karty", "schema", "--check"}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output.String(), "0 levels") {
		t.Fatal("missing schema generation feedback:", output.String())
	}

	if _, err := os.Stat(filepath.Join(".karty", "schemas", "worldsource.base.schema.json")); err != nil {
		t.Fatal(err)
	}
}
