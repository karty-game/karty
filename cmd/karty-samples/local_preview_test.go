package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func localPreviewProject(t *testing.T, built bool) string {
	t.Helper()

	root := previewProject(t)
	project := filepath.Join(root, "samples", "crate-room")
	writeFixture(t, filepath.Join(project, "karty.toml"), "[sdk]\nversion = '0.0.12'\n")
	previewFixture(t, project)
	writeFixture(t, filepath.Join(project, "preview.toml"),
		"title = 'Physics lab: crate room'\ndescription = 'Shoot and push crates.'\nalt = 'Crates and ramps.'\n")

	if built {
		for _, name := range []string{"index.html", "game.kart", "karty.js", "karty-host.wasm", "wasm_exec.js", "content/room.kld"} {
			writeFixture(t, filepath.Join(project, "dist", "web", filepath.FromSlash(name)), "built "+name)
		}
	}

	return root
}

func TestLocalDocsIncludePhysicsScreenshotAndCompletePlayableSnapshot(t *testing.T) {
	t.Parallel()

	root := localPreviewProject(t, true)
	site := filepath.Join(root, "dist", "samples-site")
	writeFixture(t, filepath.Join(site, "demo", "game.kart"), "preserved public demo")
	writeFixture(t, filepath.Join(site, "runtime", "host.wasm"), "preserved shared runtime")
	writeFixture(t, filepath.Join(site, "crate-room", "content", "stale.kld"), "previous content")

	if err := previewSamples(root); err != nil {
		t.Fatal(err)
	}

	page, err := os.ReadFile(filepath.Join(site, "index.html"))
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"Physics lab: crate room", `href="crate-room/"`, `src="crate-room/preview.jpg"`} {
		if !strings.Contains(string(page), want) {
			t.Fatalf("local gallery missing %q", want)
		}
	}

	for _, name := range []string{"index.html", "game.kart", "karty.js", "karty-host.wasm", "wasm_exec.js", "content/room.kld"} {
		actual, err := os.ReadFile(filepath.Join(site, "crate-room", filepath.FromSlash(name)))
		if err != nil || string(actual) != "built "+name {
			t.Fatalf("incomplete local snapshot %s: %q, %v", name, actual, err)
		}
	}

	if _, err := os.Stat(filepath.Join(site, "crate-room", "content", "stale.kld")); !os.IsNotExist(err) {
		t.Fatalf("stale content survived snapshot replacement: %v", err)
	}

	original, err := os.ReadFile(filepath.Join(root, "samples", "crate-room", "preview.jpg"))
	if err != nil {
		t.Fatal(err)
	}

	actual, err := os.ReadFile(filepath.Join(site, "crate-room", "preview.jpg"))
	if err != nil || !bytes.Equal(actual, original) {
		t.Fatalf("local screenshot differs: %v", err)
	}

	writeFixture(t, filepath.Join(root, "samples", "crate-room", "dist", "web", "game.kart"), "updated local game")

	if err := previewSamples(root); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]string{
		"crate-room/game.kart": "updated local game",
		"demo/game.kart":       "preserved public demo",
		"runtime/host.wasm":    "preserved shared runtime",
	} {
		contents, err := os.ReadFile(filepath.Join(site, filepath.FromSlash(path)))
		if err != nil || string(contents) != want {
			t.Fatalf("preview refresh %s: %q, %v", path, contents, err)
		}
	}

	names, err := sampleNames(root)
	if err != nil || !slices.Equal(names, []string{"crate-room", "demo"}) {
		t.Fatalf("sample missing from published build list: %v, %v", names, err)
	}
}

func TestLocalDocsWithoutCandidateBuildShowSetupAndScreenshot(t *testing.T) {
	t.Parallel()

	root := localPreviewProject(t, false)
	if err := previewSamples(root); err != nil {
		t.Fatal(err)
	}

	page, err := os.ReadFile(filepath.Join(root, "dist", "samples-site", "crate-room", "index.html"))
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"Physics lab: crate room", "Build locally to play", "candidate SDK 0.0.12", `src="preview.jpg"`} {
		if !strings.Contains(string(page), want) {
			t.Fatalf("unbuilt local sample missing %q", want)
		}
	}

	if err := requireFile(filepath.Join(root, "dist", "samples-site", "crate-room", "preview.jpg")); err != nil {
		t.Fatal(err)
	}
}

func TestLocalDocsReplaceHangarsReleasedBuildWithCandidateSnapshot(t *testing.T) {
	t.Parallel()

	root := localPreviewProject(t, true)
	if err := os.Rename(filepath.Join(root, "samples", "crate-room"), filepath.Join(root, "samples", "hangar")); err != nil {
		t.Fatal(err)
	}

	writeFixture(t, filepath.Join(root, "dist", "samples-site", "hangar", "game.kart"), "previous released Hangar")

	if err := previewSamples(root); err != nil {
		t.Fatal(err)
	}

	actual, err := os.ReadFile(filepath.Join(root, "dist", "samples-site", "hangar", "game.kart"))
	if err != nil || string(actual) != "built game.kart" {
		t.Fatalf("local docs retained old static Hangar: %q, %v", actual, err)
	}
}

func TestLocalDocsRejectUnsafeOrPartialBuildAndPreservePreviousPreview(t *testing.T) {
	t.Parallel()

	for _, invalid := range []string{"missing host", "linked source", "linked content", "linked output"} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()

			root := localPreviewProject(t, true)
			previous := invalidateLocalPreview(t, root, invalid)

			if err := previewSamples(root); err == nil {
				t.Fatal("unsafe or partial local sample accepted")
			}

			contents, err := os.ReadFile(previous)
			if err != nil || string(contents) != "previous playable preview" {
				t.Fatalf("invalid build changed previous preview: %q, %v", contents, err)
			}

			stages, err := filepath.Glob(filepath.Join(root, "dist", ".docs-sample-*"))
			if err != nil || len(stages) != 0 {
				t.Fatalf("failed preview left staging directories: %v, %v", stages, err)
			}
		})
	}
}

func invalidateLocalPreview(t *testing.T, root, invalid string) string {
	t.Helper()

	source := filepath.Join(root, "samples", "crate-room", "dist", "web")
	output := filepath.Join(root, "dist", "samples-site", "crate-room")
	outside := t.TempDir()
	previous := filepath.Join(output, "index.html")
	writeFixture(t, previous, "previous playable preview")

	switch invalid {
	case "missing host":
		if err := os.Remove(filepath.Join(source, "karty-host.wasm")); err != nil {
			t.Fatal(err)
		}
	case "linked source":
		if err := os.RemoveAll(source); err != nil {
			t.Fatal(err)
		}

		if err := os.Symlink(outside, source); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
	case "linked content":
		if err := os.Symlink(outside, filepath.Join(source, "content", "outside")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
	case "linked output":
		if err := os.RemoveAll(output); err != nil {
			t.Fatal(err)
		}

		previous = filepath.Join(outside, "index.html")
		writeFixture(t, previous, "previous playable preview")

		if err := os.Symlink(outside, output); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
	}

	return previous
}
