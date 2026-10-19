package levelbuild

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/internal/processorassets"
)

func appendProcessorTextures(directory string, definition *manifest, assets *assetBuild) error {
	if assets == nil || assets.projectRoot == "" {
		return nil
	}

	root, err := os.OpenRoot(assets.projectRoot)
	if err != nil {
		return err
	}
	defer root.Close()

	textures, err := processorassets.Read(root)
	if err != nil {
		return err
	}

	relative, err := filepath.Rel(filepath.Join(assets.projectRoot, "levels"), directory)
	if err != nil {
		return err
	}

	selected := []processorassets.Texture{}

	for _, texture := range textures {
		if texture.Level == filepath.ToSlash(relative) {
			selected = append(selected, texture)
		}
	}

	assigned, err := processorassets.AssignIDs(root, selected)
	if err != nil {
		return err
	}

	for index, texture := range selected {
		if assigned[index].ID != texture.ID || assigned[index].Name != texture.Name {
			return fmt.Errorf("%w: generated texture catalog does not match authored level", ErrSource)
		}

		definition.Textures = append(
			definition.Textures,
			textureEntry{Name: texture.Name, Source: texture.Source, Profile: texture.Profile, Generated: true},
		)
	}

	return nil
}
