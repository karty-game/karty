package build_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/build"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestProductionBuildRejectsDecoderBeforeCompilationOrArtifactReplacement(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "game")
	if err := createTexturedGame(directory, manifest); err != nil {
		t.Fatal(err)
	}

	compiler, wasmTools := buildTools(t)

	script := "#!/bin/sh\nif [ \"$1\" = list ]; then printf '%s\\n' example.com/wrapper encoding/json; exit 0; fi\necho unexpected compilation >&2\nexit 99\n"
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nif \"%~1\"==\"list\" (echo example.com/wrapper & echo encoding/json & exit /b 0)\r\necho unexpected compilation >&2\r\nexit /b 99\r\n"
	}

	if err := os.WriteFile(compiler, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(compiler, 0700); err != nil {
		t.Fatal(err)
	}

	raw := filepath.Join(directory, "dist", "raw")
	if err := os.MkdirAll(raw, 0755); err != nil {
		t.Fatal(err)
	}

	artifact := filepath.Join(raw, "game.kart")

	const previous = "previous cartridge"

	if err := os.WriteFile(artifact, []byte(previous), 0600); err != nil {
		t.Fatal(err)
	}

	err = build.RunWithOptions(t.Context(), directory, build.Options{TinyGo: compiler, WasmTools: wasmTools})
	if err == nil || !strings.Contains(err.Error(), "encoding/json") || strings.Contains(err.Error(), "unexpected compilation") {
		t.Fatalf("production policy was not applied before compilation: %v", err)
	}

	data, readErr := os.ReadFile(artifact)
	if readErr != nil || string(data) != previous {
		t.Fatal("rejected dependency replaced the previous artifact", readErr)
	}
}
