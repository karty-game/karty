// Package levelbuild discovers author-owned level manifests and produces
// deterministic, passive .kld cartridges.
package levelbuild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-sdk/format/level"
	sdkworld "github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty-sdk/format/worldterrain"
	uicompiler "github.com/karty-game/karty-ui/compiler"
	ui "github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/actionbuild"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/packages"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/pelletier/go-toml/v2"
)

const maxLevelNameLength = 1024

var (
	ErrManifest      = errors.New("level manifest is invalid")
	ErrDuplicateName = errors.New("level logical name is duplicated")
	ErrSource        = errors.New("level data source is invalid")
)

type manifest struct {
	Level struct {
		Name     string `toml:"name"`
		Kind     string `toml:"kind"`
		Metadata string `toml:"metadata"`
	} `toml:"level"`
	Data     []dataEntry            `toml:"data"`
	World    worldSettings          `toml:"world"`
	Terrain  *terrainBuildSettings  `toml:"terrain"`
	Lightmap *lightmapBuildSettings `toml:"lightmap"`
	Chunks   *chunkBuildSettings    `toml:"chunks"`
	Textures []textureEntry         `toml:"textures"`
	UI       []dataEntry            `toml:"ui"`
	Theme    dataEntry              `toml:"theme"`
}

type dataEntry struct {
	Name   string `toml:"name"`
	Source string `toml:"source"`
}

type worldSettings struct {
	Name                   string   `toml:"name"`
	Source                 string   `toml:"source"`
	MaxSpriteTexelsPerUnit *float64 `toml:"max_sprite_texels_per_unit"`
}

type textureEntry struct {
	Generated              bool                 `toml:"-"`
	Imported               bool                 `toml:"-"`
	Name                   string               `toml:"name"`
	Source                 string               `toml:"source"`
	Profile                string               `toml:"profile"`
	Transform              textureTransform     `toml:"transform"`
	Materialize            *materializeSettings `toml:"materialize"`
	MaterialStrengths      *materialStrengths   `toml:"material_strengths"`
	Emission               *materialEmission    `toml:"emission"`
	MaxSpriteTexelsPerUnit *float64             `toml:"max_sprite_texels_per_unit"`
}

type textureTransform struct {
	MaxWidth  uint32 `toml:"max_width"`
	MaxHeight uint32 `toml:"max_height"`
	Filter    string `toml:"filter"`
	BitDepth  uint8  `toml:"bit_depth"`
}

type metadataTexture struct {
	ID       uint32 `json:"id"`
	Name     string `json:"name"`
	Width    int    `json:"-"`
	Height   int    `json:"-"`
	Coverage string `json:"-"`
}

// Artifact is a complete generated level-data cartridge.
type Artifact struct {
	// AuthoredActors lists all compiled content identities instantiated as actors.
	AuthoredActors  []string
	Name            string
	Kind            string
	SourceDirectory string
	Bytes           []byte
	ContentSHA256   string
	EnvelopeVersion uint16
	// Textures describes processed cache artifacts included in this level.
	Textures []assetpipeline.Texture
	// Features is the sorted runtime capability set required by this level.
	// The game manifest must include the union for every packaged level.
	Features []string
	// Chunks are render resources belonging to this logical level, never mounted
	// as additional game levels or authored actor instances.
	Chunks []ChunkArtifact
}

// ChunkArtifact is a content-addressed passive child cartridge. Its identity
// and exact byte length are bound by the parent level's chunk catalog.
type ChunkArtifact struct {
	ID              int
	Bytes           []byte
	ContentSHA256   string
	EnvelopeVersion uint16
}

type assetBuild struct {
	packages    *packages.Set
	projectRoot string
	manifest    sdk.Manifest
	screenshot  bool
	development bool
	// Tests may supply fixture atlas bytes; public builds always use the managed
	// Go asset processor, including the shared dev/cache path.
	materials func(context.Context, string, sdk.Manifest, sdkworld.Document, map[uint32][]byte, ...map[uint32]assetpipeline.MaterializeOptions) (worldmaterial.Pair, assetpipeline.Artifact, error)
}

