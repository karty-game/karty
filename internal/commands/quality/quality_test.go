package quality

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

func qualityProject(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())

	if err := os.WriteFile("karty.toml", []byte("[project]\nname='demo'\n[sdk]\nversion='0.0.12'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

//nolint:paralleltest // Configuration discovery uses the process working directory.
func TestConfigurationPrecedenceAndCleanup(t *testing.T) {
	qualityProject(t)

	path, cleanup, err := configuration("")
	if err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(contents, defaultConfig) {
		t.Fatalf("default config: %s, %v", contents, err)
	}

	cleanup()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary config remains: %v", err)
	}

	if err := os.WriteFile(".golangci.yaml", []byte("user config"), 0o600); err != nil {
		t.Fatal(err)
	}

	path, cleanup, err = configuration("")
	if err != nil || path != ".golangci.yaml" {
		t.Fatalf("project config = %q, %v", path, err)
	}

	cleanup()

	path, cleanup, err = configuration("custom.toml")
	if err != nil || path != "custom.toml" {
		t.Fatalf("explicit config = %q, %v", path, err)
	}

	cleanup()

	if contents, err := os.ReadFile(".golangci.yaml"); err != nil || string(contents) != "user config" {
		t.Fatal("modified user config")
	}
}

func TestQualityRunsWASIAndPropagatesFailures(t *testing.T) {
	qualityProject(t)
	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "amd64")
	t.Setenv("GOWORK", "/unrelated/go.work")

	failure := os.ErrPermission
	for _, example := range []struct {
		name      string
		args      []string
		want      []string
		output    string
		failure   error
		wantError bool
	}{
		{name: "fmt", want: []string{"fmt", "./..."}},
		{name: "fmt", args: []string{"--check", "./src/..."}, want: []string{"fmt", "--diff", "./src/..."}, output: "diff --git a/src/main.go b/src/main.go\n", wantError: true},
		{name: "lint", args: []string{"--fix", "./src/..."}, want: []string{"run", "--fix", "./src/..."}, failure: failure, wantError: true},
	} {
		var output bytes.Buffer

		command := Command(example.name)
		command.Action = func(ctx context.Context, command *cli.Command) error {
			return run(ctx, command, example.name, func(child *exec.Cmd) error {
				assertToolInvocation(t, child, example.want)

				_, err := io.WriteString(child.Stdout, example.output)
				if err != nil {
					t.Fatal(err)
				}

				return example.failure
			})
		}
		root := &cli.Command{Name: "karty", Commands: []*cli.Command{command}, Writer: &output, ErrWriter: &output}

		args := append([]string{"karty", example.name, "--golangci-lint", "/unused/golangci-lint"}, example.args...)

		err := root.Run(context.Background(), args)
		if (err != nil) != example.wantError {
			t.Fatalf("%v: %v", args, err)
		}

		if example.failure != nil && !errors.Is(err, example.failure) {
			t.Fatalf("lost tool failure: %v", err)
		}

		if !strings.Contains(output.String(), example.output) {
			t.Fatal("lost tool output")
		}
	}
}

func assertToolInvocation(t *testing.T, child *exec.Cmd, arguments []string) {
	t.Helper()

	for _, want := range arguments {
		if !slices.Contains(child.Args, want) {
			t.Errorf("args %v missing %q", child.Args, want)
		}
	}

	for _, want := range []string{"GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0", "GOWORK=off"} {
		if !slices.Contains(child.Env, want) {
			t.Errorf("environment missing %s", want)
		}
	}

	for _, value := range child.Env {
		if value == "GOOS=linux" || value == "GOWORK=/unrelated/go.work" {
			t.Errorf("inherited %s", value)
		}
	}
}
