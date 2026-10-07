package assetusage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/assetusage"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
)

func TestUIUsage(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, body string
		keepAll    bool
	}{
		{"typed", `new(engine.Game).ShowUI(engine.UIInventory)`, false},
		{"literal", `new(engine.Game).ShowUI("ui.inventory")`, false},
		{"dynamic", `name:=engine.UIInventory;new(engine.Game).ShowUI(name)`, true},
		{"method escape", `show:=new(engine.Game).ShowUI;show("ui.inventory")`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			directory := usageProject(t, `package main
import "example.com/game/.karty/engine"
func main(){`+test.body+`}`)

			result := assetusage.AnalyzeUI(directory, "example.com/game", []string{"ui.inventory", "ui.unused"})
			if result.KeepAll != test.keepAll {
				t.Fatalf("usage: %+v", result)
			}

			if !test.keepAll && (result.Live["ui.inventory"] == "" || result.Live["ui.unused"] != "") {
				t.Fatalf("incorrect live set: %+v", result)
			}
		})
	}
}

func TestLocalUIScreenReachability(t *testing.T) {
	t.Parallel()

	testLocalUIScreenReachability(t, release.SDKVersion(), ".kui")
}

func testLocalUIScreenReachability(t *testing.T, version, extension string) {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "demo")

	manifest, err := sdk.Resolve(version)
	if err != nil {
		t.Fatal(err)
	}

	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(directory, "karty.toml")

	config, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	config = append(config, []byte("\n[[assets.ui]]\nname = \"ui.unused\"\nsource = \"assets/ui/unused"+extension+"\"\n")...)
	//nolint:gosec // Fixed karty.toml inside this test's newly scaffolded temporary project.
	if err := os.WriteFile(path, config, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(directory, "assets/ui/unused"+extension),
		[]byte(`<template>
<panel><label>Unused</label></panel>
</template>

<script setup lang="go">
func setup(title string) {

}
</script>`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	names := []string{
		"ui.menu", "ui.levels", "ui.loading", "ui.error", "ui.pause", "ui.inventory", "ui.releasing",
		"ui.hud", "ui.unused", "ui.item-row",
	}

	result := assetusage.AnalyzeUI(directory, "example.com/demo", names)
	if result.KeepAll || result.Live["ui.unused"] != "" || result.Live["ui.hud"] != "" {
		t.Fatalf("unexpected retention: %+v", result)
	}

	for _, name := range names[:7] {
		if result.Live[name] == "" {
			t.Errorf("reachable screen %s stripped: %+v", name, result)
		}
	}

	if result.Live["ui.item-row"] == "" {
		t.Fatal("nested child stripped")
	}
}
