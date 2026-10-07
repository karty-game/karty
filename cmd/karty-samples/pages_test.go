package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func sampleFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()

	state, source := filepath.Join(root, "state"), filepath.Join(root, "input")
	if err := os.Mkdir(state, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"index.html", "ui-demo/index.html", "media-lab/index.html"} {
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

	writeFixture(t, filepath.Join(source, "ui-demo", "index.html"), "updated")
	writeFixture(t, filepath.Join(source, "new-demo", "index.html"), "new sample")

	if err := assemble(state, source, "pr/1", false, nil); err != nil {
		t.Fatal(err)
	}

	for destination, expected := range map[string]string{"main": "sample", "pr/2": "sample", "pr/1": "updated"} {
		data, err := os.ReadFile(filepath.Join(state, filepath.FromSlash(destination), "ui-demo", "index.html"))
		if err != nil || string(data) != expected {
			t.Fatalf("%s preview changed: %s, %v", destination, data, err)
		}
	}

	if err := requireFile(filepath.Join(state, "pr", "1", "new-demo", "index.html")); err != nil {
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

	for _, invalid := range []string{"symlink", "metadata", "unexpected", "missing", "incomplete", "oversize"} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()

			state, source := sampleFixture(t)
			if err := assemble(state, source, "pr/1", false, nil); err != nil {
				t.Fatal(err)
			}

			switch invalid {
			case "symlink":
				if err := os.Symlink(filepath.Join(state, "index.html"), filepath.Join(source, "ui-demo", "leak")); err != nil {
					t.Fatal(err)
				}
			case "metadata":
				writeFixture(t, filepath.Join(source, "ui-demo", ".git", "config"), "metadata")
			case "unexpected":
				writeFixture(t, filepath.Join(source, "CNAME"), "unexpected.example")
			case "missing":
				if err := os.Remove(filepath.Join(source, "ui-demo", "index.html")); err != nil {
					t.Fatal(err)
				}
			case "incomplete":
				writeFixture(t, filepath.Join(source, "new-demo", "fixture.wasm"), "incomplete")
			case "oversize":
				path := filepath.Join(source, "ui-demo", "fixture.wasm")
				writeFixture(t, path, "")

				if err := os.Truncate(path, maxSampleBytes+1); err != nil {
					t.Fatal(err)
				}
			}

			if err := assemble(state, source, "pr/1", false, nil); err == nil {
				t.Fatal("invalid artifact accepted")
			}

			if err := requireFile(filepath.Join(state, "pr", "1", "ui-demo", "index.html")); err != nil {
				t.Fatal("working preview replaced before validation", err)
			}
		})
	}
}

func TestSampleDiscoveryIncludesNewProjects(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, name := range []string{"first-demo", "new-demo"} {
		writeFixture(t, filepath.Join(root, "samples", name, "karty.toml"), "[sdk]\nversion = '1.0.0'\n")
	}

	writeFixture(t, filepath.Join(root, "samples", "README.md"), "Sample documentation")
	writeFixture(t, filepath.Join(root, "samples", "artwork", "texture.png"), "Artwork without a project")

	names, err := sampleNames(root)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(names, []string{"first-demo", "new-demo"}) {
		t.Fatalf("unexpected sample projects: %v", names)
	}
}

func TestSampleDiscoveryRejectsInvalidProjects(t *testing.T) {
	t.Parallel()

	for _, invalid := range []string{"empty", "name", "symlink"} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeFixture(t, filepath.Join(root, "samples", "README.md"), "Sample documentation")

			switch invalid {
			case "name":
				writeFixture(t, filepath.Join(root, "samples", "bad name", "karty.toml"), "[sdk]\nversion = '1.0.0'\n")
			case "symlink":
				config := filepath.Join(root, "samples", "new-demo", "karty.toml")
				writeFixture(t, config, "[sdk]\nversion = '1.0.0'\n")

				if err := os.Rename(config, filepath.Join(root, "external.toml")); err != nil {
					t.Fatal(err)
				}

				if err := os.Symlink(filepath.Join(root, "external.toml"), config); err != nil {
					t.Fatal(err)
				}
			}

			if _, err := sampleNames(root); err == nil {
				t.Fatal("invalid project discovery accepted")
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
