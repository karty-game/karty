package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/website/internal/generator"
)

// previewSamples refreshes reading pages and screenshots without rebuilding games.
// Local demos use a complete snapshot of their existing web build.
func previewSamples(directory string) error {
	root, err := filepath.Abs(directory)
	if err != nil {
		return err
	}

	names, err := sampleNames(root)
	if err != nil {
		return err
	}

	cards, err := sampleCards(root, names)
	if err != nil {
		return err
	}

	site := filepath.Join(root, "dist", "samples-site")
	if err := generator.Generate(filepath.Join(site, "guide")); err != nil {
		return err
	}

	if err := generator.GenerateDesign(filepath.Join(site, "design")); err != nil {
		return err
	}

	if err := generator.GenerateTechnical(filepath.Join(site, "technical")); err != nil {
		return err
	}

	output, err := os.OpenRoot(site)
	if err != nil {
		return err
	}
	defer output.Close()

	project, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer project.Close()

	for index, name := range names {
		local, err := useLocalSample(project, output, name)
		if err != nil {
			return err
		}

		if local {
			if err := previewLocalSample(project, output, name, cards[index]); err != nil {
				return err
			}

			continue
		}

		if err := output.Mkdir(name, 0o755); err != nil && !os.IsExist(err) {
			return err
		}

		if err := requirePreviewDirectory(output, name); err != nil {
			return err
		}

		if err := copySamplePreview(filepath.Join(root, "samples", name), filepath.Join(site, name)); err != nil {
			return err
		}
	}

	return writeGallery(site, cards)
}

// Local builds take precedence over assembled previews regardless of SDK release
// status. Preserve existing assembled games when no local build is available.
func useLocalSample(project, output *os.Root, name string) (bool, error) {
	if _, err := project.Lstat(filepath.Join("samples", name, "dist", "web")); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}

	_, err := output.Lstat(name)
	if os.IsNotExist(err) {
		return true, nil
	}

	return false, err
}

func requirePreviewDirectory(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("preview output must be a real directory: %w", os.ErrInvalid)
	}

	return nil
}
