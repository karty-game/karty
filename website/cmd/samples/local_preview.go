package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/internal/brand"
	"github.com/karty-game/karty/website"
)

// Local docs use existing artifacts without installing an SDK. Copy a complete,
// confined snapshot before replacing the local preview.
func previewLocalSample(project, output *os.Root, name string, card sampleCard) error {
	stage := filepath.Join("dist", ".docs-sample-"+rand.Text())
	if err := project.Mkdir(stage, 0o700); err != nil {
		return err
	}
	defer func() { _ = project.RemoveAll(stage) }()

	sourcePath := filepath.Join("samples", name, "dist", "web")

	info, err := project.Lstat(sourcePath)
	switch {
	case os.IsNotExist(err):
		if err = writeLocalSampleInstructions(project, stage, card); err != nil {
			return err
		}
	case err != nil:
		return err
	case !info.IsDir() || info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("local sample web output must be a real directory: %w", os.ErrInvalid)
	default:
		source, openErr := project.OpenRoot(sourcePath)
		if openErr != nil {
			return openErr
		}
		defer source.Close()

		if err = validateLocalSampleWeb(source.FS(), name); err != nil {
			return err
		}

		if err = copySamples(project, source.FS(), stage); err != nil {
			return err
		}
	}

	if err = copySamplePreview(filepath.Join(project.Name(), "samples", name), filepath.Join(project.Name(), stage)); err != nil {
		return err
	}

	if _, err = output.Lstat(name); err == nil {
		if err = requirePreviewDirectory(output, name); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	// Both directories remain under the same project root. Preserve the prior
	// snapshot if publication of the new one fails.
	destination := filepath.Join("dist", "samples-site", name)
	backup := stage + "-previous"

	hadPrevious := err == nil
	if hadPrevious {
		if err = project.Rename(destination, backup); err != nil {
			return err
		}
	}

	if err = project.Rename(stage, destination); err != nil {
		if hadPrevious {
			return errors.Join(err, project.Rename(backup, destination))
		}

		return err
	}

	if hadPrevious {
		return project.RemoveAll(backup)
	}

	return nil
}

func validateLocalSampleWeb(source fs.FS, name string) error {
	count, total := 0, int64(0)

	if err := fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if path == "." {
			return nil
		}

		size, err := validateEntry(name+"/"+path, entry)
		if err != nil {
			return err
		}

		count++

		total += size
		if count > maxSampleEntries || total > maxSampleBytes {
			return fmt.Errorf("local sample exceeds preview bounds: %w", os.ErrInvalid)
		}

		return nil
	}); err != nil {
		return err
	}

	for _, name := range []string{"index.html", "game.kart", "karty.js", "karty-host.wasm", "wasm_exec.js"} {
		if err := requireSampleFile(source, name); err != nil {
			return err
		}
	}

	return nil
}

func writeLocalSampleInstructions(project *os.Root, stage string, card sampleCard) error {
	social, err := website.NewSocial(card.Preview.Title+" · Karty samples", card.Preview.Description, card.DemoURL)
	if err != nil {
		return err
	}

	page, err := template.New("local-preview.html").Funcs(template.FuncMap{
		"faviconURL": brand.FaviconURL,
		"siteCSS": func() template.CSS {
			return template.CSS(website.CSS()) // #nosec G203 -- trusted embedded stylesheet.
		},
	}).ParseFS(website.Files, "templates/local-preview.html", "templates/social.html")
	if err != nil {
		return err
	}

	var output bytes.Buffer
	if err = page.Execute(&output, struct {
		Preview   samplePreview
		SourceURL string
		Social    website.Social
	}{card.Preview, card.SourceURL, social}); err != nil {
		return err
	}

	return project.WriteFile(filepath.Join(stage, "index.html"), output.Bytes(), 0o600)
}
