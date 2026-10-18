package packages

import (
	"context"
	"fmt"
	"os"
	"path"
	"reflect"

	"github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/project"
	"github.com/pelletier/go-toml/v2"
)

// ResolveUI adds package exports to the in-memory project, staging all inputs
// inside its root. Authored sources and the project's theme file stay untouched.
func ResolveUI(ctx context.Context, directory string, assets project.Assets, set *Set) (project.Assets, error) {
	if err := ctx.Err(); err != nil {
		return assets, err
	}

	if set == nil || len(set.Items) == 0 {
		return assets, nil
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return assets, err
	}
	defer root.Close()

	for _, name := range set.Names() {
		assets, err = appendUIExports(ctx, root, assets, set.Items[name])
		if err != nil {
			return assets, err
		}
	}

	assets.Fonts, err = resolveUIFonts(ctx, root, assets.Fonts, set)
	if err != nil {
		return assets, err
	}

	assets.Theme.Source, err = resolveUITheme(ctx, root, assets.Theme.Source, set)

	return assets, err
}

func appendUIExports(ctx context.Context, root *os.Root, assets project.Assets, item *Resolved) (project.Assets, error) {
	uiNames, imageNames := map[string]bool{}, map[string]bool{}
	for _, entry := range assets.UI {
		uiNames[entry.Name] = true
	}

	for _, entry := range assets.Textures {
		imageNames[entry.Name] = true
	}

	exports := item.Manifest.UI
	for _, entry := range exports.Components {
		qualified := item.Manifest.Name + "." + entry.Name
		if uiNames[qualified] {
			return assets, fmt.Errorf("%w: duplicate UI asset %s", ErrPackage, qualified)
		}

		uiNames[qualified] = true

		source, err := stageUISource(ctx, root, item, entry.Source)
		if err != nil {
			return assets, err
		}

		assets.UI = append(assets.UI, project.Texture{Name: qualified, Source: source})
	}

	for _, entry := range exports.Layouts {
		source, err := stageUISource(ctx, root, item, entry)
		if err != nil {
			return assets, err
		}

		assets.Layouts = append(assets.Layouts, project.Layout{Source: source})
	}

	for _, entry := range exports.Images {
		qualified := item.Manifest.Name + "." + entry.Name
		if imageNames[qualified] {
			return assets, fmt.Errorf("%w: duplicate UI image %s", ErrPackage, qualified)
		}

		imageNames[qualified] = true

		source, err := stageUIFile(ctx, root, item.Root, entry.Source, MaxFileBytes)
		if err != nil {
			return assets, err
		}

		assets.Textures = append(assets.Textures, project.Texture{Name: qualified, Source: source, Profile: entry.Profile})
	}

	return assets, nil
}

func resolveUIFonts(ctx context.Context, root *os.Root, consumer []project.Font, set *Set) ([]project.Font, error) {
	fonts := map[string]bool{}
	for _, entry := range consumer {
		fonts[entry.Role] = true
	}

	defaults := map[string]project.Font{}

	for _, name := range set.Names() {
		item := set.Items[name]
		for _, entry := range item.Manifest.UI.Fonts {
			if fonts[entry.Role] {
				continue
			}

			source, err := stageUIFile(ctx, root, item.Root, entry.Source, MaxFileBytes)
			if err != nil {
				return nil, err
			}

			font := project.Font{Role: entry.Role, Source: source}
			if previous, exists := defaults[entry.Role]; exists && previous.Source != font.Source {
				return nil, fmt.Errorf("%w: conflicting package font role %s; provide a project font", ErrPackage, entry.Role)
			}

			defaults[entry.Role] = font
		}
	}

	result := append([]project.Font(nil), consumer...)

	for _, role := range []string{"body", "display", "mono"} {
		if font, exists := defaults[role]; exists {
			result = append(result, font)
		}
	}

	return result, nil
}

