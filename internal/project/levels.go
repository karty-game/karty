package project

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"unicode/utf8"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty/internal/contextio"
	"github.com/pelletier/go-toml/v2"
)

const maxLevelNameLength = 1024

const (
	errInvalidLevelName   staticError = "level manifest requires a valid logical name and kind"
	errDuplicateLevelName staticError = "level logical name is duplicated"
)

// LevelNames discovers logical names without compiling worlds or reading assets.
// Only immediate level directories participate, matching the level builder.
func LevelNames(ctx context.Context, directory string) ([]string, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	entries, err := fs.ReadDir(contextio.Filesystem(ctx, root.FS()), "levels")
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var names []string

	seen := make(map[string]bool)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name, err := readLevelName(ctx, root, path.Join("levels", entry.Name(), "level.toml"))
		if os.IsNotExist(err) {
			continue
		}

		if err != nil {
			return nil, err
		}

		if seen[name] {
			return nil, fmt.Errorf("%q: %w", name, errDuplicateLevelName)
		}

		seen[name] = true
		names = append(names, name)
	}

	slices.Sort(names)

	return names, nil
}

func readLevelName(ctx context.Context, root *os.Root, filename string) (string, error) {
	// Match the level builder's confinement: a manifest stays inside its own level.
	directory, err := root.OpenRoot(path.Dir(filename))
	if err != nil {
		return "", err
	}
	defer directory.Close()

	file, err := directory.Open(path.Base(filename))
	if err != nil {
		return "", err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", err
	}

	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > level.MaxMetadataSize {
		return "", fmt.Errorf("%s: %w", filename, errInvalidLevelName)
	}

	contents, err := contextio.ReadAll(ctx, io.LimitReader(file, level.MaxMetadataSize+1))
	if err != nil {
		return "", err
	}

	if len(contents) > level.MaxMetadataSize {
		return "", fmt.Errorf("%s: %w", filename, errInvalidLevelName)
	}

	var manifest struct {
		Level struct{ Name, Kind string }
	}
	if err := toml.Unmarshal(contents, &manifest); err != nil {
		return "", fmt.Errorf("%s: %w", filename, err)
	}

	name := manifest.Level.Name
	if name == "" || len(name) > maxLevelNameLength || !utf8.ValidString(name) || manifest.Level.Kind == "" {
		return "", fmt.Errorf("%s: %w", filename, errInvalidLevelName)
	}

	return name, nil
}