// BuildAllWithAssets processes level textures with the selected SDK before
// packaging them. Its artifacts report runtime features for the game manifest.
func BuildAllWithAssets(
	ctx context.Context,
	projectDirectory string,
	uiSchema uint32,
	themeSource string,
	manifest sdk.Manifest,
) ([]Artifact, error) {
	return BuildAllWithOptions(ctx, projectDirectory, uiSchema, themeSource, manifest, Options{})
}

// Options controls whether a build may produce new chunk lighting.
type Options struct {
	Development bool
}

// BuildAllWithOptions processes assets while honoring the development policy.
// Development consumes a complete matching bake or builds mesh-only chunks.
func BuildAllWithOptions(
	ctx context.Context,
	projectDirectory string,
	uiSchema uint32,
	themeSource string,
	manifest sdk.Manifest,
	options Options,
) ([]Artifact, error) {
	return buildAll(ctx, projectDirectory, uiSchema, themeSource, &assetBuild{
		projectRoot: projectDirectory, manifest: manifest, development: options.Development,
	})
}

func buildAll(
	ctx context.Context,
	projectDirectory string,
	uiSchema uint32,
	themeSource string,
	assets *assetBuild,
) ([]Artifact, error) {
	set, err := packages.Restore(ctx, projectDirectory)
	if err != nil {
		return nil, err
	}
	defer set.Close()

	if assets != nil {
		assets.packages = set
	}

	levelsDirectory := filepath.Join(projectDirectory, "levels")

	entries, err := os.ReadDir(levelsDirectory)
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read levels directory: %w", err)
	}

	artifacts := make([]Artifact, 0, len(entries))
	theme := uicompiler.DefaultTheme()

	if themeSource != "" {
		contents, err := uicompiler.ReadSource(projectDirectory, themeSource)
		if err != nil {
			return nil, err
		}

		theme, err = uicompiler.ParseTheme(themeSource, contents)
		if err != nil {
			return nil, err
		}
	}

	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		directory := filepath.Join(levelsDirectory, entry.Name())
		// #nosec G703 -- directory is an immediate non-symlink child of the explicit project levels directory.
		if _, err := os.Stat(filepath.Join(directory, "level.toml")); os.IsNotExist(err) {
			continue
		}

		artifact, err := build(ctx, directory, uiSchema, theme, assets)
		if err != nil {
			return nil, err
		}

		if _, exists := seen[artifact.Name]; exists {
			return nil, fmt.Errorf("%q: %w", artifact.Name, ErrDuplicateName)
		}

		seen[artifact.Name] = struct{}{}
		artifacts = append(artifacts, artifact)
	}

	slices.SortFunc(artifacts, func(left, right Artifact) int { return strings.Compare(left.Name, right.Name) })

	return artifacts, nil
}

