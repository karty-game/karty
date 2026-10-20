package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	uicompiler "github.com/karty-game/karty-ui/compiler"
)

func TestReviewedSDKStagesRenamedUISetters(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o700); err != nil {
		t.Fatal(err)
	}

	view, err := uicompiler.Compile(
		"menu.kui",
		[]byte(`<template><panel><label>{title}</label></panel></template><script setup lang="go">func setup(title string) {}</script>`),
	)
	if err != nil {
		t.Fatal(err)
	}

	stage, cleanup, err := stageClient(t.Context(), root, "example.com/game", []uicompiler.Component{view}, nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	source, err := os.ReadFile(filepath.Join(stage, "karty_ui_Menu.go"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(source), ".SetText(") || strings.Contains(string(source), ".Set(") {
		t.Fatalf("wrong SDK setter contract: %s", source)
	}

	if err := writeUIPackage(root, "example.com/game", []uicompiler.Component{view}, 3); err != nil {
		t.Fatal(err)
	}

	source, err = os.ReadFile(filepath.Join(root, ".karty", "ui", "karty_ui_Menu.go"))
	if err != nil || !strings.Contains(string(source), ".SetText(") {
		t.Fatal("importable UI package uses wrong SDK contract", err)
	}
}
