package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/build"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
)

func runUI(ctx context.Context) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("repository root: %w", err)
	}

	temporary, err := os.MkdirTemp("", "karty-check-ui-")
	if err != nil {
		return fmt.Errorf("UI fixture: %w", err)
	}
	defer os.RemoveAll(temporary)

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		return err
	}

	project := filepath.Join(temporary, "ui-demo")
	if err := scaffold.CreateTemplate(project, "ui-demo", manifest, "ui"); err != nil {
		return err
	}

	if err := build.RunWithOptions(
		ctx,
		project,
		build.Options{Target: "web", Host: os.Getenv("KARTY_HOST_WEB")},
	); err != nil {
		return err
	}

	if err := checkUIArtifacts(filepath.Join(project, "dist/web/game.kart")); err != nil {
		return err
	}

	return command(ctx, root, nil, "node", "internal/build/testdata/ui-browser.test.mjs", filepath.Join(project, "dist/web"))
}

func checkUIArtifacts(path string) error {
	wasm, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read UI artifact: %w", err)
	}

	section, err := cartridge.ExtractSection(wasm, ui.SectionName)
	if err != nil {
		return err
	}

	assets, err := cartridge.DecodeAssets(section)
	if err != nil {
		return err
	}

	expected := map[string]bool{
		"ui.error":     true,
		"ui.inventory": true,
		"ui.item-row":  true,
		"ui.levels":    true,
		"ui.loading":   true,
		"ui.menu":      true,
		"ui.pause":     true,
		"ui.releasing": true,
	}
	if len(assets) != len(expected) {
		return fmt.Errorf("expected %d retained UI assets, got %d: %w", len(expected), len(assets), ui.ErrTemplate)
	}

	for _, asset := range assets {
		if !expected[asset.Name] {
			return fmt.Errorf("unexpected retained UI asset %s: %w", asset.Name, ui.ErrTemplate)
		}

		delete(expected, asset.Name)
	}

	return nil
}
