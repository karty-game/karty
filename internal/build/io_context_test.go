package build

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/levelbuild"
)

type cancelCopyReader struct {
	cancel context.CancelFunc
	reads  int
}

func TestFileHashConfinesArtifactReads(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	path := filepath.Join(directory, "karty.js")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	if hash, err := fileHash(t.Context(), path); err != nil || hash != "2cf24dba5fb0a30e" {
		t.Fatalf("regular artifact hash = %q, %v", hash, err)
	}

	escape := filepath.Join(t.TempDir(), "escape.js")
	if err := os.Symlink(path, escape); err != nil {
		t.Fatal(err)
	}

	if _, err := fileHash(t.Context(), escape); err == nil {
		t.Fatal("artifact hashing followed a symlink outside its directory")
	}
}

func (reader *cancelCopyReader) Read(destination []byte) (int, error) {
	reader.reads++
	n := copy(destination, "partial replacement bytes")

	reader.cancel()

	return n, nil
}

func TestCanceledCopyKeepsDestinationAndRemovesPartialFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	destination := filepath.Join(root, "karty-host.wasm")
	if err := os.WriteFile(destination, []byte("previous host"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	input := &cancelCopyReader{cancel: cancel}
	if err := copyToFile(ctx, input, destination, 0o600); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled copy returned %v", err)
	}

	contents, err := os.ReadFile(destination)
	if err != nil || string(contents) != "previous host" || input.reads != 1 {
		t.Fatalf("canceled copy changed destination or kept reading: %q, reads=%d, %v", contents, input.reads, err)
	}

	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "karty-host.wasm" {
		t.Fatalf("partial copy was not removed: %v, %v", entries, err)
	}

	if err := copyToFile(t.Context(), strings.NewReader("completed host"), destination, 0o600); err != nil {
		t.Fatal(err)
	}

	contents, err = os.ReadFile(destination)
	if err != nil || string(contents) != "completed host" {
		t.Fatalf("completed copy was not published: %q, %v", contents, err)
	}
}

func TestCanceledLevelStagingKeepsPublishedContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	content := filepath.Join(root, "content")
	if err := os.Mkdir(content, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(content, "old.kld"), []byte("previous level"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := stageLevels(ctx, root, []levelbuild.Artifact{{Name: "replacement"}}, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled level staging returned %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "content" {
		t.Fatalf("canceled staging created additional outputs: %v, %v", entries, err)
	}

	contents, err := os.ReadFile(filepath.Join(content, "old.kld"))
	if err != nil || string(contents) != "previous level" {
		t.Fatalf("canceled staging changed published content: %q, %v", contents, err)
	}
}
