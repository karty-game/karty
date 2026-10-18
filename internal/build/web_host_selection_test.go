package build

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWebHostSelectionIgnoresExecutableRelativeArtifacts(t *testing.T) {
	t.Parallel()

	const child = "KARTY_TEST_WEB_HOST_SELECTION_CHILD"

	if os.Getenv(child) == "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}

		legacy := filepath.Join(filepath.Dir(executable), "..", "..", "host", "web", "karty-host.wasm")
		if info, err := os.Stat(legacy); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("legacy host fixture unavailable: %v", err)
		}

		if host, err := findWebHostArtifact(); host != "" || !errors.Is(err, errWebHostRequired) {
			t.Fatalf("implicit host selection = %q, %v; want SDK host resolution", host, err)
		}

		return
	}

	root := t.TempDir()

	legacy := filepath.Join(root, "host", "web", "karty-host.wasm")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(legacy, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	relocated := filepath.Join(root, "cli", "bin", filepath.Base(executable))
	if err := os.MkdirAll(filepath.Dir(relocated), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.Link(executable, relocated); err != nil {
		contents, err := os.ReadFile(executable)
		if err != nil {
			t.Fatal(err)
		}

		// #nosec G703 -- The executable copy is inside this test's freshly created t.TempDir.
		if err := os.WriteFile(relocated, contents, 0o600); err != nil {
			t.Fatal(err)
		}

		// #nosec G703 -- The executable copy is inside this test's freshly created t.TempDir.
		if err := os.Chmod(relocated, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	// #nosec G204 -- The subprocess is this test binary and runs only this exact regression.
	command := exec.CommandContext(
		t.Context(), relocated,
		"-test.run=^TestWebHostSelectionIgnoresExecutableRelativeArtifacts$", "-test.count=1",
	)

	command.Env = append(os.Environ(), child+"=1", "KARTY_HOST_WEB=")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("relocated CLI host selection: %v\n%s", err, output)
	}
}

func TestWebHostSelectionKeepsExplicitEnvironmentOverride(t *testing.T) {
	host := filepath.Join(t.TempDir(), "custom-host.wasm")
	if err := os.WriteFile(host, []byte("custom host"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("KARTY_HOST_WEB", host)

	if selected, err := findWebHostArtifact(); err != nil || selected != host {
		t.Fatalf("explicit host selection = %q, %v; want %q", selected, err, host)
	}
}
