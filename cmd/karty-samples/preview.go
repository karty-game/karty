package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/internal/guide"
)

// previewSamples refreshes reading pages and screenshots without rebuilding games.
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
	if err := guide.Generate(filepath.Join(site, "guide")); err != nil {
		return err
	}

	output, err := os.OpenRoot(site)
	if err != nil {
		return err
	}
	defer output.Close()

	for _, name := range names {
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
