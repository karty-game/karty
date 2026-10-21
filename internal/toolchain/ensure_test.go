package toolchain_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

func TestEnsureRejectsMissingPlatformArtifactBeforeDownload(t *testing.T) {
	// This checks managed artifact resolution. A contributor's local override
	// intentionally bypasses that path and must not change the test's inputs.
	t.Setenv("KARTY_TINYGO", "")

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	manifest.Artifacts.TinyGo = nil

	_, err = toolchain.Ensure(context.Background(), manifest, toolchain.EnsureOptions{NeedTinyGo: true})
	if err == nil {
		t.Fatal("Ensure() error = nil, want missing platform artifact")
	}

	want := "TinyGo for " + runtime.GOOS + "-" + runtime.GOARCH
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Ensure() error = %q, want %q", err, want)
	}
}

func TestEnsureTinyGoUsesEnvironmentBeforeManagedInstall(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("KARTY_HOME", cacheDir)
	environment := writeExecutable(t, filepath.Join(t.TempDir(), testExecutableName("tinygo")))
	t.Setenv("KARTY_TINYGO", environment)

	// No managed artifacts are available: resolving these would fail before
	// reaching the local executable, even though no download is needed.
	manifest := sdk.Manifest{}

	paths, err := toolchain.Ensure(t.Context(), manifest, toolchain.EnsureOptions{NeedTinyGo: true})
	if err != nil || paths.TinyGo != environment {
		t.Fatalf("Ensure() = %+v, %v; want TinyGo %q", paths, err, environment)
	}

	override := writeExecutable(t, filepath.Join(t.TempDir(), testExecutableName("override")))

	paths, err = toolchain.Ensure(t.Context(), manifest, toolchain.EnsureOptions{NeedTinyGo: true, TinyGoOverride: override})
	if err != nil || paths.TinyGo != override {
		t.Fatalf("Ensure() explicit override = %+v, %v; want TinyGo %q", paths, err, override)
	}

	t.Setenv("KARTY_TINYGO", filepath.Join(t.TempDir(), "missing"))

	if _, err := toolchain.Ensure(
		t.Context(),
		manifest,
		toolchain.EnsureOptions{NeedTinyGo: true},
	); err == nil ||
		!strings.Contains(err.Error(), "KARTY_TINYGO") {
		t.Fatalf("Ensure() invalid environment path error = %v", err)
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("Ensure() modified managed cache for an override: %v, %v", entries, err)
	}
}
