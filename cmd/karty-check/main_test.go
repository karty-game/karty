package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPongSampleDoesNotOwnContributorInstrumentation(t *testing.T) {
	t.Parallel()

	root := filepath.Clean(filepath.Join("..", ".."))
	sample := filepath.Join(root, "samples", "pong")

	sampleRoot, err := os.OpenRoot(sample)
	if err != nil {
		t.Fatal(err)
	}
	defer sampleRoot.Close()

	err = filepath.WalkDir(sample, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() && (entry.Name() == ".karty" || entry.Name() == "dist" || entry.Name() == "tmp") {
			return filepath.SkipDir
		}

		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(sample, path)
		if err != nil {
			return err
		}

		contents, err := sampleRoot.ReadFile(relative)
		if err != nil {
			return err
		}

		if strings.Contains(string(contents), "karty_alloccheck") || strings.Contains(string(contents), "karty_allocations") {
			t.Errorf("sample contains contributor-only allocation instrumentation: %s", path)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
