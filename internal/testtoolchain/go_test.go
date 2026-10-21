package testtoolchain

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/process"
	"github.com/karty-game/karty/internal/sdk"
)

func TestGoFromMiseUsesSDKPin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	path, err := goFromMise(t.Context(), "1.27.1", func(ctx context.Context, args ...string) ([]byte, error) {
		if ctx != t.Context() || !slices.Equal(args, []string{"exec", "go@1.27.1", "--", "go", "env", "GOROOT"}) {
			t.Fatalf("mise invocation: %v, %v", ctx, args)
		}

		return []byte(root + "\r\n"), nil
	})
	if err != nil || filepath.Dir(filepath.Dir(path)) != root || !strings.HasPrefix(filepath.Base(path), "go") {
		t.Fatalf("Go path = %q, %v", path, err)
	}
}

func TestGoFromMiseRejectsFailedOrInvalidSelection(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct {
		name, version, output string
		failure               error
	}{
		{name: "missing SDK pin"},
		{name: "install failed", version: "1.27.1", output: "download refused", failure: os.ErrPermission},
		{name: "empty root", version: "1.27.1"},
		{name: "relative root", version: "1.27.1", output: "relative"},
		{name: "extra output", version: "1.27.1", output: t.TempDir() + "\nextra"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			_, err := goFromMise(t.Context(), scenario.version, func(context.Context, ...string) ([]byte, error) {
				if scenario.version == "" {
					t.Fatal("mise invoked without an SDK Go pin")
				}

				return []byte(scenario.output), scenario.failure
			})
			if err == nil ||
				scenario.failure != nil && (!errors.Is(err, scenario.failure) || !strings.Contains(err.Error(), scenario.output)) {
				t.Fatalf("invalid mise result accepted or diagnostic lost: %v", err)
			}
		})
	}
}

func TestGoHonorsExplicitCompilerAndRejectsWrongVersion(t *testing.T) {
	want, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}

	output, err := process.CommandContext(t.Context(), want, "version").Output()
	if err != nil {
		t.Fatal(err)
	}

	fields := strings.Fields(string(output))
	if len(fields) != 4 || fields[0] != "go" || fields[1] != "version" {
		t.Fatalf("unexpected compiler version: %q", output)
	}

	t.Setenv("KARTY_GO", want)
	// Explicit compilers must work without mise being installed or available.
	t.Setenv("PATH", t.TempDir())

	manifest := sdk.Manifest{}
	manifest.Tools.Go = strings.TrimPrefix(fields[2], "go")

	got, err := Go(t.Context(), manifest)
	if err != nil || got != want {
		t.Fatalf("explicit Go = %q, %v; want %q", got, err, want)
	}

	manifest.Tools.Go = "0.0.0"
	if _, err = Go(t.Context(), manifest); err == nil {
		t.Fatal("explicit compiler version mismatch accepted")
	}
}
