package assetusage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/assetusage"
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
	t.Run("0.0.1", func(t *testing.T) { t.Parallel(); testLocalUIScreenReachability(t, "0.0.1") })
	t.Run("0.0.1", func(t *testing.T) { t.Parallel(); testLocalUIScreenReachability(t, "0.0.1") })
}

func testLocalUIScreenReachability(t *testing.T, version string) {
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

	config = append(config, []byte("\n[[assets.ui]]\nname = \"ui.unused\"\nsource = \"assets/ui/unused.ui\"\n")...)
	//nolint:gosec // Fixed karty.toml inside this test's newly scaffolded temporary project.
	if err := os.WriteFile(path, config, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(directory, "assets/ui/unused.ui"),
		[]byte(`kartui Unused(title string) { <panel><label>Unused</label></panel> }`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	names := []string{"ui.menu", "ui.levels", "ui.loading", "ui.error", "ui.pause", "ui.inventory", "ui.releasing", "ui.hud", "ui.unused"}
	if version == "0.0.1" {
		names = append(names, "ui.item-row")
	}

	result := assetusage.AnalyzeUIViews(directory, "example.com/demo", names, map[string]string{"ShowHud": "ui.hud"})
	if result.KeepAll || result.Live["ui.unused"] != "" || result.Live["ui.hud"] != "" {
		t.Fatalf("unexpected retention: %+v", result)
	}

	for _, name := range names[:7] {
		if result.Live[name] == "" {
			t.Errorf("reachable screen %s stripped: %+v", name, result)
		}
	}

	if version == "0.0.1" && result.Live["ui.item-row"] == "" {
		t.Fatal("nested child stripped")
	}
}

func TestUIComponentUsage(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`new(engine.Game).ShowInventory()`, `show := new(engine.Game).ShowInventory; show()`} {
		directory := usageProject(t, `package main
import "example.com/game/.karty/engine"
func main(){`+body+`}`)

		result := assetusage.AnalyzeUIViews(
			directory,
			"example.com/game",
			[]string{"ui.inventory", "ui.unused"},
			map[string]string{"ShowInventory": "ui.inventory"},
		)
		if result.KeepAll || result.Live["ui.inventory"] == "" || result.Live["ui.unused"] != "" {
			t.Fatalf("component usage: %+v", result)
		}
	}
}
