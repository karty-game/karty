package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/build"
)

func TestRuntimeProbeRespectsClientDependencyPolicy(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("testdata/runtime-probe.go")
	if err != nil {
		t.Fatal(err)
	}

	directory := t.TempDir()
	for name, source := range map[string][]byte{
		"go.mod":                  []byte("module example.com/runtime-probe\n\ngo 1.27.0\n"),
		"src/main.go":             contents,
		".karty/engine/engine.go": []byte("package engine\n"),
		".karty/assets/assets.go": []byte("package assets\n"),
	} {
		writeComponentFixture(t, directory, name, source)
	}

	compiler, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}

	// Inspect the probe's actual imports without compiling its generated API.
	// Generated SDK bindings have separate transitive dependency coverage.
	if err := build.CheckClientDependencies(t.Context(), directory, "go", compiler, "./src"); err != nil {
		t.Fatalf("runtime probe violates the cartridge dependency policy: %v", err)
	}
}

func TestRuntimeProbeDoesNotOwnContributorInstrumentation(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("testdata/runtime-probe.go")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(contents), "karty_alloccheck") || strings.Contains(string(contents), "karty_allocations") {
		t.Fatal("runtime probe contains contributor-only allocation instrumentation")
	}
}
