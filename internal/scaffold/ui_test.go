package scaffold_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
)

func TestUITemplate(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"src/main.go", "assets/ui/menu.ui", "assets/ui/hud.ui", "levels/first/level.toml", ".karty/engine/game.go", ".karty/engine/ui-views.go"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}

	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err == nil {
		t.Fatal("overwrote project")
	}
}

func TestCompositionUITemplate(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(directory, "assets/ui/item-row.ui")); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"Menu", "Inventory", "ItemRow"} {
		if _, err := os.Stat(filepath.Join(directory, ".karty/ui/karty_ui_"+name+".go")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStyledUITemplate(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"assets/ui/theme.toml", "assets/ui/menu.ui", ".karty/ui/karty_ui_Menu.go"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}

	menu, err := os.ReadFile(filepath.Join(directory, "assets/ui/menu.ui"))
	if err != nil || !strings.Contains(string(menu), "style {") {
		t.Fatal("styled starter missing style block")
	}
}

func TestImageThemeSDKUsesCompatibleStyledStarter(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	config, err := os.ReadFile(filepath.Join(directory, "karty.toml"))
	if err != nil || !strings.Contains(string(config), `version = "0.0.1"`) {
		t.Fatalf("SDK 0.6 starter config = %s, error = %v", config, err)
	}

	if _, err := os.Stat(filepath.Join(directory, ".karty/ui/karty_ui_Menu.go")); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutSDKUsesCompatibleStarter(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	config, err := os.ReadFile(filepath.Join(directory, "karty.toml"))
	if err != nil || !strings.Contains(string(config), `version = "0.0.1"`) {
		t.Fatalf("SDK 0.7 starter config = %s, error = %v", config, err)
	}

	if _, err := os.Stat(filepath.Join(directory, ".karty/ui/karty_ui_Menu.go")); err != nil {
		t.Fatal(err)
	}
}

func TestRecentPresentationSDKsUseCompatibleStarter(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"0.0.1", "0.0.1", "0.0.1"} {
		manifest, err := sdk.Resolve(version)
		if err != nil {
			t.Fatal(err)
		}

		directory := filepath.Join(t.TempDir(), "demo")
		if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
			t.Fatal(err)
		}

		config, err := os.ReadFile(filepath.Join(directory, "karty.toml"))
		if err != nil || !strings.Contains(string(config), `version = "`+version+`"`) {
			t.Fatalf("SDK %s starter config = %s, error = %v", version, config, err)
		}

		if _, err := os.Stat(filepath.Join(directory, ".karty/ui/karty_ui_Menu.go")); err != nil {
			t.Fatal(err)
		}

		docsPath := filepath.Join(directory, ".karty", "docs", "README.md")

		_, docsErr := os.Stat(docsPath)
		if version == "0.0.1" && docsErr != nil {
			t.Fatalf("SDK %s reference missing: %v", version, docsErr)
		}

		if version != "0.0.1" && !os.IsNotExist(docsErr) {
			t.Fatalf("SDK %s unexpectedly wrote latest reference: %v", version, docsErr)
		}
	}
}
