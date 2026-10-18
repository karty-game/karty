package assetusage_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/assetusage"
	"github.com/karty-game/karty/internal/packages"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
)

func TestResolvedPackageUIReachabilityKeepsChildrenAndStripsUnusedExports(t *testing.T) {
	t.Parallel()

	version := release.SDKVersion()

	manifest, err := sdk.Resolve(version)
	if err != nil {
		t.Fatal(err)
	}

	game := filepath.Join(t.TempDir(), "game")
	if err := scaffold.CreateTemplate(game, "demo", manifest, "ui"); err != nil {
		t.Fatal(err)
	}

	kit := filepath.Join(t.TempDir(), "kit")
	if err := os.CopyFS(kit, os.DirFS("../../testdata/package-ui/library")); err != nil {
		t.Fatal(err)
	}

	name := filepath.Join(kit, packages.ManifestName)

	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}

	kitRoot, err := os.OpenRoot(kit)
	if err != nil {
		t.Fatal(err)
	}
	defer kitRoot.Close()

	if err := kitRoot.WriteFile(packages.ManifestName, bytes.ReplaceAll(data, []byte("0.0.13"), []byte(version)), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := packages.Import(t.Context(), game, kit, false); err != nil {
		t.Fatal(err)
	}

	data, err = os.ReadFile("../../testdata/package-ui/game/game-screen.kui")
	if err != nil {
		t.Fatal(err)
	}

	gameRoot, err := os.OpenRoot(game)
	if err != nil {
		t.Fatal(err)
	}
	defer gameRoot.Close()

	if err := packages.MakeDirectories(gameRoot, "ui"); err != nil {
		t.Fatal(err)
	}

	if err := gameRoot.WriteFile("ui/game-screen.kui", data, 0600); err != nil {
		t.Fatal(err)
	}

	if err := gameRoot.WriteFile(
		"src/main.go",
		[]byte("package main\nimport ui \"example.com/demo/.karty/ui\"\nfunc main(){ _ = ui.GameScreen }\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	config, err := project.Load(game)
	if err != nil {
		t.Fatal(err)
	}

	set, err := packages.Restore(t.Context(), game)
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()

	assets, err := packages.ResolveUI(t.Context(), game, config.Assets, set)
	if err != nil {
		t.Fatal(err)
	}

	views, err := project.CompileUI(game, assets.UI, assets.Layouts, assets.Theme.Source)
	if err != nil {
		t.Fatal(err)
	}

	result := assetusage.AnalyzeUIWithComponents(
		game,
		"example.com/demo",
		[]string{"ui.game-screen", "hud-kit.badge", "hud-kit.unused"},
		views,
	)
	if result.KeepAll || result.Live["ui.game-screen"] == "" || result.Live["hud-kit.badge"] == "" || result.Live["hud-kit.unused"] != "" {
		t.Fatalf("incorrect package reachability: %+v", result)
	}

	if result := assetusage.AnalyzeWithUI(game, "example.com/demo", []string{"hud-kit.panel"}, views); result.KeepAll {
		t.Fatalf("resolved UI broke texture analysis: %+v", result)
	}
}
