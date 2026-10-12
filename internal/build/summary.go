package build

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/toolchain"
)

// Summary describes the completed output directory, including its runtime.
// Bytes are on-disk bytes, not decoded memory or compressed download sizes.
type Summary struct {
	Project, SDK, Target, Platform, Directory string
	HostURL                                   string
	Levels, Chunks, Files                     int
	Textures, Sounds, Streams                 int
	Bytes, CartridgeBytes                     int64
	Duration                                  time.Duration
}

func summarizeBuild(directory, name, version string, levels []levelbuild.Artifact,
	assets assetpipeline.Report, options Options,
) (Summary, error) {
	target := options.Target
	if target == "" {
		target = "raw"

		if options.Host != "" {
			target = "native"
		}
	}

	output := filepath.Join("dist", target)
	if options.Platform != "" {
		output = filepath.Join(output, options.Platform)
	}

	report := Summary{
		Project: name, SDK: version, Target: target, Platform: options.Platform,
		Directory: filepath.Join(directory, output), Levels: len(levels),
		Textures: len(assets.Textures) + len(assets.Levels), Sounds: len(assets.Sounds), Streams: len(assets.AudioStreams),
	}
	if options.cdnHost != nil {
		report.HostURL = options.cdnHost.URL
	}

	for _, level := range levels {
		report.Chunks += len(level.Chunks)
	}

	projectRoot, err := os.OpenRoot(directory)
	if err != nil {
		return Summary{}, err
	}
	defer projectRoot.Close()

	root, err := projectRoot.OpenRoot(output)
	if err != nil {
		return Summary{}, err
	}
	defer root.Close()

	info, err := root.Stat("game.kart")
	if err != nil {
		return Summary{}, err
	}

	report.CartridgeBytes = info.Size()
	err = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			// Default native output shares a parent with explicit platform builds.
			if target == "native" && options.Platform == "" && path == entry.Name() {
				if _, platformErr := toolchain.NativePlatform(entry.Name()); platformErr == nil {
					return fs.SkipDir
				}
			}

			return nil
		}

		if !entry.Type().IsRegular() {
			return fmt.Errorf("output entry %s is not a regular file: %w", path, os.ErrInvalid)
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		report.Files++
		report.Bytes += info.Size()

		return nil
	})

	return report, err
}
