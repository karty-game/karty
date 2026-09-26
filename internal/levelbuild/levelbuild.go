// Package levelbuild discovers author-owned level manifests and produces
// deterministic, passive .kld cartridges.
package levelbuild

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty-ui/schema"
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
	Data     []dataEntry    `toml:"data"`
	Textures []textureEntry `toml:"textures"`
	UI       []dataEntry    `toml:"ui"`
	Theme    dataEntry      `toml:"theme"`
}

type dataEntry struct {
	Name   string `toml:"name"`
	Source string `toml:"source"`
}

type textureEntry struct {
	Name   string `toml:"name"`
	Source string `toml:"source"`
}

type metadataTexture struct {
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	Width  int    `json:"-"`
	Height int    `json:"-"`
}

type metadataIdentity struct {
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	EnvelopeVersion uint16 `json:"envelopeVersion"`
}

// Artifact is a complete generated level-data cartridge.
type Artifact struct {
	Name            string
	Kind            string
	SourceDirectory string
	Bytes           []byte
	ContentSHA256   string
	EnvelopeVersion uint16
}

// BuildAll discovers levels/*/level.toml and builds them in logical-name order.
func BuildAll(projectDirectory string) ([]Artifact, error) {
	return BuildAllWithStyles(projectDirectory, false)
}

func BuildAllWithStyles(projectDirectory string, allowStyles bool) ([]Artifact, error) {
	if allowStyles {
		return BuildAllWithTheme(projectDirectory, 3, "")
	}

	return BuildAllWithTheme(projectDirectory, 2, "")
}

func BuildAllWithTheme(projectDirectory string, uiSchema uint32, themeSource string) ([]Artifact, error) {
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
		if _, err := os.Stat(filepath.Join(directory, "level.toml")); os.IsNotExist(err) {
			continue
		}

		artifact, err := build(directory, uiSchema, theme)
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

func build(directory string, uiSchema uint32, inheritedTheme uicompiler.Theme) (Artifact, error) {
	manifestPath := filepath.Join(directory, "level.toml")

	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		return Artifact{}, fmt.Errorf("read %s: %w", manifestPath, err)
	}

	var definition manifest
	if err := toml.Unmarshal(contents, &definition); err != nil {
		return Artifact{}, fmt.Errorf("parse %s: %w", manifestPath, err)
	}

	if definition.Level.Name == "" || len(definition.Level.Name) > maxLevelNameLength ||
		!utf8.ValidString(definition.Level.Name) || definition.Level.Kind == "" {
		return Artifact{}, fmt.Errorf("%s: %w", manifestPath, ErrManifest)
	}

	textures, textureMetadata, err := loadTextures(directory, definition.Textures)
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

		template, err := uicompiler.DecodeSourceWithTheme(entry.Source, contents, theme)
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

	envelope, err := level.Encode(metadata, data)
	if err != nil {
		return Artifact{}, fmt.Errorf("encode level %q: %w", definition.Level.Name, err)
	}

	module, err := level.WrapModule(envelope)
	if err != nil {
		return Artifact{}, fmt.Errorf("wrap level %q: %w", definition.Level.Name, err)
	}

	digest := sha256.Sum256(module)

	return Artifact{
		Name:            definition.Level.Name,
		Kind:            definition.Level.Kind,
		SourceDirectory: directory,
		Bytes:           module,
		ContentSHA256:   hex.EncodeToString(digest[:]),
		EnvelopeVersion: level.EnvelopeVersion,
	}, nil
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
	identity := metadataIdentity{
		Name: definition.Level.Name, Kind: definition.Level.Kind, EnvelopeVersion: level.EnvelopeVersion,
	}

	if definition.Level.Metadata != "" {
		contents, err := readConfinedFile(directory, definition.Level.Metadata, level.MaxMetadataSize)
		if err != nil {
			return nil, err
		}

		var metadata map[string]any
		if err := json.Unmarshal(contents, &metadata); err != nil {
			return nil, fmt.Errorf("parse level metadata: %w", err)
		}

		metadata["kartyTextures"] = textures
		metadata["kartyLevel"] = identity

		return json.Marshal(metadata)
	}

	metadata := struct {
		Schema   string            `json:"schema"`
		Name     string            `json:"name"`
		Kind     string            `json:"kind"`
		Level    metadataIdentity  `json:"kartyLevel"`
		Textures []metadataTexture `json:"kartyTextures"`
	}{
		Schema: "karty.level@1", Name: definition.Level.Name, Kind: definition.Level.Kind,
		Level: identity, Textures: textures,
	}

	contents, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("encode level metadata: %w", err)
	}

	return contents, nil
}

func loadTextures(directory string, entries []textureEntry) ([]level.SourceEntry, []metadataTexture, error) {
	sorted := slices.Clone(entries)
	slices.SortFunc(sorted, func(left, right textureEntry) int { return strings.Compare(left.Name, right.Name) })
	result := make([]level.SourceEntry, 0, len(sorted))
	metadata := make([]metadataTexture, 0, len(sorted))

	previous := ""
	for index, entry := range sorted {
		if entry.Name == "" || entry.Source == "" || !utf8.ValidString(entry.Name) || entry.Name == previous {
			return nil, nil, fmt.Errorf("texture %q: %w", entry.Name, ErrSource)
		}

		contents, err := readConfinedFile(directory, entry.Source, level.MaxEntrySize)
		if err != nil {
			return nil, nil, fmt.Errorf("texture %q: %w", entry.Name, err)
		}

		configuration, format, err := image.DecodeConfig(bytes.NewReader(contents))
		if err != nil || format != "png" || configuration.Width < 1 || configuration.Height < 1 {
			return nil, nil, fmt.Errorf("texture %q: %w", entry.Name, ErrSource)
		}

		assetID := uint32(index + 1)
		result = append(result, level.SourceEntry{Name: level.TextureEntryName(assetID), Kind: level.EntryTexture, Data: contents})
		metadata = append(metadata, metadataTexture{
			ID: assetID, Name: entry.Name, Width: configuration.Width, Height: configuration.Height,
		})
		previous = entry.Name
	}

	return result, metadata, nil
}

func loadData(directory string, entries []dataEntry) ([]level.SourceEntry, error) {
	data := make([]level.SourceEntry, 0, len(entries))

	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.Name == "" || entry.Source == "" || !utf8.ValidString(entry.Name) {
			return nil, ErrSource
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
	if filepath.IsAbs(relative) {
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

	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > int64(maximum) {
		return nil, ErrSource
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, ErrSource
	}

	return contents, nil
}