func build(
	ctx context.Context,
	directory string,
	uiSchema uint32,
	inheritedTheme uicompiler.Theme,
	assets *assetBuild,
) (Artifact, error) {
	definition, err := readLevelManifest(directory, assets)
	if err != nil {
		return Artifact{}, err
	}

	var set *packages.Set
	if assets != nil {
		set = assets.packages
	}

	if set == nil {
		set, err = levelPackages(ctx, directory)
		if err != nil {
			return Artifact{}, err
		}
		defer set.Close()
	}

	definition, err = resolveLevelPackages(ctx, directory, definition, set)
	if err != nil {
		return Artifact{}, err
	}

	definition, err = prepareLevelWorld(ctx, directory, definition, assets)
	if err != nil {
		return Artifact{}, err
	}

	definition, err = prepareSpriteDensity(directory, definition, assets)
	if err != nil {
		return Artifact{}, err
	}

	textures, textureMetadata, processedTextures, features, err := loadTextures(ctx, directory, definition.Textures, assets)
	if err != nil {
		return Artifact{}, err
	}

	metadata, err := loadMetadata(directory, definition, textureMetadata)
	if err != nil {
		return Artifact{}, err
	}

	data, err := loadData(directory, definition.Data)
	if err != nil {
		return Artifact{}, err
	}

	var chunks []ChunkArtifact

	if definition.World.Source != "" {
		atlas, materialErr := compileWorldAssets(ctx, directory, definition, textureMetadata, textures, data, metadata, assets)
		if materialErr != nil {
			return Artifact{}, materialErr
		}

		data, metadata = atlas.data, atlas.metadata
		textures = atlas.sources
		processedTextures = append(processedTextures, atlas.textures...)
		features = append(features, atlas.features...)
		chunks = atlas.chunks

		slices.Sort(features)
		features = slices.Compact(features)
	}

	data = append(data, textures...)

	theme, err := resolveTheme(directory, definition.Theme.Source, uiSchema, inheritedTheme, textureMetadata)
	if err != nil {
		return Artifact{}, err
	}

	for _, entry := range definition.UI {
		contents, err := readConfinedFile(directory, entry.Source, ui.MaxAssetBytes)
		if err != nil {
			return Artifact{}, err
		}

		template, err := project.DecodeUIWithTheme(entry.Source, contents, theme)
		if err != nil {
			return Artifact{}, err
		}

		if template.Version > uiSchema {
			return Artifact{}, fmt.Errorf("UI styles require SDK 0.0.1: %w", ErrManifest)
		}

		encoded, err := ui.EncodeComposition(template)
		if err != nil {
			return Artifact{}, err
		}

		if entry.Name == "" {
			return Artifact{}, ErrManifest
		}

		data = append(data, level.SourceEntry{Name: ui.AssetPrefix + entry.Name, Kind: level.EntryData, Data: encoded})
	}

	data, authoredActors, err := appendAuthoredActions(directory, data, assets)
	if err != nil {
		return Artifact{}, err
	}

	data, processedTextures, features, metadata, err = finalizeLevelTextures(
		ctx,
		data,
		processedTextures,
		textureMetadata,
		features,
		metadata,
		assets,
	)
	if err != nil {
		return Artifact{}, err
	}

	module, err := encodeLevelModule(definition.Level.Name, metadata, data)
	if err != nil {
		return Artifact{}, err
	}

	digest := sha256.Sum256(module)

	return Artifact{
		AuthoredActors:  authoredActors,
		Name:            definition.Level.Name,
		Kind:            definition.Level.Kind,
		SourceDirectory: directory,
		Bytes:           module,
		ContentSHA256:   hex.EncodeToString(digest[:]),
		EnvelopeVersion: level.EnvelopeVersion,
		Textures:        processedTextures,
		Features:        features,
		Chunks:          chunks,
	}, nil
}

func encodeLevelModule(name string, metadata []byte, data []level.SourceEntry) ([]byte, error) {
	envelope, err := level.Encode(metadata, data)
	if err != nil {
		return nil, fmt.Errorf("encode level %q: %w", name, err)
	}

	module, err := level.WrapModule(envelope)
	if err != nil {
		return nil, fmt.Errorf("wrap level %q: %w", name, err)
	}

	return module, nil
}

func resolveTheme(
	directory, source string,
	uiSchema uint32,
	inherited uicompiler.Theme,
	textures []metadataTexture,
) (uicompiler.Theme, error) {
	if source == "" {
		return inherited, nil
	}

	if uiSchema < ui.SchemaStyle {
		return uicompiler.Theme{}, fmt.Errorf("level theme requires SDK 0.0.1: %w", ErrManifest)
	}

	contents, err := readConfinedFile(directory, source, ui.MaxAssetBytes)
	if err != nil {
		return uicompiler.Theme{}, err
	}

	theme, err := uicompiler.ParseTheme(source, contents)
	if err != nil || len(theme.Images) == 0 {
		return theme, err
	}

	if uiSchema < ui.SchemaImageStyle {
		return uicompiler.Theme{}, fmt.Errorf("level theme images require SDK 0.0.1: %w", ErrManifest)
	}

	ids := make(map[string]uint32, len(textures))

	dimensions := make(map[string][2]int, len(textures))
	for _, texture := range textures {
		ids[texture.Name] = texture.ID
		dimensions[texture.Name] = [2]int{texture.Width, texture.Height}
	}

	return theme.ResolveLevelImages(ids, dimensions)
}

