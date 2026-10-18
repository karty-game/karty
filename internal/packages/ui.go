package packages

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty-ui/schema"
)

const MaxUIComponents = 128

// UIExports are authored sources, compiled against the consuming game's SDK.
// Component and layout names retain KartUI's ordinary filename convention.
type UIExports struct {
	Components []UIComponent `toml:"components,omitempty"`
	Layouts    []string      `toml:"layouts,omitempty"`
	Images     []Texture     `toml:"images,omitempty"`
	Fonts      []UIFont      `toml:"fonts,omitempty"`
	Theme      string        `toml:"theme,omitempty"`
}

type UIComponent struct {
	Name   string `toml:"name"`
	Source string `toml:"source"`
}

type UIFont struct {
	Role   string `toml:"role"`
	Source string `toml:"source"`
}

func (exports UIExports) empty() bool {
	return len(exports.Components) == 0 && len(exports.Layouts) == 0 && len(exports.Images) == 0 && len(exports.Fonts) == 0 &&
		exports.Theme == ""
}

// Validate every advertised source, including unused components. A private copy
// gives the existing compiler ordinary confined files without following package
// links or relaxing its project root. Large image/font bytes aren't retained.
func validateUI(ctx context.Context, root *os.Root, exports UIExports) error {
	if exports.empty() {
		return nil
	}

	theme, err := readUITheme(ctx, root, exports.Theme)
	if err != nil {
		return err
	}

	if err := qualifyUIImages(theme, "package", exports.Images); err != nil {
		return err
	}

	inputs, err := uiInputs(exports)
	if err != nil {
		return err
	}

	data := map[string][]byte{}
	total := 0

	for name, limit := range inputs {
		contents, err := Read(ctx, root, name, limit)
		if err != nil {
			return fmt.Errorf("package UI %s: %w", name, err)
		}

		total += len(contents)
		if total > MaxTotalBytes {
			return ErrPackage
		}

		if strings.HasSuffix(name, ".kui") || name == exports.Theme {
			data[name] = contents
		}
	}

	directory, err := os.MkdirTemp("", "karty-package-ui-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)

	stage, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer stage.Close()

	for name, contents := range data {
		if err := WriteGenerated(ctx, stage, name, contents, true); err != nil {
			return err
		}
	}

	sources := make([]uicompiler.Source, 0, len(exports.Components))
	for _, component := range exports.Components {
		sources = append(sources, uicompiler.Source{Name: component.Name, Source: component.Source})
	}

	layouts := make([]uicompiler.LayoutSource, 0, len(exports.Layouts))
	for _, source := range exports.Layouts {
		layouts = append(layouts, uicompiler.LayoutSource{Source: source})
	}

	_, err = uicompiler.LoadProjectWithTheme(directory, sources, layouts, exports.Theme)

	return err
}

func uiInputs(exports UIExports) (map[string]int64, error) {
	if len(exports.Components)+len(exports.Layouts) > MaxUIComponents || len(exports.Images) > MaxTextures || len(exports.Fonts) > 3 {
		return nil, ErrPackage
	}

	inputs := map[string]int64{}

	names := map[string]bool{}
	for _, entry := range exports.Components {
		if !namePattern.MatchString(entry.Name) || names[entry.Name] || !validPath(entry.Source) ||
			!strings.HasSuffix(entry.Source, ".kui") {
			return nil, ErrPackage
		}

		names[entry.Name] = true
		inputs[entry.Source] = ui.MaxAssetBytes
	}

	for _, source := range exports.Layouts {
		if !validPath(source) || !strings.HasSuffix(source, ".kui") || inputs[source] != 0 {
			return nil, ErrPackage
		}

		inputs[source] = ui.MaxAssetBytes
	}

	names = map[string]bool{}
	for _, entry := range exports.Images {
		if !namePattern.MatchString(entry.Name) || names[entry.Name] || !validPath(entry.Source) ||
			!strings.HasSuffix(entry.Source, ".png") ||
			entry.Profile == "" {
			return nil, ErrPackage
		}

		names[entry.Name] = true
		inputs[entry.Source] = MaxFileBytes
	}

	names = map[string]bool{}
	for _, entry := range exports.Fonts {
		if (entry.Role != "body" && entry.Role != "display" && entry.Role != "mono") || names[entry.Role] || !validPath(entry.Source) ||
			(path.Ext(entry.Source) != ".ttf" && path.Ext(entry.Source) != ".otf") {
			return nil, ErrPackage
		}

		names[entry.Role] = true
		inputs[entry.Source] = MaxFileBytes
	}

	if exports.Theme != "" {
		if !validPath(exports.Theme) || inputs[exports.Theme] != 0 {
			return nil, ErrPackage
		}

		inputs[exports.Theme] = ui.MaxAssetBytes
	}

	return inputs, nil
}
