package build_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty/internal/build"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestDevelopmentBakesRequireDevelopmentMode(t *testing.T) {
	t.Parallel()

	if err := build.RunWithOptions(t.Context(), t.TempDir(), build.Options{DevelopmentBakes: true}); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("development baking accepted outside development mode: %v", err)
	}
}

func TestDevelopmentCompilerDoesNotChangeFinalBuildSelection(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell compiler fixture; real Go/TinyGo execution is covered by check-web")
	}

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(t.TempDir(), "game")
	if err := createTexturedGame(root, manifest); err != nil {
		t.Fatal(err)
	}

	tinygo, wasmTools := buildTools(t)

	script, err := os.ReadFile(tinygo)
	if err != nil {
		t.Fatal(err)
	}

	goTool := filepath.Join(t.TempDir(), "go")

	script = []byte(
		strings.Replace(
			string(script),
			"#!/bin/sh\n",
			fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = version ]; then echo 'go version go%s linux/amd64'; exit 0; fi\n", manifest.Tools.Go),
			1,
		),
	)
	//nolint:gosec // Fixed compiler filename inside a private test-owned temporary directory.
	if err := os.WriteFile(goTool, script, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(goTool, 0o700); err != nil {
		t.Fatal(err)
	}

	config, err := os.ReadFile(filepath.Join(root, "karty.toml"))
	if err != nil {
		t.Fatal(err)
	}

	for _, development := range []bool{true, false} {
		if err := build.RunWithOptions(
			t.Context(),
			root,
			build.Options{Development: development, Go: goTool, TinyGo: tinygo, WasmTools: wasmTools},
		); err != nil {
			t.Fatal(err)
		}

		bytes, err := os.ReadFile(filepath.Join(root, "dist/raw/game.kart"))
		if err != nil {
			t.Fatal(err)
		}

		section, err := cartridge.ExtractSection(bytes, cartridge.ManifestSectionName)
		if err != nil {
			t.Fatal(err)
		}

		actual, err := cartridge.DecodeManifest(section)
		if err != nil {
			t.Fatal(err)
		}

		want := "tinygo"
		if development {
			want = "go"
		}

		if actual.Compiler != want {
			t.Fatalf("development=%v selected %s, want %s", development, actual.Compiler, want)
		}
	}

	after, err := os.ReadFile(filepath.Join(root, "karty.toml"))
	if err != nil || string(after) != string(config) {
		t.Fatal("development rewrote the project's final build configuration", err)
	}
}
