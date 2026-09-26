package toolchain_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/karty-game/karty/internal/toolchain"
)

func TestTinyGoUsesOverrideBeforeEnvironment(t *testing.T) {
	t.Parallel()
	override := writeExecutable(t, filepath.Join(t.TempDir(), "override"))
	environment := writeExecutable(t, filepath.Join(t.TempDir(), "environment"))

	path, err := toolchain.TinyGo(toolchain.TinyGoOptions{Override: override, Getenv: func(string) string { return environment }})
	if err != nil {
		t.Fatalf("TinyGo() error = %v", err)
	}

	if path != override {
		t.Errorf("TinyGo() = %q, want %q", path, override)
	}
}

func TestTinyGoUsesEnvironmentBeforeCache(t *testing.T) {
	t.Parallel()
	environment := writeExecutable(t, filepath.Join(t.TempDir(), "environment"))

	path, err := toolchain.TinyGo(
		toolchain.TinyGoOptions{Version: "0.42.0", CacheDir: t.TempDir(), Getenv: func(string) string { return environment }},
	)
	if err != nil {
		t.Fatalf("TinyGo() error = %v", err)
	}

	if path != environment {
		t.Errorf("TinyGo() = %q, want %q", path, environment)
	}
}

func TestTinyGoUsesManagedCache(t *testing.T) {
	t.Parallel()
	cacheDir := t.TempDir()
	path := filepath.Join(
		cacheDir,
		"tools",
		"tinygo",
		"0.42.0",
		runtime.GOOS+"-"+runtime.GOARCH,
		"tinygo",
		"bin",
		testExecutableName("tinygo"),
	)
	writeExecutable(t, path)

	got, err := toolchain.TinyGo(toolchain.TinyGoOptions{Version: "0.42.0", CacheDir: cacheDir, Getenv: func(string) string { return "" }})
	if err != nil {
		t.Fatalf("TinyGo() error = %v", err)
	}

	if got != path {
		t.Errorf("TinyGo() = %q, want %q", got, path)
	}
}

func writeExecutable(t *testing.T, path string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("tool"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}

	return path
}
