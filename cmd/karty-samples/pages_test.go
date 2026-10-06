package main

import (
	"os"
	"path/filepath"
	"testing"
)

func sampleFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()

	state, source := filepath.Join(root, "state"), filepath.Join(root, "input")
	if err := os.Mkdir(state, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"index.html", "pong/index.html", "ui-demo/index.html", "media-lab/index.html"} {
		writeFixture(t, filepath.Join(source, filepath.FromSlash(name)), "sample")
	}

	return state, source
}

func writeFixture(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAssembleIsolatesPreviewsAndRemovesClosedPRs(t *testing.T) {
	t.Parallel()

	state, source := sampleFixture(t)
	for _, destination := range []string{"main", "pr/1", "pr/2"} {
		if err := assemble(state, source, destination, false, nil); err != nil {
			t.Fatal(err)
		}
	}

	writeFixture(t, filepath.Join(source, "pong", "index.html"), "updated")
	writeFixture(t, filepath.Join(source, "world-camera", "index.html"), "camera")

	if err := assemble(state, source, "pr/1", false, nil); err != nil {
		t.Fatal(err)
	}

	for destination, expected := range map[string]string{"main": "sample", "pr/2": "sample", "pr/1": "updated"} {
		data, err := os.ReadFile(filepath.Join(state, filepath.FromSlash(destination), "pong", "index.html"))
		if err != nil || string(data) != expected {
			t.Fatalf("%s preview changed: %s, %v", destination, data, err)
		}
	}

	if err := requireFile(filepath.Join(state, "pr", "1", "world-camera", "index.html")); err != nil {
		t.Fatal(err)
	}

	if err := assemble(state, source, "pr/1", true, []int{}); err != nil {
		t.Fatal(err)
	}

	for _, number := range []string{"1", "2"} {
		if _, err := os.Stat(filepath.Join(state, "pr", number)); !os.IsNotExist(err) {
			t.Fatalf("closed preview %s survived: %v", number, err)
		}
	}

	if err := requireFile(filepath.Join(state, "main", "index.html")); err != nil {
		t.Fatal("main preview removed", err)
	}
}

func TestAssembleRejectsArtifactsBeforeReplacingPreview(t *testing.T) {
	t.Parallel()

	for _, invalid := range []string{"symlink", "metadata", "unexpected", "missing", "camera", "oversize"} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()

			state, source := sampleFixture(t)
			if err := assemble(state, source, "pr/1", false, nil); err != nil {
				t.Fatal(err)
			}

			switch invalid {
			case "symlink":
				if err := os.Symlink(filepath.Join(state, "index.html"), filepath.Join(source, "pong", "leak")); err != nil {
					t.Fatal(err)
				}
			case "metadata":
				writeFixture(t, filepath.Join(source, "pong", ".git", "config"), "metadata")
			case "unexpected":
				writeFixture(t, filepath.Join(source, "CNAME"), "unexpected.example")
			case "missing":
				if err := os.Remove(filepath.Join(source, "pong", "index.html")); err != nil {
					t.Fatal(err)
				}
			case "camera":
				writeFixture(t, filepath.Join(source, "world-camera", "fixture.wasm"), "incomplete")
			case "oversize":
				path := filepath.Join(source, "pong", "fixture.wasm")
				writeFixture(t, path, "")

				if err := os.Truncate(path, maxSampleBytes+1); err != nil {
					t.Fatal(err)
				}
			}

			if err := assemble(state, source, "pr/1", false, nil); err == nil {
				t.Fatal("invalid artifact accepted")
			}

			if err := requireFile(filepath.Join(state, "pr", "1", "pong", "index.html")); err != nil {
				t.Fatal("working preview replaced before validation", err)
			}
		})
	}
}

func TestAssembleRejectsDestinationEscapes(t *testing.T) {
	t.Parallel()

	state, source := sampleFixture(t)
	for _, destination := range []string{"../escape", "pr/../../main", "/tmp/escape", "pr/0", "pr/01", "pr/1\\escape"} {
		if err := assemble(state, source, destination, false, nil); err == nil {
			t.Fatalf("destination escape accepted: %s", destination)
		}
	}

	if err := assemble(state, source, "main", true, nil); err == nil {
		t.Fatal("main removal accepted")
	}

	if err := os.Symlink(t.TempDir(), filepath.Join(state, "pr")); err != nil {
		t.Fatal(err)
	}

	if err := assemble(state, source, "pr/1", false, nil); err == nil {
		t.Fatal("symlinked preview parent accepted")
	}
}

func TestSampleVersionUsesExactPublicPin(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"0.0.1", "0.0.8", "../escape", "", "latest"} {
		directory := t.TempDir()
		writeFixture(t, filepath.Join(directory, "karty.toml"), "[sdk]\nversion = \""+version+"\"\n")
		got, err := sampleVersion(directory)

		valid := version == "0.0.1" || version == "0.0.8"
		if (err == nil) != valid || (valid && got != version) {
			t.Fatalf("SDK pin %q: %q, %v", version, got, err)
		}
	}
}
