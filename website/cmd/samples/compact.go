package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/website/internal/generator"
)

// Compact an immutable website snapshot from existing complete sample builds.
// No compiler, baker, SDK installation or original distribution is changed.
func compactSamples(root, noticeDirectory string) error {
	notices, err := readCompactNotices(noticeDirectory)
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

	site := filepath.Join(root, "dist", "samples-site-compact-"+rand.Text())
	if err = os.MkdirAll(site, 0o750); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "Website snapshot:", site)

	for _, name := range names {
		source := filepath.Join(root, "samples", name, "dist", "web")
		if err = copyCompleteSampleWeb(source, filepath.Join(site, name), name); err != nil {
			return err
		}

		version, err := sampleVersion(filepath.Join(root, "samples", name))
		if err != nil {
			return err
		}

		if err = shareSampleRuntime(site, name, version); err != nil {
			return fmt.Errorf("sample %s: %w", name, err)
		}

		if err = copySamplePreview(filepath.Join(root, "samples", name), filepath.Join(site, name)); err != nil {
			return err
		}

		if len(notices) != 0 {
			if err = writeSampleNotices(filepath.Join(site, name), notices); err != nil {
				return err
			}
		}
	}

	if err = generator.GenerateSiteDocs(site); err != nil {
		return err
	}

	if err = writeGallery(site, cards); err != nil {
		return err
	}

	return validateSamples(os.DirFS(site))
}

func readCompactNotices(directory string) (map[string][]byte, error) {
	if directory == "" {
		return map[string][]byte{}, nil
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	notices := map[string][]byte{}

	for _, name := range []string{"RUNTIME_LICENSE.md", "SDK_LICENSE.md"} {
		info, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}

		if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxNoticeBytes {
			return nil, fmt.Errorf("candidate notice must be a bounded regular file: %w", os.ErrInvalid)
		}

		notices[name], err = root.ReadFile(name)
		if err != nil {
			return nil, err
		}
	}

	return notices, nil
}

func copyCompleteSampleWeb(source, destination, name string) error {
	parent, err := os.OpenRoot(filepath.Dir(source))
	if err != nil {
		return err
	}
	defer parent.Close()

	base := filepath.Base(source)

	info, err := parent.Lstat(base)
	if err != nil {
		return err
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("sample web source must be a real directory: %w", os.ErrInvalid)
	}

	root, err := parent.OpenRoot(base)
	if err != nil {
		return err
	}
	defer root.Close()

	if err := validateLocalSampleWeb(root, name); err != nil {
		return err
	}

	return os.CopyFS(destination, root.FS())
}
