package build

import (
	"context"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/karty-game/karty/internal/packages"
)

func writeGeneratedClientAPI(ctx context.Context, directory string, generated map[string][]byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("open generated client API project: %w", err)
	}
	defer root.Close()

	names := make([]string, 0, len(generated))
	for name, contents := range generated {
		if !packages.ValidPath(name) || len(contents) > packages.MaxArchiveBytes {
			return fmt.Errorf("generated client API path %q: %w", name, os.ErrInvalid)
		}

		names = append(names, name)
	}

	slices.Sort(names)

	legacy := []string{"assets.go", "sounds.go", "videos.go", "music.go", "environments.go", "ui-assets.go"}
	// Check the entire write and removal set before replacing the old assets.
	// Rooted operations enforce confinement even if a path changes afterwards.
	if err := checkGeneratedPath(root, ".karty/assets", true); err != nil {
		return err
	}

	for _, name := range legacy {
		if err := checkGeneratedPath(root, ".karty/engine/"+name, false); err != nil {
			return err
		}
	}

	for _, name := range names {
		if err := checkGeneratedPath(root, ".karty/"+name, false); err != nil {
			return err
		}

		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if _, conflict := generated[parent]; conflict {
				return fmt.Errorf("generated client API paths %q and %q conflict: %w", parent, name, os.ErrInvalid)
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := root.RemoveAll(".karty/assets"); err != nil {
		return fmt.Errorf("replace generated assets package: %w", err)
	}

	for _, name := range legacy {
		if err := root.Remove(".karty/engine/" + name); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove legacy generated asset file %s: %w", name, err)
		}
	}

	for _, name := range names {
		// Publish with rename so neither file symlinks nor hard links can
		// redirect a write into another file.
		if err := packages.WriteGenerated(ctx, root, ".karty/"+name, generated[name], false); err != nil {
			return fmt.Errorf("write generated client API %s: %w", name, err)
		}
	}

	return nil
}

// checkGeneratedPath rejects existing symlinks, including links that stay within
// the project: generated infrastructure must never redirect into user files.
func checkGeneratedPath(root *os.Root, name string, directory bool) error {
	if !packages.ValidPath(name) {
		return fmt.Errorf("generated output path %q: %w", name, os.ErrInvalid)
	}

	parts := strings.Split(name, "/")
	for index := range parts {
		part := strings.Join(parts[:index+1], "/")

		info, err := root.Lstat(part)
		if os.IsNotExist(err) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("inspect generated output %s: %w", part, err)
		}

		wantDirectory := directory || index < len(parts)-1
		if wantDirectory && !info.IsDir() {
			return fmt.Errorf("generated output %s must be a real directory: %w", part, os.ErrInvalid)
		}

		if !wantDirectory && !info.Mode().IsRegular() {
			return fmt.Errorf("generated output %s must be a regular file: %w", part, os.ErrInvalid)
		}
	}

	return nil
}

func removeLegacyGeneratedClientAPI(directory string) error {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("open legacy generated client API project: %w", err)
	}
	defer root.Close()

	paths := []string{"api_codegen.go", "src/api_codegen.go"}

	for _, parent := range []string{"src/engine", ".karty/engine"} {
		for _, name := range []string{"game_codegen.go", "components_codegen.go"} {
			paths = append(paths, parent+"/"+name)
		}
	}

	for _, name := range paths {
		if err := checkGeneratedPath(root, name, false); err != nil {
			return err
		}
	}

	for _, name := range paths {
		if err := root.Remove(name); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove legacy generated client API: %w", err)
		}
	}

	return nil
}
