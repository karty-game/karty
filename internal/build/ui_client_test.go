package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/release"
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

	view, err := uicompiler.Compile("menu.kui", []byte(`<template>
<panel><label>{ title }</label></panel>
</template>

<script setup lang="go">
func setup(title string) {

}
</script>`))
	if err != nil {
		t.Fatal(err)
	}

	stage, cleanup, err := stageClient(root, "example.com/demo", []uicompiler.Component{view}, nil)
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

	repeated, repeatedCleanup, err := stageClient(root, "example.com/demo", []uicompiler.Component{view}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if repeated != stage {
		t.Fatalf("identical inputs changed staging path: %s / %s", stage, repeated)
	}

	if _, _, err := stageClient(root, "example.com/demo", []uicompiler.Component{view}, nil); err == nil {
		t.Fatal("concurrent staging reused an occupied directory")
	}

	data, err = os.ReadFile(filepath.Join(repeated, "main.go"))
	if err != nil || !strings.Contains(string(data), "func main(){}") {
		t.Fatal("occupied stage was modified")
	}

	repeatedCleanup()

	if err := os.Symlink(filepath.Join(src, "main.go"), filepath.Join(src, "link.go")); err != nil {
		t.Fatal(err)
	}

	if _, _, err := stageClient(root, "example.com/demo", []uicompiler.Component{view}, nil); err == nil {
		t.Fatal("accepted source symlink")
	}
}

func TestPersistentUIPackageOwnership(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()

	view, err := uicompiler.Compile("menu.kui", []byte(`<template>
<panel><label>{ title }</label></panel>
</template>

<script setup lang="go">
func setup(title string) {

}
</script>`))
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

func TestSingleFileComponentDiscoveryStagingAndEmbedding(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for name, source := range map[string]string{
		"go.mod":      "module example.com/demo\n",
		"karty.toml":  "[project]\nname='demo'\n[sdk]\nversion='" + release.SDKVersion() + "'\n",
		"src/main.go": "package main\nfunc main(){}\n",
		"ui/views/menu.kui": `<template><panel><label>{ title }</label></panel></template>

<script setup lang="go">
func setup(title string) {}
</script>`,
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	config, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	views, err := project.CompileUI(root, config.Assets.UI, config.Assets.Layouts, "")
	if err != nil || len(views) != 1 {
		t.Fatalf("discovered components: %v", err)
	}

	stage, cleanup, err := stageClient(root, "example.com/demo", views, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	generated, err := os.ReadFile(filepath.Join(stage, "karty_ui_Menu.go"))
	if err != nil || !strings.Contains(string(generated), "title string") {
		t.Fatalf("typed component staging: %s %v", generated, err)
	}

	artifact := filepath.Join(root, "game.kart")
	if err := os.WriteFile(artifact, []byte{0, 'a', 's', 'm', 1, 0, 0, 0}, 0o600); err != nil {
		t.Fatal(err)
	}

	config.Assets.UI[0].Keep = true
	if err := embedUIAssets(root, artifact, config.Assets.UI, config.Assets.Layouts, ""); err != nil {
		t.Fatal(err)
	}

	wasm, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}

	section, err := cartridge.ExtractSection(wasm, ui.SectionName)
	if err != nil {
		t.Fatalf("UI section: %v", err)
	}

	assets, err := cartridge.DecodeAssets(section)
	if err != nil || len(assets) != 1 || assets[0].Name != "ui.menu" {
		t.Fatalf("UI assets: %+v %v", assets, err)
	}

	if _, err := ui.DecodeComposition(assets[0].Bytes); err != nil {
		t.Fatal(err)
	}
}