func loadMetadata(directory string, definition manifest, textures []metadataTexture) ([]byte, error) {
	metadata := level.Metadata{
		"schema": level.StringValue("karty.level@2"),
		"name":   level.StringValue(definition.Level.Name),
		"kind":   level.StringValue(definition.Level.Kind),
	}
	if definition.Level.Metadata != "" {
		contents, err := readConfinedFile(directory, definition.Level.Metadata, level.MaxMetadataSize)
		if err != nil {
			return nil, err
		}

		metadata, err = parseAuthoredMetadata(contents, definition.Level.Metadata)
		if err != nil {
			return nil, fmt.Errorf("parse level metadata: %w", err)
		}
	}
	// Only the selected SDK build pipeline may declare compiled material/terrain resources.
	delete(metadata, worldmaterial.MetadataKey)
	delete(metadata, worldterrain.MetadataKey)

	if definition.Terrain != nil {
		metadata[worldterrain.MetadataKey] = level.StringValue(worldterrain.Schema)
	}

	metadata["kartyLevel"] = level.ObjectValue(level.Metadata{
		"name":            level.StringValue(definition.Level.Name),
		"kind":            level.StringValue(definition.Level.Kind),
		"envelopeVersion": level.UintValue(uint64(level.EnvelopeVersion)),
	})

	values := make([]level.Value, 0, len(textures))
	for _, texture := range textures {
		values = append(
			values,
			level.ObjectValue(level.Metadata{"id": level.UintValue(uint64(texture.ID)), "name": level.StringValue(texture.Name)}),
		)
	}

	metadata["kartyTextures"] = level.ArrayValue(values)

	return level.EncodeMetadata(metadata)
}

func loadTextures(
	ctx context.Context,
	directory string,
	entries []textureEntry,
	assets *assetBuild,
) ([]level.SourceEntry, []metadataTexture, []assetpipeline.Texture, []string, error) {
	sorted := slices.Clone(entries)
	slices.SortFunc(sorted, func(left, right textureEntry) int {
		if left.Generated != right.Generated {
			if left.Generated {
				return 1
			}

			return -1
		}

		return strings.Compare(left.Name, right.Name)
	})
	result := make([]level.SourceEntry, 0, len(sorted))
	metadata := make([]metadataTexture, 0, len(sorted))
	processedTextures := make([]assetpipeline.Texture, 0, len(sorted))
	features := make([]string, 0, 1)

	previous := ""
	for index, entry := range sorted {
		if entry.Name == "" || entry.Source == "" || !utf8.ValidString(entry.Name) || entry.Name == previous {
			return nil, nil, nil, nil, fmt.Errorf("texture %q: %w", entry.Name, ErrSource)
		}

		contents, width, height, processed, err := loadProcessedTexture(ctx, directory, entry, assets)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("texture %q: %w", entry.Name, err)
		}

		if processed != nil {
			processedTextures = append(processedTextures, *processed)
			if processed.Processor == string(asset.ProcessorQOIv1) {
				features = append(features, cartridge.FeatureTextureQOIv1)
			}
		}

		assetID := uint32(index + 1)
		result = append(result, level.SourceEntry{Name: level.TextureEntryName(assetID), Kind: level.EntryTexture, Data: contents})
		metadata = append(metadata, metadataTexture{
			ID: assetID, Name: entry.Name, Width: width, Height: height,
		})
		previous = entry.Name
	}

	slices.Sort(features)
	features = slices.Compact(features)

	return result, metadata, processedTextures, features, nil
}

