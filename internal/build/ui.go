package build

import (
	"fmt"
	"os"
	"strings"

	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/assetusage"
	"github.com/karty-game/karty/internal/project"
)

// UI assets have their own embedded section; they are never staged loose.
func embedUIAssets(
	directory, artifact string,
	entries []project.Texture,
	layouts []project.Layout,
	themeSource string,
) error {
	if len(entries) == 0 {
		return nil
	}

	module, err := project.ModulePath(directory)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}

	views, err := project.CompileUI(directory, entries, layouts, themeSource)
	if err != nil {
		return err
	}

	methods := map[string]string{}
	for _, view := range views {
		methods["Show"+view.Name] = view.Asset
	}

	usage := assetusage.AnalyzeUIViews(directory, module, names, methods)

	assets := make([]cartridge.Asset, 0, len(entries))
	theme := uicompiler.DefaultTheme()

	if themeSource != "" {
		data, readErr := uicompiler.ReadSource(directory, themeSource)
		if readErr != nil {
			return readErr
		}

		theme, err = uicompiler.ParseTheme(themeSource, data)
		if err != nil {
			return err
		}
	}

	for _, entry := range entries {
		if entry.Name == "" {
			return ui.ErrTemplate
		}

		data, err := uicompiler.ReadSource(directory, entry.Source)
		if err != nil {
			return err
		}

		var template ui.Template

		if strings.HasSuffix(entry.Source, ".ui") {
			var component uicompiler.Component

			component, err = compileUIEntry(entry.Source, data, theme, views)
			template = component.Template
		} else {
			template, err = ui.Decode(data)
		}

		if err != nil {
			return fmt.Errorf("UI %q: %w", entry.Name, err)
		}

		encoded, err := ui.EncodeComposition(template)
		if err != nil {
			return err
		}

		if _, live := usage.Live[entry.Name]; live || usage.KeepAll || entry.Keep {
			assets = append(assets, cartridge.Asset{Name: entry.Name, Bytes: encoded})
		}
	}

	bundle, err := cartridge.EncodeAssets(assets)
	if err != nil {
		return fmt.Errorf("encode UI bundle: %w", err)
	}

	wasm, err := os.ReadFile(artifact)
	if err != nil {
		return fmt.Errorf("read cartridge for UI: %w", err)
	}

	wasm, err = cartridge.EmbedSection(wasm, ui.SectionName, bundle)
	if err != nil {
		return fmt.Errorf("embed UI: %w", err)
	}
	//nolint:gosec // Fixed build-owned dist/raw/game.kart artifact.
	if err := os.WriteFile(artifact, wasm, 0o600); err != nil {
		return fmt.Errorf("write UI cartridge: %w", err)
	}

	return nil
}

func compileUIEntry(source string, data []byte, theme uicompiler.Theme, views []uicompiler.Component) (uicompiler.Component, error) {
	for _, view := range views {
		if view.Source == source {
			return view, nil
		}
	}

	return uicompiler.CompileWithTheme(source, data, theme)
}
