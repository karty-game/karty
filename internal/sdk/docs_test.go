package sdk_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/sdk"
)

//nolint:wsl_v5 // Arrange/assert groups remain compact in this filesystem test.
func TestSyncDocsWritesPinnedReferenceAndProtectsOwnership(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := sdk.SyncDocs(directory, manifest); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(directory, ".karty", "docs")
	for _, name := range []string{"README.md", "kartui.md", "styles.md", "theme.md", "karty-toml.md", "assets.md"} {
		contents, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatalf("read generated reference %s: %v", name, readErr)
		}
		if strings.Count(string(contents), "DO NOT EDIT")+strings.Count(string(contents), "WARNING") < 2 {
			t.Fatalf("generated reference %s lacks header guards", name)
		}
	}

	path := filepath.Join(root, "styles.md")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, property := range []string{
		"background", "color", "background-image", "padding", "gap",
		"flex-direction", "align-items", "justify-content", "overflow",
		"position", "left", "right", "top", "bottom", "font-size",
		"font-family", "text-align", "image", "image-fit", "icon",
		"icon-position", "icon-size", "icon-gap", "tint", "min-width",
		"min-height", "max-width", "max-height", "flex-grow", "margin",
		"transition-duration", "transition-delay", "transition-easing",
		"transition-enter", "transition-exit",
	} {
		if !strings.Contains(string(contents), property) {
			t.Fatalf("style reference omits %s", property)
		}
	}

	for _, name := range []string{"kartui.md", "theme.md", "karty-toml.md", "assets.md"} {
		contents, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(contents), "not supported") && name == "kartui.md" {
			t.Fatalf("%s must state unsupported syntax behavior", name)
		}
	}

	// The path is constructed entirely beneath t.TempDir.
	if err := os.WriteFile(path, append(contents, []byte("\nuser edit\n")...), 0o600); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	if err := sdk.SyncDocs(directory, manifest); err == nil || !strings.Contains(err.Error(), "was modified") {
		t.Fatalf("modified reference was not rejected: %v", err)
	}
}

//nolint:wsl_v5 // Arrange/assert groups remain compact in this filesystem test.
func TestSyncDocsIsIdempotentForTheCurrentSDK(t *testing.T) {
	t.Parallel()

	latest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := sdk.SyncDocs(directory, latest); err != nil {
		t.Fatal(err)
	}

	if err := sdk.SyncDocs(directory, latest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, ".karty", "docs", "README.md")); err != nil {
		t.Fatalf("current SDK reference is missing: %v", err)
	}
}
