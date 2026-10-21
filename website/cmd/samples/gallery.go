package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/karty-game/karty/internal/brand"
	"github.com/karty-game/karty/website"
	"github.com/pelletier/go-toml/v2"
)

const maxPreviewBytes = 512 * 1024

// Each sample keeps its introduction beside its source and screenshot.
type samplePreview struct {
	Hidden      bool   `toml:"hidden"`
	Title       string `toml:"title"`
	Description string `toml:"description"`
	Alt         string `toml:"alt"`
}

type sampleCard struct {
	Preview   samplePreview
	DemoURL   string
	ImageURL  string
	SourceURL string
}

func sampleCards(root string, names []string) ([]sampleCard, error) {
	cards := make([]sampleCard, 0, len(names))

	for _, name := range names {
		if !samplePathPart.MatchString(name) || name == "guide" || name == "design" || name == "technical" || name == "runtime" {
			return nil, fmt.Errorf("invalid sample name %q: %w", name, os.ErrInvalid)
		}

		preview, err := readSamplePreview(filepath.Join(root, "samples", name))
		if err != nil {
			return nil, fmt.Errorf("sample %s preview: %w", name, err)
		}

		cards = append(cards, sampleCard{
			Preview: preview, DemoURL: name + "/", ImageURL: name + "/preview.jpg",
			SourceURL: "https://github.com/karty-game/karty/tree/main/samples/" + name,
		})
	}

	return cards, nil
}

func readSamplePreview(directory string) (samplePreview, error) {
	var preview samplePreview

	root, err := os.OpenRoot(directory)
	if err != nil {
		return preview, err
	}

	defer root.Close()

	for _, name := range []string{"preview.toml", "preview.jpg"} {
		info, err := root.Lstat(name)
		if err != nil {
			return preview, err
		}

		if !info.Mode().IsRegular() || info.Size() > maxPreviewBytes {
			return preview, fmt.Errorf("%s must be a regular file under 512 KiB: %w", name, os.ErrInvalid)
		}
	}

	metadata, err := root.ReadFile("preview.toml")
	if err != nil {
		return preview, err
	}

	if err = toml.NewDecoder(bytes.NewReader(metadata)).DisallowUnknownFields().Decode(&preview); err != nil {
		return preview, err
	}

	for _, text := range []string{preview.Title, preview.Description, preview.Alt} {
		if strings.TrimSpace(text) == "" || len(text) > 400 {
			return preview, fmt.Errorf("preview title, description and alt must contain 1 to 400 characters: %w", os.ErrInvalid)
		}
	}

	image, err := root.ReadFile("preview.jpg")
	if err != nil {
		return preview, err
	}

	config, err := jpeg.DecodeConfig(bytes.NewReader(image))
	if err != nil || config.Width != 960 || config.Height != 540 {
		return preview, fmt.Errorf("preview.jpg must be a 960 by 540 JPEG: %w", os.ErrInvalid)
	}

	return preview, nil
}

func copySamplePreview(directory, target string) error {
	if _, err := readSamplePreview(directory); err != nil {
		return err
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}

	defer root.Close()

	contents, err := root.ReadFile("preview.jpg")
	if err != nil {
		return err
	}

	output, err := os.OpenRoot(target)
	if err != nil {
		return err
	}

	defer output.Close()

	if info, err := output.Lstat("preview.jpg"); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("preview output must be a regular file: %w", os.ErrInvalid)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	return output.WriteFile("preview.jpg", contents, 0o600)
}

func writeGallery(directory string, cards []sampleCard) error {
	// Hidden projects still build and retain their direct URLs.
	cards = slices.DeleteFunc(slices.Clone(cards), func(card sampleCard) bool { return card.Preview.Hidden })

	social, err := website.NewSocial(
		"Karty samples",
		"Play Karty samples, explore their Go source, and choose a guide for your own 2D or 3D game.",
		"",
	)
	if err != nil {
		return err
	}

	page, err := template.New("gallery.html").Funcs(template.FuncMap{
		"faviconURL": brand.FaviconURL,
		"siteCSS": func() template.CSS {
			return template.CSS(website.CSS()) // #nosec G203 -- stylesheet is trusted, embedded repository source.
		},
	}).ParseFS(website.Files, "templates/gallery.html", "templates/social.html")
	if err != nil {
		return err
	}

	var output bytes.Buffer

	if err = page.Execute(&output, struct {
		Cards  []sampleCard
		Social website.Social
	}{cards, social}); err != nil {
		return err
	}

	return writeGalleryFile(directory, output.Bytes())
}

func writeGalleryFile(directory string, contents []byte) error {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()

	if info, err := root.Lstat("index.html"); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("gallery output must be a regular file: %w", os.ErrInvalid)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	const temporary = ".index.html.tmp"

	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temporary) }()

	_, writeErr := file.Write(contents)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}

	return root.Rename(temporary, "index.html")
}
