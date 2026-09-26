package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-ui/compiler"
)

func TestUIClientStagingOwnership(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	src := filepath.Join(root, "src")
	if err := os.Mkdir(src, 0o750); err != nil {
		t.Fatal(err)
	}

	original := []byte("package main\nfunc main(){}\n")
	if err := os.WriteFile(filepath.Join(src, "main.go"), original, 0o600); err != nil {
		t.Fatal(err)
	}

	view, err := uicompiler.Compile("menu.ui", []byte(`kartui Menu(title string) { <panel><label>{ title }</label></panel> }`))
	if err != nil {
		t.Fatal(err)
	}

	stage, cleanup, err := stageUIClient(root, "example.com/demo", []uicompiler.Component{view})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	generated, err := os.ReadFile(filepath.Join(stage, "karty_ui_Menu.go"))
	if err != nil || !strings.Contains(string(generated), "DO NOT EDIT") {
		t.Fatalf("generated source: %s %v", generated, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil || len(entries) != 1 {
		t.Fatalf("user src was changed: %v %v", entries, err)
	}

	data, err := os.ReadFile(filepath.Join(src, "main.go"))
	if err != nil || string(data) != string(original) {
		t.Fatal("user source changed")
	}

	cleanup()

	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatal("private staging directory survived cleanup")
	}

	if err := os.Symlink(filepath.Join(src, "main.go"), filepath.Join(src, "link.go")); err != nil {
		t.Fatal(err)
	}

	if _, _, err := stageUIClient(root, "example.com/demo", []uicompiler.Component{view}); err == nil {
		t.Fatal("accepted source symlink")
	}
}

func TestPersistentUIPackageOwnership(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()

	view, err := uicompiler.Compile("menu.ui", []byte(`kartui Menu(title string) { <panel><label>{ title }</label></panel> }`))
	if err != nil {
		t.Fatal(err)
	}

	views := []uicompiler.Component{view}
	if err := writeUIPackage(directory, "example.com/demo", views); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(directory, ".karty", "ui", "karty_ui_Menu.go")

	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "package ui") || strings.Count(string(data), "WARNING:") < 2 {
		t.Fatalf("generated package: %s %v", data, err)
	}

	if err := writeUIPackage(directory, "example.com/demo", nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("stale generated file retained")
	}

	if err := os.WriteFile(path, []byte("package ui // user owned\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := writeUIPackage(directory, "example.com/demo", views); err == nil {
		t.Fatal("overwrote user source")
	}

	data, _ = os.ReadFile(path)
	if string(data) != "package ui // user owned\n" {
		t.Fatal("user source changed")
	}
}

func TestPersistentUIPackageRejectsSymlinks(t *testing.T) {
	t.Parallel()

	for _, relative := range []string{".karty", ".karty/ui", ".karty/ui/karty_ui_Menu.go"} {
		t.Run(relative, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()

			path := filepath.Join(directory, relative)
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}

			if err := os.Symlink(t.TempDir(), path); err != nil {
				t.Fatal(err)
			}

			if err := writeUIPackage(directory, "example.com/demo", nil); err == nil {
				t.Fatal("accepted symlink")
			}
		})
	}
}
