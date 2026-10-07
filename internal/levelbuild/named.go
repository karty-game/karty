package levelbuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty-sdk/format/level"
	uicompiler "github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/pelletier/go-toml/v2"
)

// BuildNamedWithAssets builds only the selected level, without compiling a game
// client or unrelated level assets. Names may be logical names or folder names.
func BuildNamedWithAssets(
	ctx context.Context,
	root, name string,
	uiSchema uint32,
	themeSource string,
	selected sdk.Manifest,
) (Artifact, error) {
	return buildNamed(ctx, root, name, uiSchema, themeSource, selected, false)
}

// BuildNamedForScreenshot reuses a valid bake or creates a four-sample,
// one-bounce bake for the selected world before packaging its resources.
func BuildNamedForScreenshot(
	ctx context.Context,
	root, name string,
	uiSchema uint32,
	themeSource string,
	selected sdk.Manifest,
) (Artifact, error) {
	return buildNamed(ctx, root, name, uiSchema, themeSource, selected, true)
}

func buildNamed(
	ctx context.Context,
	root, name string,
	uiSchema uint32,
	themeSource string,
	selected sdk.Manifest,
	screenshot bool,
) (Artifact, error) {
	entries, err := os.ReadDir(filepath.Join(root, "levels"))
	if err != nil {
		return Artifact{}, err
	}

	var directory string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		candidate := filepath.Join(root, "levels", entry.Name())

		data, err := readConfinedFile(candidate, "level.toml", level.MaxMetadataSize)
		if os.IsNotExist(err) {
			continue
		}

		if err != nil {
			return Artifact{}, err
		}

		var definition manifest
		if err := toml.Unmarshal(data, &definition); err != nil {
			return Artifact{}, err
		}

		if name != entry.Name() && name != definition.Level.Name {
			continue
		}

		if directory != "" {
			return Artifact{}, fmt.Errorf("ambiguous level %q: %w", name, ErrDuplicateName)
		}

		if definition.Level.Kind != "world" || definition.World.Source == "" {
			return Artifact{}, fmt.Errorf("screenshot requires a world level: %w", ErrManifest)
		}

		directory = candidate
	}

	if directory == "" {
		return Artifact{}, fmt.Errorf("unknown level %q: %w", name, os.ErrNotExist)
	}

	theme := uicompiler.DefaultTheme()

	if themeSource != "" {
		data, err := uicompiler.ReadSource(root, themeSource)
		if err != nil {
			return Artifact{}, err
		}

		theme, err = uicompiler.ParseTheme(themeSource, data)
		if err != nil {
			return Artifact{}, err
		}
	}

	return build(ctx, directory, uiSchema, theme, &assetBuild{projectRoot: root, manifest: selected, screenshot: screenshot})
}
