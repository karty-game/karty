package levelbuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/packages"
)

// Package inputs are staged inside the level root, so every existing compiler,
// schema, sky, reflection, bake and texture read keeps its confinement rules.
// The authored level identity and all unrelated manifest settings are preserved.
func resolveLevelPackages(ctx context.Context, directory string, definition manifest, set *packages.Set) (manifest, error) {
	if set == nil || len(set.Items) == 0 || definition.World.Source == "" {
		return definition, nil
	}

	data, err := readConfinedFile(directory, definition.World.Source, world.MaxEncodedSize)
	if err != nil {
		return definition, err
	}

	data, textures, err := packages.ResolveWorld(ctx, set, data, schemaTextures(definition))
	if err != nil {
		return definition, fmt.Errorf("%s package imports: %w", directory, err)
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return definition, err
	}
	defer root.Close()

	prefix := ".karty/packages/"
	worldPath := prefix + packages.Digest(data) + ".yaml"

	if err := ctx.Err(); err != nil {
		return definition, err
	}

	if err := packages.WriteGenerated(ctx, root, worldPath, data, true); err != nil {
		return definition, err
	}

	definition.World.Source = worldPath

	for _, texture := range textures {
		name := prefix + packages.Digest(texture.Data) + ".png"
		if err := packages.WriteGenerated(ctx, root, name, texture.Data, true); err != nil {
			return definition, err
		}

		entry := textureEntry{Name: texture.Name, Source: name, Profile: texture.Profile}
		if options := texture.Materialize; options != nil {
			entry.Materialize = &materializeSettings{
				Preset: options.Preset, NormalStrength: options.NormalStrength,
				HeightContrast: options.HeightContrast, MetallicScale: options.MetallicScale, AODepthScale: options.AODepthScale,
			}
		}

		definition.Textures = append(definition.Textures, entry)
	}

	return definition, nil
}

func levelPackages(ctx context.Context, directory string) (*packages.Set, error) {
	return packages.Restore(ctx, filepath.Dir(filepath.Dir(directory)))
}