func resolveUITheme(ctx context.Context, root *os.Root, consumer string, set *Set) (string, error) {
	consumerTheme, err := readUITheme(ctx, root, consumer)
	if err != nil {
		return "", err
	}

	theme := map[string]any{"extends": "dark"}

	for _, name := range set.Names() {
		item := set.Items[name]

		values, err := readUITheme(ctx, item.Root, item.Manifest.UI.Theme)
		if err != nil {
			return "", err
		}

		if err := qualifyUIImages(values, name, item.Manifest.UI.Images); err != nil {
			return "", err
		}

		if err := mergeUITheme(theme, values, consumerTheme); err != nil {
			return "", err
		}
	}

	if len(theme) == 1 {
		return consumer, nil
	}

	if err := mergeUITheme(theme, consumerTheme, consumerTheme); err != nil {
		return "", err
	}

	encoded, err := toml.Marshal(theme)
	if err != nil {
		return "", err
	}

	if len(encoded) > ui.MaxAssetBytes {
		return "", ErrPackage
	}

	name := ".karty/package-ui/" + Digest(encoded) + "/theme.toml"
	if err := WriteGenerated(ctx, root, name, encoded, true); err != nil {
		return "", err
	}

	return name, nil
}

// UI source identities must stay stable across presentation edits: their paths
// appear in generated Go line directives and compiler debug information. Atomic
// cache copies preserve confinement without turning a style edit into new code.
func stageUISource(ctx context.Context, target *os.Root, item *Resolved, name string) (string, error) {
	data, err := Read(ctx, item.Root, name, ui.MaxAssetBytes)
	if err != nil {
		return "", err
	}

	staged := ".karty/package-ui/sources/" + Digest([]byte(item.Manifest.Name+"\x00"+name)) + "/" + path.Base(name)
	if err := WriteGenerated(ctx, target, staged, data, false); err != nil {
		return "", err
	}

	return staged, nil
}

func stageUIFile(ctx context.Context, target, source *os.Root, name string, limit int64) (string, error) {
	data, err := Read(ctx, source, name, limit)
	if err != nil {
		return "", err
	}

	staged := ".karty/package-ui/" + Digest(data) + "/" + path.Base(name)
	if err := WriteGenerated(ctx, target, staged, data, true); err != nil {
		return "", err
	}

	return staged, nil
}

func readUITheme(ctx context.Context, root *os.Root, name string) (map[string]any, error) {
	if name == "" {
		return map[string]any{}, nil
	}

	data, err := Read(ctx, root, name, ui.MaxAssetBytes)
	if err != nil {
		return nil, err
	}

	if _, err := uicompiler.ParseTheme(name, data); err != nil {
		return nil, err
	}

	var result map[string]any
	if err := toml.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func qualifyUIImages(theme map[string]any, name string, images []Texture) error {
	declared := map[string]bool{}
	for _, image := range images {
		declared[image.Name] = true
	}

	group, _ := theme["images"].(map[string]any)
	for key, value := range group {
		image, imageValid := value.(map[string]any)
		if !imageValid {
			return ErrPackage
		}

		asset, assetValid := image["asset"].(string)
		if !assetValid {
			return ErrPackage
		}

		if !declared[asset] {
			return fmt.Errorf("%w: package theme image %s references undeclared image %s", ErrPackage, key, asset)
		}

		image["asset"] = name + "." + asset
	}

	return nil
}

func mergeUITheme(target, values, consumer map[string]any) error {
	for group, value := range values {
		if group == "extends" {
			continue
		}

		entries, ok := value.(map[string]any)
		if !ok {
			return ErrPackage
		}

		merged, _ := target[group].(map[string]any)
		if merged == nil {
			merged = map[string]any{}
			target[group] = merged
		}

		overrides, _ := consumer[group].(map[string]any)
		for key, token := range entries {
			if override, exists := overrides[key]; exists {
				merged[key] = override

				continue
			}

			if previous, exists := merged[key]; exists && !reflect.DeepEqual(previous, token) {
				return fmt.Errorf("%w: conflicting package theme token %s.%s; provide a project override", ErrPackage, group, key)
			}

			merged[key] = token
		}
	}

	return nil
}
