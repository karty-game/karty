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

func TestTinyGoUsesKartyHome(t *testing.T) {
	t.Parallel()

	for _, explicitCache := range []bool{false, true} {
		t.Run(map[bool]string{false: "environment", true: "explicit cache wins"}[explicitCache], func(t *testing.T) {
			t.Parallel()
			environmentHome := t.TempDir()
			cacheDir := ""
			selectedHome := environmentHome

			if explicitCache {
				cacheDir = t.TempDir()
				selectedHome = cacheDir
			}

			want := writeExecutable(t, filepath.Join(
				selectedHome, "tools", "tinygo", "0.42.0", runtime.GOOS+"-"+runtime.GOARCH,
				"tinygo", "bin", testExecutableName("tinygo"),
			))

			got, err := toolchain.TinyGo(toolchain.TinyGoOptions{
				Version: "0.42.0", CacheDir: cacheDir,
				Getenv: func(key string) string {
					if key == "KARTY_HOME" {
						return environmentHome
					}

					return ""
				},
			})
			if err != nil || got != want {
				t.Fatalf("TinyGo() = %q, %v; want %q", got, err, want)
			}
		})
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
