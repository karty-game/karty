package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/release"
)

func previewProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	project := filepath.Join(root, "samples", "demo")
	writeFixture(t, filepath.Join(project, "karty.toml"), "[sdk]\nversion = '"+release.SDKVersion()+"'\n")
	previewFixture(t, project)

	return root
}

func TestPreviewRefreshesReadingPagesWithoutReplacingGameArtifacts(t *testing.T) {
	t.Parallel()

	root := previewProject(t)
	site := filepath.Join(root, "dist", "samples-site")
	game := filepath.Join(site, "demo", "game.kart")
	writeFixture(t, game, "existing game")
	writeFixture(t, filepath.Join(site, "demo", "index.html"), "existing launcher")
	writeFixture(t, filepath.Join(site, "runtime", "host.wasm"), "existing runtime")

	if err := previewSamples(root); err != nil {
		t.Fatal(err)
	}

	if err := previewSamples(root); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]string{
		"demo/game.kart": "existing game", "demo/index.html": "existing launcher", "runtime/host.wasm": "existing runtime",
	} {
		data, err := os.ReadFile(filepath.Join(site, filepath.FromSlash(path)))
		if err != nil || string(data) != want {
			t.Fatalf("preview changed %s: %q, %v", path, data, err)
		}
	}

	for _, path := range []string{"index.html", "guide/index.html", "guide/guide.md", "demo/preview.jpg"} {
		if err := requireFile(filepath.Join(site, filepath.FromSlash(path))); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPreviewRejectsLinkedOutputDirectoriesAndFiles(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"demo", "index.html"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			root := previewProject(t)
			site := filepath.Join(root, "dist", "samples-site")

			outside := t.TempDir()
			if err := os.MkdirAll(site, 0o755); err != nil {
				t.Fatal(err)
			}

			linkTarget := outside
			if target == "index.html" {
				linkTarget = filepath.Join(outside, "index.html")
				writeFixture(t, linkTarget, "keep me")
			}

			if err := os.Symlink(linkTarget, filepath.Join(site, target)); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}

			if err := previewSamples(root); !errors.Is(err, os.ErrInvalid) {
				t.Fatalf("linked output accepted: %v", err)
			}

			if target == "index.html" {
				data, err := os.ReadFile(linkTarget)
				if err != nil || string(data) != "keep me" {
					t.Fatalf("linked file changed: %q, %v", data, err)
				}
			}
		})
	}
}