func loadProcessedTexture(
	ctx context.Context,
	directory string,
	entry textureEntry,
	assets *assetBuild,
) ([]byte, int, int, *assetpipeline.Texture, error) {
	// Level sources are confined to the level directory, even though processing
	// and its cache belong to the encompassing project.
	sourceDirectory := directory
	if entry.Generated {
		sourceDirectory = assets.projectRoot
	}

	if _, err := readConfinedFile(sourceDirectory, entry.Source, asset.MaxSourceAssetBytes); err != nil {
		return nil, 0, 0, nil, err
	}

	profileName := entry.Profile
	if profileName == "" {
		profileName = project.DefaultTextureProfile
	}

	profile, found := assets.manifest.Assets.TextureProfiles[profileName]
	if !found {
		return nil, 0, 0, nil, fmt.Errorf("profile %q: %w", profileName, ErrSource)
	}

	absoluteSource := filepath.Join(sourceDirectory, filepath.FromSlash(entry.Source))

	relativeSource, err := filepath.Rel(assets.projectRoot, absoluteSource)
	if err != nil || relativeSource == ".." || strings.HasPrefix(relativeSource, ".."+string(filepath.Separator)) {
		return nil, 0, 0, nil, ErrSource
	}

	processed, err := assetpipeline.ProcessTexture(
		ctx,
		assets.projectRoot,
		profile,
		project.Texture{
			Name: entry.Name, Source: filepath.ToSlash(relativeSource), Profile: profileName,
			Transform: project.TextureTransform{
				MaxWidth: entry.Transform.MaxWidth, MaxHeight: entry.Transform.MaxHeight,
				Filter: entry.Transform.Filter, BitDepth: entry.Transform.BitDepth,
			},
		},
	)
	if err != nil {
		return nil, 0, 0, nil, err
	}

	contents, err := readVerifiedProcessedAsset(processed.SourcePath, level.MaxEntrySize, processed.OutputSHA256)
	if err != nil {
		return nil, 0, 0, nil, fmt.Errorf("output: %w", err)
	}

	return contents, processed.Width, processed.Height, &processed, nil
}

func readVerifiedProcessedAsset(path string, maximum int, expectedSHA256 string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > int64(maximum) {
		return nil, ErrSource
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, ErrSource
	}

	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != expectedSHA256 {
		return nil, ErrSource
	}

	return contents, nil
}

func loadData(directory string, entries []dataEntry) ([]level.SourceEntry, error) {
	data := make([]level.SourceEntry, 0, len(entries))

	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.Name == "" || entry.Source == "" || !utf8.ValidString(entry.Name) {
			return nil, ErrSource
		}

		if entry.Name == "@world/vegetation" {
			return nil, fmt.Errorf("data %q is owned by world vegetation compilation: %w", entry.Name, ErrSource)
		}

		if entry.Name == worldparticles.EntryName {
			return nil, fmt.Errorf("data %q is owned by world particle compilation: %w", entry.Name, ErrSource)
		}

		if entry.Name == worldterrain.EntryName {
			return nil, fmt.Errorf("data %q is owned by world terrain compilation: %w", entry.Name, ErrSource)
		}

		if entry.Name == worldmaterial.LayoutEntry || entry.Name == worldmaterial.AlbedoEntry || entry.Name == worldmaterial.DataEntry ||
			entry.Name == worldmaterial.MipTailEntry || entry.Name == worldmaterial.ReflectanceEntry || entry.Name == worldmaterial.ReflectanceMipTailEntry {
			return nil, fmt.Errorf("data %q is owned by the world material processor: %w", entry.Name, ErrSource)
		}

		if _, exists := seen[entry.Name]; exists {
			return nil, fmt.Errorf("%q: %w", entry.Name, ErrDuplicateName)
		}

		contents, err := readConfinedFile(directory, entry.Source, level.MaxEntrySize)
		if err != nil {
			return nil, fmt.Errorf("data %q: %w", entry.Name, err)
		}

		seen[entry.Name] = struct{}{}
		data = append(data, level.SourceEntry{Name: entry.Name, Kind: level.EntryData, Data: contents})
	}

	return data, nil
}

