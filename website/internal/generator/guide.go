// Package generator builds the public website documentation from handwritten Markdown.
package generator

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/karty-game/karty/website"
)

// Generate writes the guide landing page, game paths, shared topics, complete
// guide, downloadable Markdown and embedded images beneath output. It leaves
// other files alone and refuses linked or non-regular destinations.
func Generate(output string) error {
	files, err := pageFiles()
	if err != nil {
		return err
	}

	return generateFiles(output, files)
}

// GenerateDesign writes the design section into its explicitly selected directory.
// It uses the same confined output handling and offline theme as the guides.
func GenerateDesign(output string) error {
	pages, err := designPages()
	if err != nil {
		return err
	}

	files, err := renderPages(pages)
	if err != nil {
		return err
	}

	return generateFiles(output, files)
}

// GenerateTechnical writes API topics and downloads into a confined directory.
func GenerateTechnical(output string) error {
	pages, err := technicalPages()
	if err != nil {
		return err
	}

	files, err := renderPages(pages)
	if err != nil {
		return err
	}

	return generateFiles(output, files)
}

func generateFiles(output string, files []generatedFile) error {
	root, err := openOutput(output)
	if err != nil {
		return err
	}
	defer root.Close()

	assetRoot, err := childOutput(root, "assets")
	if err != nil {
		return err
	}
	defer assetRoot.Close()

	assets, err := website.Files.ReadDir("assets")
	if err != nil {
		return err
	}

	for _, asset := range assets {
		if err := requireRegularDestination(assetRoot, asset.Name()); err != nil {
			return err
		}
	}

	for _, file := range files {
		if err := requireRegularDestination(root, file.name); err != nil {
			return err
		}
	}

	for _, file := range files {
		if err := writeFile(root, file.name, file.contents); err != nil {
			return err
		}
	}

	for _, asset := range assets {
		contents, err := website.Files.ReadFile("assets/" + asset.Name())
		if err != nil {
			return err
		}

		if err := writeFile(assetRoot, asset.Name(), string(contents)); err != nil {
			return err
		}
	}

	return nil
}

func childOutput(root *os.Root, name string) (*os.Root, error) {
	if err := root.Mkdir(name, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("guide output %q is not a real directory: %w", name, os.ErrInvalid)
	}

	return root.OpenRoot(name)
}

func readMarkdown() (string, error) {
	chapters, err := website.Files.ReadDir("content/guides/chapters")
	if err != nil {
		return "", err
	}

	var markdown strings.Builder

	markdown.WriteString("# Make a game with Karty\n\nCreate a project, choose a 2D or 3D scene, and add an interaction you can play.\n\n")

	for _, chapter := range chapters { // ReadDir sorts chapter names.
		contents, err := website.Files.ReadFile("content/guides/chapters/" + chapter.Name())
		if err != nil {
			return "", err
		}

		markdown.WriteString(strings.TrimSpace(string(contents)))
		markdown.WriteString("\n\n")
	}

	for _, name := range []string{"2d", "fps", "isometric"} {
		contents, err := website.Files.ReadFile("content/guides/tracks/" + name + ".md")
		if err != nil {
			return "", err
		}

		track := strings.TrimSpace(string(contents))
		if body, exists := strings.CutPrefix(track, "# "); exists {
			track = "## " + strings.ReplaceAll(body, "\n## ", "\n### ")
		}

		markdown.WriteString(track)
		markdown.WriteString("\n\n")
	}

	return markdown.String(), nil
}

func openOutput(output string) (*os.Root, error) {
	if strings.TrimSpace(output) == "" {
		return nil, fmt.Errorf("guide output directory is required: %w", os.ErrInvalid)
	}

	absolute, err := filepath.Abs(output)
	if err != nil {
		return nil, err
	}

	volumeRoot := filepath.VolumeName(absolute) + string(filepath.Separator)

	root, err := os.OpenRoot(volumeRoot)
	if err != nil {
		return nil, err
	}

	// Open each directory separately so a linked output parent cannot redirect
	// writes. All subsequent file operations stay rooted in the selected folder.
	for part := range strings.SplitSeq(strings.TrimPrefix(absolute, volumeRoot), string(filepath.Separator)) {
		if part == "" {
			continue
		}

		if err := root.Mkdir(part, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			root.Close()

			return nil, err
		}

		info, err := root.Lstat(part)
		if err != nil {
			root.Close()

			return nil, err
		}

		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			root.Close()

			return nil, fmt.Errorf("guide output parent %q is not a real directory: %w", part, os.ErrInvalid)
		}

		next, err := root.OpenRoot(part)
		root.Close()

		if err != nil {
			return nil, err
		}

		root = next
	}

	return root, nil
}

func requireRegularDestination(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("guide output %q is not a regular file: %w", name, os.ErrInvalid)
	}

	return nil
}

func writeFile(root *os.Root, name, contents string) error {
	temporary := "." + name + ".tmp"

	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temporary) }()

	_, writeErr := io.WriteString(file, contents)

	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}

	return root.Rename(temporary, name)
}
