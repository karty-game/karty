// Package assetpipeline inspects and describes deterministic runtime assets.
package assetpipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
)

const (
	SchemaVersion     = 1
	TextureProcessor  = "copy-png@1"
	bytesPerRGBApixel = 4
)

var errTextureDimensions = errors.New("texture dimensions cannot be represented safely")
var errUnknownTextureProfile = errors.New("texture profile is not defined by the selected SDK")
var errUnsupportedTextureProcessor = errors.New("texture processor is not supported by this CLI")
var errCacheContent = errors.New("cached texture content does not match its declared hash")

// GeneratedNotice makes build-report ownership explicit without relying on
// comments, which JSON cannot contain.
type GeneratedNotice struct {
	Warning       string `json:"warning"`
	DoNotEdit     string `json:"doNotEdit"`
	SourceOfTruth string `json:"sourceOfTruth"`
	Regenerate    string `json:"regenerate"`
}

// Texture describes a validated texture and its deterministic output identity.
type Texture struct {
	Name                  string `json:"name"`
	Profile               string `json:"profile"`
	Source                string `json:"source"`
	Output                string `json:"cacheOutput"`
	ContentSHA256         string `json:"contentSha256"`
	CacheKey              string `json:"cacheKey"`
	Processor             string `json:"processor"`
	Width                 int    `json:"width"`
	Height                int    `json:"height"`
	SourceBytes           int64  `json:"sourceBytes"`
	OutputBytes           int64  `json:"outputBytes"`
	EstimatedDecodedBytes int64  `json:"estimatedDecodedBytes"`
	CacheHit              bool   `json:"cacheHit"`
	SourcePath            string `json:"-"`
}

// PopulateCache verifies or atomically creates project-owned processor output.
func PopulateCache(directory string, report *Report) error {
	cacheDirectory := filepath.Join(directory, ".karty", "cache", "assets", fmt.Sprintf("v%d", SchemaVersion))
	if err := os.MkdirAll(cacheDirectory, 0o750); err != nil {
		return fmt.Errorf("create asset cache: %w", err)
	}

	for index := range report.Textures {
		texture := &report.Textures[index]
		cachePath := filepath.Join(cacheDirectory, texture.CacheKey+".png")

		valid, err := validCacheEntry(cachePath, texture.ContentSHA256)
		if err != nil {
			return fmt.Errorf("verify texture %q cache: %w", texture.Name, err)
		}

		if valid {
			texture.CacheHit = true
			texture.SourcePath = cachePath

			continue
		}

		if err := writeCacheEntry(cacheDirectory, cachePath, texture.SourcePath, texture.ContentSHA256); err != nil {
			return fmt.Errorf("cache texture %q: %w", texture.Name, err)
		}

		texture.CacheHit = false
		texture.SourcePath = cachePath
	}

	return nil
}

func validCacheEntry(path, expectedHash string) (bool, error) {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	digest := sha256.Sum256(contents)

	return hex.EncodeToString(digest[:]) == expectedHash, nil
}

func writeCacheEntry(directory, destination, source, expectedHash string) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != expectedHash {
		return errCacheContent
	}

	temporary, err := os.CreateTemp(directory, ".asset-")
	if err != nil {
		return err
	}

	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()

		return err
	}

	if err := temporary.Sync(); err != nil {
		temporary.Close()

		return err
	}

	if err := temporary.Close(); err != nil {
		return err
	}

	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(temporaryPath, destination); err != nil {
		if valid, validationErr := validCacheEntry(destination, expectedHash); validationErr == nil && valid {
			return nil
		}

		return err
	}

	return nil
}

// Summary records encoded and estimated decoded texture costs separately.
type Summary struct {
	TextureCount          int   `json:"textureCount"`
	DeclaredTextureCount  int   `json:"declaredTextureCount"`
	UsedTextureCount      int   `json:"usedTextureCount"`
	KeptTextureCount      int   `json:"keptTextureCount"`
	StrippedTextureCount  int   `json:"strippedTextureCount"`
	SourceBytes           int64 `json:"sourceBytes"`
	OutputBytes           int64 `json:"outputBytes"`
	StrippedSourceBytes   int64 `json:"strippedSourceBytes"`
	EstimatedDecodedBytes int64 `json:"estimatedDecodedBytes"`
	DeclaredFontCount     int   `json:"declaredFontCount"`
	UsedFontCount         int   `json:"usedFontCount"`
	StrippedFontCount     int   `json:"strippedFontCount"`
	FontSourceBytes       int64 `json:"fontSourceBytes"`
	StrippedFontBytes     int64 `json:"strippedFontBytes"`
}

