package dev

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedDevBuildResolvesSDKInsteadOfPassingHostOverride(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KARTY_HOST_WEB", "")

	host, err := devHost("")
	if err != nil {
		t.Fatal(err)
	}

	command := strings.Join(devBuildCommand("karty", host), " ")
	if host != "" || strings.Contains(command, "--host") {
		t.Fatalf("managed build converted to local override: %s", command)
	}

	if !strings.Contains(command, "--target web") {
		t.Fatal(command)
	}
}

func TestDevBuildPreservesLocalHostAndRejectsMissingOverride(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "local host.wasm")
	if err := os.WriteFile(path, []byte("wasm"), 0600); err != nil {
		t.Fatal(err)
	}

	host, err := devHost(path)
	if err != nil {
		t.Fatal(err)
	}

	command := strings.Join(devBuildCommand("karty", host), " ")
	if !strings.Contains(command, "--host "+shellQuote(path)) {
		t.Fatal(command)
	}

	if _, err := devHost(path + ".missing"); err == nil {
		t.Fatal("missing override accepted")
	}
}