func readConfinedFile(directory, relative string, maximum int) ([]byte, error) {
	if filepath.IsAbs(relative) || maximum < 1 {
		return nil, ErrSource
	}

	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, ErrSource
	}

	path, err := filepath.EvalSymlinks(filepath.Join(directory, filepath.FromSlash(relative)))
	if err != nil {
		return nil, ErrSource
	}

	confined, err := filepath.Rel(root, path)
	if err != nil || confined == ".." || strings.HasPrefix(confined, ".."+string(filepath.Separator)) {
		return nil, ErrSource
	}

	confinedRoot, err := os.OpenRoot(root)
	if err != nil {
		return nil, ErrSource
	}
	defer confinedRoot.Close()

	file, err := confinedRoot.Open(confined)
	if err != nil {
		return nil, ErrSource
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > int64(maximum) {
		return nil, ErrSource
	}

	contents, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil || len(contents) == 0 || len(contents) > maximum {
		return nil, ErrSource
	}

	return contents, nil
}

func appendAuthoredActions(directory string, data []level.SourceEntry, assets *assetBuild) ([]level.SourceEntry, []string, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()

	var authoredActors []string

	for _, entry := range data {
		if entry.Name == sdkworld.EntryName {
			document, decodeErr := sdkworld.Decode(entry.Data)
			if decodeErr != nil {
				return nil, nil, decodeErr
			}

			for _, content := range document.Contents {
				authoredActors = append(authoredActors, content.ID)
			}
		}
	}

	if _, err := root.Lstat("actions.json"); err == nil {
		return nil, nil, fmt.Errorf("actions.json is unsupported; migrate to actions.yaml: %w", ErrManifest)
	} else if !os.IsNotExist(err) {
		return nil, nil, err
	}

	if info, statErr := root.Lstat("actions.yaml"); statErr == nil {
		if !info.Mode().IsRegular() {
			return nil, nil, fmt.Errorf("actions.yaml must be a regular authored file: %w", ErrManifest)
		}

		compiled, err := authoredActionsEntry(directory, assets)
		if err != nil {
			return nil, nil, err
		}

		data = append(data, level.SourceEntry{Name: "karty/actions@1", Kind: level.EntryData, Data: compiled})
	} else if !os.IsNotExist(statErr) {
		return nil, nil, statErr
	}

	return data, authoredActors, nil
}

func authoredActionsEntry(directory string, assets *assetBuild) ([]byte, error) {
	if assets == nil || !assets.manifest.SupportsAuthoredActions() {
		return nil, fmt.Errorf("selected SDK does not support authored actions: %w", ErrManifest)
	}

	contents, err := readConfinedFile(directory, "actions.yaml", 64*1024)
	if err != nil {
		return nil, err
	}

	return actionbuild.CompileDocument(contents)
}

func readLevelManifest(directory string, assets *assetBuild) (manifest, error) {
	manifestPath := filepath.Join(directory, "level.toml")

	contents, err := readConfinedFile(directory, "level.toml", level.MaxMetadataSize)
	if err != nil {
		return manifest{}, fmt.Errorf("read %s: %w", manifestPath, err)
	}

	var definition manifest
	if err := toml.Unmarshal(contents, &definition); err != nil {
		return manifest{}, fmt.Errorf("parse %s: %w", manifestPath, err)
	}

	if definition.Level.Name == "" || len(definition.Level.Name) > maxLevelNameLength ||
		!utf8.ValidString(definition.Level.Name) || definition.Level.Kind == "" {
		return manifest{}, fmt.Errorf("%s: %w", manifestPath, ErrManifest)
	}

	if err := validateMaterializeKeys(contents); err != nil {
		return manifest{}, err
	}

	if err := validateEnvironmentLightmapKeys(contents); err != nil {
		return manifest{}, err
	}

	if err := validateMaterializeSettings(definition, assets); err != nil {
		return manifest{}, err
	}

	if err := validateMaterialEmission(definition, assets); err != nil {
		return manifest{}, err
	}

	if err := validateMaterialStrengths(definition, assets); err != nil {
		return manifest{}, err
	}

	if err := validateLightmapSettings(definition, assets); err != nil {
		return manifest{}, err
	}

	if err := validateChunkSettings(definition, assets); err != nil {
		return manifest{}, err
	}

	if err := validateTerrainSettings(definition, assets); err != nil {
		return manifest{}, err
	}

	if err := appendProcessorTextures(directory, &definition, assets); err != nil {
		return manifest{}, err
	}

	return definition, nil
}