// Usage explains why each declared texture was retained or stripped.
type Usage struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	SourceBytes int64  `json:"sourceBytes"`
}

// FontUsage explains whether a declared semantic font role was embedded.
type FontUsage struct {
	Role        string `json:"role"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	SourceBytes int64  `json:"sourceBytes"`
}

// Report is the deterministic asset inspection output for a build.
type Report struct {
	Generated GeneratedNotice `json:"generated"`
	Schema    int             `json:"schema"`
	SDK       string          `json:"sdk"`
	Summary   Summary         `json:"summary"`
	Usage     []Usage         `json:"usage"`
	Fonts     []FontUsage     `json:"fonts"`
	Textures  []Texture       `json:"textures"`
}

// AnalyzeTextures validates PNG inputs and computes content-addressed outputs.
// The first processor deliberately preserves PNG bytes until measurements
// justify adding a pinned optimizer.
func AnalyzeTextures(directory string, manifest sdk.Manifest, declarations []project.Texture) (Report, error) {
	report := Report{
		Generated: GeneratedNotice{
			Warning:       "GENERATED FILE. DO NOT EDIT.",
			DoNotEdit:     "MANUAL CHANGES WILL BE OVERWRITTEN.",
			SourceOfTruth: "karty.toml, declared asset files, and the selected Karty SDK",
			Regenerate:    "karty build",
		},
		Schema:   SchemaVersion,
		SDK:      manifest.Version,
		Textures: make([]Texture, 0, len(declarations)),
	}

	for _, declaration := range declarations {
		profile, exists := manifest.Assets.TextureProfiles[declaration.Profile]
		if !exists {
			return Report{}, fmt.Errorf("texture %q profile %q: %w", declaration.Name, declaration.Profile, errUnknownTextureProfile)
		}

		if profile.Processor != TextureProcessor {
			return Report{}, fmt.Errorf("texture %q processor %q: %w", declaration.Name, profile.Processor, errUnsupportedTextureProcessor)
		}

		texture, err := analyzeTexture(directory, manifest.Version, profile.Processor, declaration)
		if err != nil {
			return Report{}, err
		}

		report.Textures = append(report.Textures, texture)
		report.Summary.SourceBytes += texture.SourceBytes
		report.Summary.OutputBytes += texture.OutputBytes
		report.Summary.EstimatedDecodedBytes += texture.EstimatedDecodedBytes
	}

	report.Summary.TextureCount = len(report.Textures)

	return report, nil
}

func analyzeTexture(directory, sdkVersion, processor string, declaration project.Texture) (Texture, error) {
	sourcePath := filepath.Join(directory, filepath.FromSlash(declaration.Source))

	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		return Texture{}, fmt.Errorf("read texture %q: %w", declaration.Name, err)
	}

	configuration, err := png.DecodeConfig(bytes.NewReader(contents))
	if err != nil {
		return Texture{}, fmt.Errorf("decode texture %q as PNG: %w", declaration.Name, err)
	}

	width := int64(configuration.Width)

	height := int64(configuration.Height)
	if width < 1 || height < 1 || width > math.MaxInt64/bytesPerRGBApixel/height {
		return Texture{}, fmt.Errorf("texture %q: %w", declaration.Name, errTextureDimensions)
	}

	contentDigest := sha256.Sum256(contents)
	contentHash := hex.EncodeToString(contentDigest[:])
	cacheDigest := sha256.Sum256([]byte(fmt.Sprintf(
		"schema=%d\x00sdk=%s\x00kind=texture\x00profile=%s\x00processor=%s\x00content=%s",
		SchemaVersion,
		sdkVersion,
		declaration.Profile,
		processor,
		contentHash,
	)))

	source := filepath.ToSlash(declaration.Source)
	output := contentHash + ".png"
	size := int64(len(contents))

	return Texture{
		Name:                  declaration.Name,
		Profile:               declaration.Profile,
		Source:                source,
		Output:                output,
		ContentSHA256:         contentHash,
		CacheKey:              hex.EncodeToString(cacheDigest[:]),
		Processor:             processor,
		Width:                 configuration.Width,
		Height:                configuration.Height,
		SourceBytes:           size,
		OutputBytes:           size,
		EstimatedDecodedBytes: width * height * bytesPerRGBApixel,
		SourcePath:            sourcePath,
	}, nil
}
