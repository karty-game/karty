package sdk

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

type cancelResourceFS struct {
	fs.FS

	cancel context.CancelFunc
}

type cancelResourceFile struct {
	fs.File

	cancel context.CancelFunc
}

func (tree cancelResourceFS) Open(name string) (fs.File, error) {
	file, err := tree.FS.Open(name)
	if err != nil {
		return nil, err
	}

	return cancelResourceFile{File: file, cancel: tree.cancel}, nil
}

func (tree cancelResourceFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(tree.FS, name)
}

func (file cancelResourceFile) Read(destination []byte) (int, error) {
	n, err := file.File.Read(destination)
	file.cancel()

	return n, err
}

func TestSDKCanceledResolveAndInstallDoNotCreateCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("KARTY_HOME", home)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := ResolveContext(ctx, "0.0.12"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	if err := InstallBundleContext(ctx, "0.0.12", nil, ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("canceled SDK operation created cache entries: %v %v", entries, err)
	}
}

func TestSDKClientFilesCancellationInterruptsResourceRead(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	resources := cancelResourceFS{FS: fstest.MapFS{"bindings/go/engine/game.go": {Data: make([]byte, 1<<20)}}, cancel: cancel}
	manifest := Manifest{resources: resources}

	files, err := ClientFilesContext(ctx, manifest, "go")
	if !errors.Is(err, context.Canceled) || files != nil {
		t.Fatalf("partial SDK bindings returned: %d %v", len(files), err)
	}
}

func TestSDKDocsCanceledPreparationKeepsLastGoodReference(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	manifest := Manifest{resources: fstest.MapFS{"docs/1/README.md": {Data: []byte("last good")}}}

	manifest.Docs.Version = "1"
	if err := SyncDocsContext(t.Context(), project, manifest); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(project, ".karty", "docs")

	before, err := os.ReadFile(filepath.Join(root, docsManifest))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	manifest.resources = cancelResourceFS{FS: fstest.MapFS{"docs/1/README.md": {Data: make([]byte, 1<<20)}}, cancel: cancel}
	if err := SyncDocsContext(ctx, project, manifest); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	after, err := os.ReadFile(filepath.Join(root, docsManifest))
	if err != nil || string(after) != string(before) {
		t.Fatalf("last good manifest changed: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil || string(contents) != "last good" {
		t.Fatalf("last good reference changed: %v", err)
	}

	entries, err := os.ReadDir(filepath.Dir(root))
	if err != nil || len(entries) != 1 || entries[0].Name() != "docs" {
		t.Fatalf("staging residue: %v %v", entries, err)
	}
}

func TestSDKDocsFailedPublicationRestoresLastGoodReference(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()

	root := filepath.Join(parent, "docs")
	if err := os.Mkdir(root, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("last good"), 0o600); err != nil {
		t.Fatal(err)
	}

	stage := filepath.Join(parent, "missing-stage")
	if err := publishDocs(t.Context(), stage, root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing stage was published: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil || string(contents) != "last good" {
		t.Fatalf("failed swap lost reference: %v", err)
	}

	if _, err := os.Stat(stage + "-previous"); !os.IsNotExist(err) {
		t.Fatalf("successful restore left backup: %v", err)
	}
}
