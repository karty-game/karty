package levelbuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/pelletier/go-toml/v2"
)

const developmentBakeWorkers = 2

// BakeDevelopment refreshes enabled automatic whole-level bakes before a live
// development snapshot is published. Matching complete bakes are reused. Chunk
// and environment lighting is produced by the ordinary level builder instead.
// Projects without automatic lightmaps need no preparation and succeed.
func BakeDevelopment(ctx context.Context, projectDirectory string, completed func(BakeReport)) ([]BakeReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(filepath.Join(projectDirectory, "levels"))
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("discover development bake levels: %w", err)
	}

	var reports []BakeReport

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return reports, err
		}

		if !entry.IsDir() {
			continue
		}

		directory := filepath.Join(projectDirectory, "levels", entry.Name())

		definition, selected, err := developmentBakeDefinition(directory)
		if err != nil {
			return reports, err
		}

		if !selected {
			continue
		}

		report, err := bakeLevel(
			ctx, projectDirectory, directory, definition,
			BakeOptions{Workers: developmentBakeWorkers, Incremental: true},
		)
		if err != nil {
			return reports, fmt.Errorf("bake level %q: %w", definition.Level.Name, err)
		}

		reports = append(reports, report)
		if completed != nil {
			completed(report)
		}
	}

	return reports, nil
}

func developmentBakeDefinition(directory string) (manifest, bool, error) {
	contents, err := readConfinedFile(directory, "level.toml", level.MaxMetadataSize)
	if err != nil {
		if _, statErr := os.Stat(filepath.Join(directory, "level.toml")); os.IsNotExist(statErr) {
			return manifest{}, false, nil
		}

		return manifest{}, false, err
	}

	var definition manifest
	if err := toml.Unmarshal(contents, &definition); err != nil {
		return manifest{}, false, fmt.Errorf("parse %s: %w", directory, err)
	}

	settings := definition.Lightmap
	if settings == nil || !settings.Enabled || !settings.Offline ||
		settings.Prebake != nil || settings.PrebakeImage != nil || settings.Environment != nil ||
		(definition.Chunks != nil && definition.Chunks.Enabled) {
		return manifest{}, false, nil
	}

	if definition.World.Source == "" {
		return manifest{}, false, fmt.Errorf("level %q needs a world source for offline lightmaps: %w", definition.Level.Name, ErrManifest)
	}

	return definition, true, nil
}
