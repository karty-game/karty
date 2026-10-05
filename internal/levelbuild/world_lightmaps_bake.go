package levelbuild

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/pelletier/go-toml/v2"
)

const bakeTextureChannels = 4

const automaticBakeDirectory = ".karty/bakes"
const automaticBakeManifest = automaticBakeDirectory + "/lightmap-prebake.json"

// BakeOptions controls an explicit offline invocation, without compiling game
// code or installing a host, SDK bundle, material generator or GPU toolchain.
type BakeOptions struct {
	Level   string
	Samples *int
	Bounces *int
	Workers int
}

// BakeReport describes one completed level. Artifacts remain generated cache
// files until a normal build packages a current pair.
type BakeReport struct {
	Name      string
	Manifest  string
	Image     string
	Duration  time.Duration
	Stats     worldlightmapbake.Stats
	Automatic bool
}

// BakeAll bakes enabled directional recipes. Level accepts a logical name or
// directory name; an empty selector means all enabled world levels.
func BakeAll(ctx context.Context, projectDirectory string, options BakeOptions, completed func(BakeReport)) ([]BakeReport, error) {
	entries, err := os.ReadDir(filepath.Join(projectDirectory, "levels"))
	if err != nil {
		return nil, fmt.Errorf("discover bake levels: %w", err)
	}

	var reports []BakeReport

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		directory := filepath.Join(projectDirectory, "levels", entry.Name())

		contents, err := readConfinedFile(directory, "level.toml", level.MaxMetadataSize)
		if err != nil {
			if _, statErr := os.Stat(filepath.Join(directory, "level.toml")); os.IsNotExist(statErr) {
				continue
			}

			return reports, err
		}

		var definition manifest
		if err := toml.Unmarshal(contents, &definition); err != nil {
			return reports, fmt.Errorf("parse %s: %w", directory, err)
		}

		if options.Level != "" && options.Level != entry.Name() && options.Level != definition.Level.Name {
			continue
		}

		if definition.Lightmap == nil || !definition.Lightmap.Enabled {
			if options.Level != "" {
				return reports, fmt.Errorf("level %q has no enabled lightmap recipe: %w", definition.Level.Name, ErrManifest)
			}

			continue
		}

		if definition.World.Source == "" || len(definition.Lightmap.Lights) == 0 {
			return reports, fmt.Errorf("level %q needs a world source and lightmap lights list: %w", definition.Level.Name, ErrManifest)
		}

		if err := ctx.Err(); err != nil {
			return reports, err
		}

		report, err := bakeLevel(ctx, directory, definition, options)
		if err != nil {
			return reports, fmt.Errorf("bake level %q: %w", definition.Level.Name, err)
		}

		reports = append(reports, report)
		if completed != nil {
			completed(report)
		}
	}

	if len(reports) == 0 {
		return nil, fmt.Errorf("no enabled directional lightmap levels match %q: %w", options.Level, ErrManifest)
	}

	return reports, nil
}

func loadBakeMaterials(directory string, definition manifest) ([]metadataTexture, []worldlightmapbake.Material, error) {
	entries := slices.Clone(definition.Textures)
	slices.SortFunc(entries, func(a, b textureEntry) int { return strings.Compare(a.Name, b.Name) })
	metadata := make([]metadataTexture, 0, len(entries))
	materials := make([]worldlightmapbake.Material, 0, len(entries))
	decodedBytes := int64(0)

	for index, entry := range entries {
		if entry.Name == "" || index > 0 && entry.Name == entries[index-1].Name {
			return nil, nil, ErrSource
		}

		contents, err := readConfinedFile(directory, entry.Source, asset.MaxSourceAssetBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("material %q: %w", entry.Name, err)
		}

		config, _, err := image.DecodeConfig(bytes.NewReader(contents))
		if err != nil || config.Width < 1 || config.Height < 1 ||
			int64(config.Width)*int64(config.Height) > asset.MaxDecodedTextureBytes/bakeTextureChannels {
			return nil, nil, fmt.Errorf("material %q dimensions: %w", entry.Name, ErrSource)
		}

		decodedBytes += int64(config.Width) * int64(config.Height) * bakeTextureChannels
		if decodedBytes > asset.MaxDecodedTextures {
			return nil, nil, fmt.Errorf("bake source texture budget: %w", ErrSource)
		}

		pixels, _, err := image.Decode(bytes.NewReader(contents))
		if err != nil {
			return nil, nil, fmt.Errorf("material %q: %w", entry.Name, err)
		}

		id := uint32(index + 1)
		metadata = append(metadata, metadataTexture{ID: id, Name: entry.Name, Width: config.Width, Height: config.Height})
		materials = append(materials, worldlightmapbake.Material{ID: id, Albedo: pixels})
	}

	return metadata, materials, nil
}

