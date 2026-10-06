package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
)

func TestPinSamplePreservesOtherVersionsAndComments(t *testing.T) {
	t.Parallel()

	source := "# Keep authoring choices\n[project]\nname = 'demo'\nversion = '2.0.0'\n[sdk]\nversion = '0.0.5' # pinned\n[assets]\nversion = '3.0.0'\n"

	result, err := pinSample([]byte(source), "0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	expected := strings.Replace(source, "version = '0.0.5'", `version = "0.0.7"`, 1)
	if string(result) != expected {
		t.Fatalf("unexpected rewrite: %s", result)
	}
}

func TestReleaseCheckDetectsAndPreparationRepairsDrift(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	path := filepath.Join(root, "samples/demo/karty.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("[sdk]\nversion = '0.0.5'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := run(root, true); err == nil {
		t.Fatal("drift accepted")
	}

	if err := run(root, false); err != nil {
		t.Fatal(err)
	}

	if err := run(root, true); err != nil {
		t.Fatal(err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(updated), release.SampleSDK) {
		t.Fatal("current SDK not selected")
	}
}

func TestPreparationPinsAllSamplesToCurrentCandidate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, sample := range []string{"world-camera", "pong", "ui-demo", "media-lab"} {
		path := filepath.Join(root, "samples", sample, "karty.toml")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte("[sdk]\nversion = '0.0.5'\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := run(root, false); err != nil {
		t.Fatal(err)
	}

	if err := run(root, true); err != nil {
		t.Fatal(err)
	}

	for _, sample := range []string{"world-camera", "pong", "ui-demo", "media-lab"} {
		data, err := os.ReadFile(filepath.Join(root, "samples", sample, "karty.toml"))
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(data), "0.0.9") {
			t.Fatalf("%s has the wrong SDK: %s", sample, data)
		}
	}
}
