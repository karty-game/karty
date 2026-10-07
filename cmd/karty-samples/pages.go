package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const maxSampleEntries = 10000
const maxSampleBytes = 200 * 1024 * 1024

func validateSamples(source fs.FS) error {
	var count int

	var total int64

	err := fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if path == "." {
			return nil
		}

		count++
		if count > maxSampleEntries {
			return fmt.Errorf("too many sample files: %w", os.ErrInvalid)
		}

		size, err := validateEntry(path, entry)
		if err != nil {
			return err
		}

		total += size
		if total > maxSampleBytes {
			return fmt.Errorf("sample output exceeds 200 MiB: %w", os.ErrInvalid)
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, name := range []string{"index.html", "ui-demo/index.html", "media-lab/index.html"} {
		if err = requireSampleFile(source, name); err != nil {
			return err
		}
	}

	if _, err = fs.Stat(source, "world-camera"); err == nil {
		return requireSampleFile(source, "world-camera/index.html")
	} else if !os.IsNotExist(err) {
		return err
	}

	return nil
}

func validateEntry(path string, entry fs.DirEntry) (int64, error) {
	if !entry.IsDir() && !entry.Type().IsRegular() {
		return 0, fmt.Errorf("non-regular sample entry: %s: %w", path, os.ErrInvalid)
	}

	parts := strings.Split(path, "/")
	for _, part := range parts {
		if !regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`).MatchString(part) {
			return 0, fmt.Errorf("invalid sample path: %s: %w", path, os.ErrInvalid)
		}
	}

	if !slices.Contains(append(sampleNames(), "index.html"), parts[0]) {
		return 0, fmt.Errorf("unexpected sample entry: %s: %w", path, os.ErrInvalid)
	}

	if entry.IsDir() {
		return 0, nil
	}

	info, err := entry.Info()
	if err != nil {
		return 0, err
	}

	return info.Size(), nil
}

func requireSampleFile(source fs.FS, path string) error {
	info, err := fs.Stat(source, path)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("missing regular file %s: %w", path, os.ErrInvalid)
	}

	return nil
}

func requireFile(path string) error {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()

	return requireSampleFile(root.FS(), filepath.Base(path))
}

func assemble(state, source, destination string, remove bool, openPRs []int) error {
	if !regexp.MustCompile(`^(main|pr/[1-9][0-9]*)$`).MatchString(destination) || (remove && destination == "main") {
		return fmt.Errorf("invalid destination: %w", os.ErrInvalid)
	}

	// Rooted operations confine preview writes, even if state contains links.
	root, err := os.OpenRoot(state)
	if err != nil {
		return err
	}
	defer root.Close()

	if info, err := root.Lstat("pr"); err == nil && !info.IsDir() {
		return fmt.Errorf("preview parent is not a directory: %w", os.ErrInvalid)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	if !remove {
		if err = replacePreview(root, source, destination); err != nil {
			return err
		}
	} else if err = root.RemoveAll(destination); err != nil {
		return err
	}

	return updatePreviews(root, openPRs)
}

func replacePreview(root *os.Root, source, destination string) error {
	incoming, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer incoming.Close()

	// Validate every untrusted entry before replacing a working preview.
	// Nothing from the artifact is ever executed.
	if err = validateSamples(incoming.FS()); err != nil {
		return err
	}

	if err = root.RemoveAll(destination); err != nil {
		return err
	}

	return copySamples(root, incoming.FS(), destination)
}

func copySamples(root *os.Root, source fs.FS, destination string) error {
	return fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		target := filepath.Join(destination, filepath.FromSlash(path))
		if entry.IsDir() {
			return root.MkdirAll(target, 0o755)
		}

		input, err := source.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()

		output, err := root.Create(target)
		if err != nil {
			return err
		}

		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()

		if copyErr != nil {
			return copyErr
		}

		return closeErr
	})
}

func updatePreviews(root *os.Root, openPRs []int) error {
	previews, err := fs.ReadDir(root.FS(), "pr")
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var links strings.Builder
	if err = requireSampleFile(root.FS(), "main/index.html"); err == nil {
		links.WriteString("<li><a href=\"main/\">Latest samples</a></li>")
	}

	for _, preview := range previews {
		number, err := strconv.Atoi(preview.Name())
		if err != nil || !preview.IsDir() {
			continue
		}

		if openPRs != nil && !slices.Contains(openPRs, number) {
			if err = root.RemoveAll(filepath.Join("pr", preview.Name())); err != nil {
				return err
			}

			continue
		}

		fmt.Fprintf(&links, "<li><a href=\"pr/%s/\">PR #%s</a></li>", preview.Name(), preview.Name())
	}

	return root.WriteFile("index.html", indexHTML("Latest demos and pull request previews.", links.String()), 0o600)
}

func writeIndex(directory, description, links string) error {
	return os.WriteFile(filepath.Join(directory, "index.html"), indexHTML(description, links), 0o600)
}

func indexHTML(description, links string) []byte {
	contents := `<!doctype html><html lang="en"><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<title>Karty samples</title><style>body{font:18px system-ui;max-width:48rem;` +
		`margin:4rem auto;padding:0 1rem;background:#101827;color:#eef2ff}` +
		`a{color:#91caff}li{margin:1rem 0}</style><h1>Karty samples</h1>` +
		"<p>" + description + "</p><ul>" + links + "</ul></html>"

	return []byte(contents)
}
