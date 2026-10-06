package scaffold_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
)

func TestUITemplate(t *testing.T) {
	t.Parallel()

	if os.Getenv("KARTY_TEST_SDK") != release.SampleSDK {
		t.Skip("requires candidate SFC templates; set KARTY_TEST_SDK=0.0.9")
	}

	manifest, err := sdk.Resolve(release.SampleSDK)
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"src/main.go", "assets/ui/menu.kui", "assets/ui/hud.kui", "levels/first/level.toml", ".karty/engine/game.go", ".karty/assets/ui.go"} {
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

	if os.Getenv("KARTY_TEST_SDK") != release.SampleSDK {
		t.Skip("requires candidate SFC templates; set KARTY_TEST_SDK=0.0.9")
	}

	manifest, err := sdk.Resolve(release.SampleSDK)
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(directory, "assets/ui/item-row.kui")); err != nil {
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

	if os.Getenv("KARTY_TEST_SDK") != release.SampleSDK {
		t.Skip("requires candidate SFC templates; set KARTY_TEST_SDK=0.0.9")
	}

	manifest, err := sdk.Resolve(release.SampleSDK)
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"assets/ui/theme.toml", "assets/ui/menu.kui", ".karty/ui/karty_ui_Menu.go"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}

	menu, err := os.ReadFile(filepath.Join(directory, "assets/ui/menu.kui"))
	if err != nil || !strings.Contains(string(menu), "<style>") {
		t.Fatal("styled starter missing style block")
	}
}

func TestImageThemeSDKUsesCompatibleStyledStarter(t *testing.T) {
	t.Parallel()

	if os.Getenv("KARTY_TEST_SDK") != release.SampleSDK {
		t.Skip("requires candidate SFC templates; set KARTY_TEST_SDK=0.0.9")
	}

	manifest, err := sdk.Resolve(release.SampleSDK)
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	config, err := os.ReadFile(filepath.Join(directory, "karty.toml"))
	if err != nil || !strings.Contains(string(config), `version = "`+release.SampleSDK+`"`) {
		t.Fatalf("SDK 0.6 starter config = %s, error = %v", config, err)
	}

	if _, err := os.Stat(filepath.Join(directory, ".karty/ui/karty_ui_Menu.go")); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutSDKUsesCompatibleStarter(t *testing.T) {
	t.Parallel()

	if os.Getenv("KARTY_TEST_SDK") != release.SampleSDK {
		t.Skip("requires candidate SFC templates; set KARTY_TEST_SDK=0.0.9")
	}

	manifest, err := sdk.Resolve(release.SampleSDK)
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(t.TempDir(), "demo")
	if err := scaffold.CreateTemplate(directory, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	config, err := os.ReadFile(filepath.Join(directory, "karty.toml"))
	if err != nil || !strings.Contains(string(config), `version = "`+release.SampleSDK+`"`) {
		t.Fatalf("SDK 0.7 starter config = %s, error = %v", config, err)
	}

	if _, err := os.Stat(filepath.Join(directory, ".karty/ui/karty_ui_Menu.go")); err != nil {
		t.Fatal(err)
	}
}

func TestRecentPresentationSDKsUseCompatibleStarter(t *testing.T) {
	t.Parallel()

	if os.Getenv("KARTY_TEST_SDK") != release.SampleSDK {
		t.Skip("requires candidate SFC templates; set KARTY_TEST_SDK=0.0.9")
	}

	for _, version := range []string{release.SampleSDK} {
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
		if version == release.SampleSDK && docsErr != nil {
			t.Fatalf("SDK %s reference missing: %v", version, docsErr)
		}

		if version != release.SampleSDK && !os.IsNotExist(docsErr) {
			t.Fatalf("SDK %s unexpectedly wrote latest reference: %v", version, docsErr)
		}
	}
}