func bakeLevel(ctx context.Context, directory string, definition manifest, options BakeOptions) (BakeReport, error) {
	started := time.Now()

	cache, err := prepareBakeDirectory(directory)
	if err != nil {
		return BakeReport{}, err
	}

	metadata, materials, err := loadBakeMaterials(directory, definition)
	if err != nil {
		return BakeReport{}, err
	}

	document, _, err := compileMaterialWorld(directory, definition, metadata)
	if err != nil {
		return BakeReport{}, err
	}

	layoutOptions, err := definition.Lightmap.options()
	if err != nil {
		return BakeReport{}, err
	}

	layout, err := worldlightmap.Compile(document, layoutOptions)
	if err != nil {
		return BakeReport{}, err
	}

	samples, bounces := 16, 1
	if definition.Lightmap.BakeSamples != nil {
		samples = *definition.Lightmap.BakeSamples
	}

	if definition.Lightmap.BakeBounces != nil {
		bounces = *definition.Lightmap.BakeBounces
	}

	if options.Samples != nil {
		samples = *options.Samples
	}

	if options.Bounces != nil {
		bounces = *options.Bounces
	}

	log.Info("Baking directional lightmap", "level_name", definition.Level.Name, "samples", samples, "bounces", bounces)
	bakeOptions := worldlightmapbake.Options{Samples: samples, Bounces: bounces, Workers: options.Workers, Seed: 1}

	result, err := worldlightmapbake.Bake(ctx, document, layout, materials, bakeOptions)
	if err != nil {
		return BakeReport{}, err
	}

	encoded, _, err := qoi.Encode(result.Image, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		return BakeReport{}, err
	}
	// The versioned offline manifest also binds surface reflectance and transport
	// settings; direct-only imported artifacts retain their original contract.
	pair, err := worldlightmap.NewOfflinePrebake(layout, &document, encoded, worldlightmap.OfflineBakeInputs{
		ReflectanceSHA256: result.ReflectanceSHA256, Samples: samples, Bounces: bounces, Seed: 1, RGBMRange: result.RGBMRange,
	})
	if err != nil {
		return BakeReport{}, err
	}

	manifest, err := worldlightmap.EncodePrebake(pair, layout, &document)
	if err != nil {
		return BakeReport{}, err
	}

	if err := ctx.Err(); err != nil {
		return BakeReport{}, err
	}

	imageName := "lightmap-" + pair.Manifest.ImageSHA256 + ".qoi"
	if err := atomicBakeWrite(cache, imageName, encoded); err != nil {
		return BakeReport{}, err
	}
	// Publish the manifest last. A concurrent build can only see a complete
	// content-addressed image or fall back to the runtime recipe.
	if err := atomicBakeWrite(cache, "lightmap-prebake.json", manifest); err != nil {
		return BakeReport{}, err
	}

	return BakeReport{
		Name:      definition.Level.Name,
		Manifest:  filepath.Join(cache, "lightmap-prebake.json"),
		Image:     filepath.Join(cache, imageName),
		Duration:  time.Since(started),
		Stats:     result.Stats,
		Automatic: definition.Lightmap.Offline,
	}, nil
}

func prepareBakeDirectory(directory string) (string, error) {
	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return "", err
	}

	current := root
	for _, name := range []string{".karty", "bakes"} {
		current = filepath.Join(current, name)
		if err := os.Mkdir(current, 0o755); err != nil && !os.IsExist(err) {
			return "", err
		}

		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("bake cache must be a confined directory: %w", ErrSource)
		}
	}

	return current, nil
}

func atomicBakeWrite(directory, name string, data []byte) error {
	file, err := os.CreateTemp(directory, ".bake-*")
	if err != nil {
		return err
	}

	path := file.Name()
	defer os.Remove(path)

	if _, err := file.Write(data); err != nil {
		file.Close()

		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	return os.Rename(path, filepath.Join(directory, name))
}

func readAutomaticPrebake(
	directory string,
	definition manifest,
	layout worldlightmap.Layout,
	document world.Document,
) (entries []level.SourceEntry) {
	defer func() {
		if len(entries) == 0 {
			log.Info("Offline lightmap unavailable or stale; using runtime bake", "level_name", definition.Level.Name)
		}
	}()

	encoded, err := readConfinedFile(directory, automaticBakeManifest, worldlightmap.MaxPrebakeManifestSize)
	if err != nil {
		return nil
	}

	var header worldlightmap.PrebakeManifest
	if json.Unmarshal(encoded, &header) != nil || len(header.ImageSHA256) != 64 {
		return nil
	}
	// The digest is used in a path only after strict lowercase hex validation.
	for _, c := range header.ImageSHA256 {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return nil
		}
	}

	imagePath := automaticBakeDirectory + "/lightmap-" + header.ImageSHA256 + ".qoi"

	image, err := readConfinedFile(directory, imagePath, worldlightmap.MaxPrebakeImageSize)
	if err != nil {
		return nil
	}

	pair, err := worldlightmap.DecodePrebake(encoded, image, layout, &document)
	if err != nil {
		return nil
	}

	if pair.Manifest.Algorithm != worldlightmap.OfflinePrebakeAlgorithm || pair.Manifest.Seed != 1 ||
		validateOfflineReflectance(directory, definition, document, pair.Manifest.ReflectanceSHA256) != nil {
		return nil
	}

	if expected := definition.Lightmap.BakeSamples; expected != nil && pair.Manifest.Samples != *expected {
		return nil
	}

	if expected := definition.Lightmap.BakeBounces; expected != nil && pair.Manifest.Bounces != *expected {
		return nil
	}

	return []level.SourceEntry{
		{Name: worldlightmap.PrebakeEntryName, Kind: level.EntryData, Data: encoded},
		{Name: worldlightmap.PrebakeImageEntryName, Kind: level.EntryData, Data: image},
	}
}

func validateOfflineReflectance(directory string, definition manifest, document world.Document, expected string) error {
	_, materials, err := loadBakeMaterials(directory, definition)
	if err != nil {
		return err
	}

	actual, err := worldlightmapbake.ReflectanceDigest(document, materials)
	if err != nil {
		return err
	}

	if actual != expected {
		return fmt.Errorf("original albedo or surface inputs changed: %w", worldlightmap.ErrLayout)
	}

	return nil
}
