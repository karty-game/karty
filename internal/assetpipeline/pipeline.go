// Package assetpipeline inspects and describes deterministic runtime assets.
package assetpipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
)

const (
	SchemaVersion       = 1
	TextureProcessor    = "copy-png@1"
	ReportSchemaVersion = 2
	bytesPerRGBApixel   = 4
)

var errTextureDimensions = errors.New("texture dimensions cannot be represented safely")
var errUnknownTextureProfile = errors.New("texture profile is not defined by the selected SDK")
var errUnsupportedTextureProcessor = errors.New("texture processor is not supported by this CLI")
var errCacheContent = errors.New("cached texture content does not match its declared hash")

var (
	// ErrAssetResourceLimits reports an aggregate decoded or packaged asset limit.
	ErrAssetResourceLimits = errors.New("processed assets exceed aggregate resource limits")
	// ErrUnknownSoundProfile reports a sound profile absent from the selected SDK.
	ErrUnknownSoundProfile = errors.New("sound profile is not defined by the selected SDK")
	// ErrSoundProcessor reports a sound processor unsupported by this CLI.
	ErrSoundProcessor = errors.New("sound processor is not supported by this CLI")
)

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
	Kind                  string             `json:"kind,omitempty"`
	Name                  string             `json:"name"`
	Profile               string             `json:"profile"`
	Source                string             `json:"source"`
	Output                string             `json:"cacheOutput"`
	ContentSHA256         string             `json:"contentSha256"`
	SourceSHA256          string             `json:"sourceSha256,omitempty"`
	OutputSHA256          string             `json:"outputSha256,omitempty"`
	CacheKey              string             `json:"cacheKey"`
	Processor             string             `json:"processor"`
	Encoding              string             `json:"encoding,omitempty"`
	Transform             *asset.ImageRecipe `json:"transform,omitempty"`
	Metadata              *ImageMetadata     `json:"metadata,omitempty"`
	Width                 int                `json:"width"`
	Height                int                `json:"height"`
	SourceBytes           int64              `json:"sourceBytes"`
	OutputBytes           int64              `json:"outputBytes"`
	EstimatedDecodedBytes int64              `json:"estimatedDecodedBytes"`
	CacheHit              bool               `json:"cacheHit"`
	SourcePath            string             `json:"-"`
}

// Sound describes a processed game sound and its stable, name-sorted catalog ID.
type Sound struct {
	ID                    uint32            `json:"id"`
	Name                  string            `json:"name"`
	Kind                  string            `json:"kind"`
	Profile               string            `json:"profile"`
	Source                string            `json:"source"`
	Output                string            `json:"cacheOutput"`
	SourceSHA256          string            `json:"sourceSha256"`
	OutputSHA256          string            `json:"outputSha256"`
	CacheKey              string            `json:"cacheKey"`
	Processor             string            `json:"processor"`
	Encoding              string            `json:"encoding"`
	Transform             asset.AudioRecipe `json:"transform"`
	Metadata              AudioMetadata     `json:"metadata"`
	SourceBytes           int64             `json:"sourceBytes"`
	OutputBytes           int64             `json:"outputBytes"`
	EstimatedDecodedBytes int64             `json:"estimatedDecodedBytes"`
	CacheHit              bool              `json:"cacheHit"`
	SourcePath            string            `json:"-"`
}

// AudioStream describes a processed long-form QOA sidecar.
type AudioStream struct {
	ID           uint32            `json:"id"`
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	Profile      string            `json:"profile"`
	Source       string            `json:"source"`
	Output       string            `json:"cacheOutput"`
	SourceSHA256 string            `json:"sourceSha256"`
	OutputSHA256 string            `json:"outputSha256"`
	CacheKey     string            `json:"cacheKey"`
	Processor    string            `json:"processor"`
	Encoding     string            `json:"encoding"`
	Transform    asset.AudioRecipe `json:"transform"`
	Metadata     AudioMetadata     `json:"metadata"`
	SourceBytes  int64             `json:"sourceBytes"`
	OutputBytes  int64             `json:"outputBytes"`
	CacheHit     bool              `json:"cacheHit"`
	SourcePath   string            `json:"-"`
}

// LevelTexture identifies a processed texture packaged into one level.
type LevelTexture struct {
	Level   string  `json:"level"`
	Texture Texture `json:"texture"`
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
	TextureCount               int   `json:"textureCount"`
	DeclaredTextureCount       int   `json:"declaredTextureCount"`
	UsedTextureCount           int   `json:"usedTextureCount"`
	KeptTextureCount           int   `json:"keptTextureCount"`
	StrippedTextureCount       int   `json:"strippedTextureCount"`
	SourceBytes                int64 `json:"sourceBytes"`
	OutputBytes                int64 `json:"outputBytes"`
	StrippedSourceBytes        int64 `json:"strippedSourceBytes"`
	EstimatedDecodedBytes      int64 `json:"estimatedDecodedBytes"`
	SoundCount                 int   `json:"soundCount"`
	SoundSourceBytes           int64 `json:"soundSourceBytes"`
	SoundOutputBytes           int64 `json:"soundOutputBytes"`
	AudioStreamCount           int   `json:"audioStreamCount"`
	AudioStreamSourceBytes     int64 `json:"audioStreamSourceBytes"`
	AudioStreamOutputBytes     int64 `json:"audioStreamOutputBytes"`
	EstimatedDecodedSoundBytes int64 `json:"estimatedDecodedSoundBytes"`
	LevelTextureCount          int   `json:"levelTextureCount"`
	LevelSourceBytes           int64 `json:"levelSourceBytes"`
	LevelOutputBytes           int64 `json:"levelOutputBytes"`
	EstimatedDecodedLevelBytes int64 `json:"estimatedDecodedLevelBytes"`
	AssetSourceBytes           int64 `json:"assetSourceBytes"`
	AssetOutputBytes           int64 `json:"assetOutputBytes"`
	DeclaredFontCount          int   `json:"declaredFontCount"`
	UsedFontCount              int   `json:"usedFontCount"`
	StrippedFontCount          int   `json:"strippedFontCount"`
	FontSourceBytes            int64 `json:"fontSourceBytes"`
	StrippedFontBytes          int64 `json:"strippedFontBytes"`
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
	Generated    GeneratedNotice `json:"generated"`
	Schema       int             `json:"schema"`
	SDK          string          `json:"sdk"`
	Features     []string        `json:"features,omitempty"`
	Summary      Summary         `json:"summary"`
	Usage        []Usage         `json:"usage"`
	Fonts        []FontUsage     `json:"fonts"`
	Textures     []Texture       `json:"textures"`
	Sounds       []Sound         `json:"sounds,omitempty"`
	AudioStreams []AudioStream   `json:"audioStreams,omitempty"`
	Levels       []LevelTexture  `json:"levelTextures,omitempty"`
}

// ProcessAudioStreams produces name-sorted, independently numbered music and
// environment sidecars. The returned files remain in the verified asset cache
// until the build stages them by content hash.
//
//nolint:golines,wsl_v5,nlreturn // Both independently numbered stream kinds deliberately share one pass.
func ProcessAudioStreams(ctx context.Context, directory string, manifest sdk.Manifest, music, environments []project.AudioStream) ([]AudioStream, error) {
	cache := NewProjectCache(directory)
	result := make([]AudioStream, 0, len(music)+len(environments))
	for _, group := range []struct {
		kind  string
		items []project.AudioStream
	}{{"music", music}, {"environment", environments}} {
		items := slices.Clone(group.items)
		slices.SortFunc(items, func(left, right project.AudioStream) int { return strings.Compare(left.Name, right.Name) })
		for index, declaration := range items {
			stream, err := processQOAAudioStream(ctx, directory, cache, manifest, declaration, group.kind, uint32(index+1))
			if err != nil {
				return nil, err
			}
			result = append(result, stream)
		}
	}
	return result, nil
}

func imageProcessorRevisions() []Revision {
	return []Revision{
		{Name: "codec", Version: "qoi@1"},
		{Name: "image-import", Version: "png-jpeg-webp@1"},
		{Name: "image-transform", Version: "imaging-lanczos3@1"},
	}
}

func audioProcessorRevisions() []Revision {
	return []Revision{
		{Name: "audio-import", Version: "wav@1"},
		{Name: "audio-normalize", Version: wavNormalizerRevision},
		{Name: "audio-resample", Version: strings.ReplaceAll(resamplerRevision, ":", "/")},
		{Name: "codec", Version: "qoa@1"},
	}
}

// ProcessGameAssets resolves SDK defaults and project overrides, then returns
// only verified cache artifacts suitable for cartridge packaging.
func ProcessGameAssets(
	ctx context.Context,
	directory string,
	manifest sdk.Manifest,
	textures []project.Texture,
	sounds []project.Sound,
) (Report, error) {
	report := newReport(manifest.Version, len(textures), len(sounds))
	cache := NewProjectCache(directory)

	for _, declaration := range textures {
		profile, exists := manifest.Assets.TextureProfiles[declaration.Profile]
		if !exists {
			return Report{}, fmt.Errorf("texture %q profile %q: %w", declaration.Name, declaration.Profile, errUnknownTextureProfile)
		}

		texture, err := processTexture(ctx, directory, manifest.Version, cache, profile, declaration)
		if err != nil {
			return Report{}, err
		}

		report.Textures = append(report.Textures, texture)
	}

	sortedSounds := slices.Clone(sounds)
	slices.SortFunc(sortedSounds, func(left, right project.Sound) int { return strings.Compare(left.Name, right.Name) })

	for index, declaration := range sortedSounds {
		sound, err := processQOASound(ctx, directory, cache, manifest, declaration, uint32(index+1))
		if err != nil {
			return Report{}, err
		}

		report.Sounds = append(report.Sounds, sound)
	}

	for _, texture := range report.Textures {
		report.Summary.SourceBytes += texture.SourceBytes
		report.Summary.OutputBytes += texture.OutputBytes
		report.Summary.EstimatedDecodedBytes += texture.EstimatedDecodedBytes

		if texture.Processor == string(asset.ProcessorQOIv1) {
			report.Features = append(report.Features, cartridge.FeatureTextureQOIv1)
		}
	}

	if report.Summary.EstimatedDecodedBytes > asset.MaxDecodedTextures {
		return Report{}, fmt.Errorf("textures: %w", ErrAssetResourceLimits)
	}

	encodedSoundBundleBytes := int64(16)

	for _, sound := range report.Sounds {
		report.Summary.SoundSourceBytes += sound.SourceBytes
		report.Summary.SoundOutputBytes += sound.OutputBytes
		report.Summary.EstimatedDecodedSoundBytes += sound.EstimatedDecodedBytes
		encodedSoundBundleBytes += int64(24+len(sound.Name)) + sound.OutputBytes
	}

	if len(report.Sounds) > cartridge.MaxSoundCount ||
		report.Summary.EstimatedDecodedSoundBytes > asset.MaxDecodedSounds ||
		encodedSoundBundleBytes > cartridge.MaxSoundBundle {
		return Report{}, fmt.Errorf("sounds: %w", ErrAssetResourceLimits)
	}

	if len(report.Sounds) > 0 {
		report.Features = append(report.Features, cartridge.FeatureSoundQOAv1)
	}

	slices.Sort(report.Features)
	report.Features = slices.Compact(report.Features)
	report.Summary.TextureCount = len(report.Textures)
	report.Summary.SoundCount = len(report.Sounds)
	report.Summary.AssetSourceBytes = report.Summary.SourceBytes + report.Summary.SoundSourceBytes
	report.Summary.AssetOutputBytes = report.Summary.OutputBytes + report.Summary.SoundOutputBytes

	return report, nil
}

// ProcessTexture resolves one texture declaration through a selected SDK
// profile and returns a verified cache artifact. It is shared by game and
// level builders so both use identical recipe and revision identities.
func ProcessTexture(
	ctx context.Context,
	projectRoot, sdkVersion string,
	profile sdk.AssetProfile,
	declaration project.Texture,
) (Texture, error) {
	return processTexture(ctx, projectRoot, sdkVersion, NewProjectCache(projectRoot), profile, declaration)
}

func processTexture(
	ctx context.Context,
	projectRoot, sdkVersion string,
	cache *Cache,
	profile sdk.AssetProfile,
	declaration project.Texture,
) (Texture, error) {
	switch profile.Processor {
	case asset.ProcessorCopyPNGv1:
		texture, err := analyzeTexture(projectRoot, sdkVersion, string(profile.Processor), declaration)
		if err != nil {
			return Texture{}, err
		}

		legacy := Report{Textures: []Texture{texture}}
		if err := PopulateCache(projectRoot, &legacy); err != nil {
			return Texture{}, fmt.Errorf("populate asset cache: %w", err)
		}

		texture = legacy.Textures[0]
		texture.Kind, texture.Encoding = "texture", "png"
		texture.SourceSHA256, texture.OutputSHA256 = texture.ContentSHA256, texture.ContentSHA256

		return texture, nil
	case asset.ProcessorQOIv1:
		return processQOITexture(ctx, projectRoot, cache, profile.Transform, declaration)
	case asset.ProcessorQOAv1:
		return Texture{}, fmt.Errorf("texture %q processor %q: %w", declaration.Name, profile.Processor, errUnsupportedTextureProcessor)
	default:
		return Texture{}, fmt.Errorf("texture %q processor %q: %w", declaration.Name, profile.Processor, errUnsupportedTextureProcessor)
	}
}

func newReport(version string, textureCount, soundCount int) Report {
	return Report{Generated: GeneratedNotice{
		Warning: "GENERATED FILE. DO NOT EDIT.", DoNotEdit: "MANUAL CHANGES WILL BE OVERWRITTEN.",
		SourceOfTruth: "karty.toml, declared asset files, and the selected Karty SDK", Regenerate: "karty build",
	}, Schema: ReportSchemaVersion, SDK: version, Textures: make([]Texture, 0, textureCount), Sounds: make([]Sound, 0, soundCount)}
}

func processQOITexture(
	ctx context.Context,
	directory string,
	cache *Cache,
	base asset.ImageRecipe,
	declaration project.Texture,
) (Texture, error) {
	recipe := base
	if declaration.Transform.MaxWidth != 0 {
		recipe.MaxWidth = declaration.Transform.MaxWidth
	}

	if declaration.Transform.MaxHeight != 0 {
		recipe.MaxHeight = declaration.Transform.MaxHeight
	}

	if declaration.Transform.Filter != "" {
		recipe.Filter = asset.ImageFilter(declaration.Transform.Filter)
	}

	if declaration.Transform.BitDepth != 0 {
		recipe.BitDepth = declaration.Transform.BitDepth
	}

	if err := recipe.Validate(); err != nil {
		return Texture{}, fmt.Errorf("texture %q transform: %w", declaration.Name, err)
	}

	snapshot, err := SnapshotFile(filepath.Join(directory, filepath.FromSlash(declaration.Source)))
	if err != nil {
		return Texture{}, fmt.Errorf("read texture %q: %w", declaration.Name, err)
	}

	cacheRecipe, err := NewRecipe("texture", string(asset.ProcessorQOIv1), imageProcessorRevisions(), recipe)
	if err != nil {
		return Texture{}, fmt.Errorf("texture %q recipe: %w", declaration.Name, err)
	}

	artifact, err := cache.Resolve(ctx, snapshot, cacheRecipe, func(_ context.Context, source SourceSnapshot, _ Recipe) (Processed, error) {
		processed, processErr := ProcessImage(source.Bytes(), recipe)
		if processErr != nil {
			return Processed{}, processErr
		}

		return Processed{Encoding: processed.Encoding, Payload: processed.EncodedQOI, Metadata: processed.Metadata}, nil
	})
	if err != nil {
		return Texture{}, fmt.Errorf("process texture %q: %w", declaration.Name, err)
	}

	var metadata ImageMetadata
	if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
		return Texture{}, fmt.Errorf("decode texture %q cache metadata: %w", declaration.Name, err)
	}

	transform := recipe

	return Texture{
		Name: declaration.Name, Kind: "texture", Profile: declaration.Profile,
		Source: filepath.ToSlash(declaration.Source), Output: artifact.OutputDigest + ".qoi",
		ContentSHA256: artifact.SourceDigest, SourceSHA256: artifact.SourceDigest, OutputSHA256: artifact.OutputDigest,
		CacheKey: artifact.CacheKey, Processor: string(asset.ProcessorQOIv1), Encoding: artifact.Encoding,
		Transform: &transform, Metadata: &metadata, Width: int(metadata.Width), Height: int(metadata.Height),
		SourceBytes: snapshot.Size(), OutputBytes: artifact.OutputBytes,
		EstimatedDecodedBytes: int64(metadata.DecodedBytes), CacheHit: artifact.CacheHit,
		SourcePath: artifact.PayloadPath,
	}, nil
}

func processQOASound(
	ctx context.Context,
	directory string,
	cache *Cache,
	manifest sdk.Manifest,
	declaration project.Sound,
	soundID uint32,
) (Sound, error) {
	profile, exists := manifest.Assets.SoundProfiles[declaration.Profile]
	if !exists {
		return Sound{}, fmt.Errorf("sound %q profile %q: %w", declaration.Name, declaration.Profile, ErrUnknownSoundProfile)
	}

	if profile.Processor != asset.ProcessorQOAv1 {
		return Sound{}, fmt.Errorf("sound %q processor %q: %w", declaration.Name, profile.Processor, ErrSoundProcessor)
	}

	recipe := profile.Transform
	if declaration.Transform.SampleRate != 0 {
		recipe.SampleRate = declaration.Transform.SampleRate
	}

	if declaration.Transform.Channels != "" {
		recipe.ChannelMode = asset.ChannelMode(declaration.Transform.Channels)
	}

	if err := recipe.Validate(); err != nil {
		return Sound{}, fmt.Errorf("sound %q transform: %w", declaration.Name, err)
	}

	snapshot, err := SnapshotFile(filepath.Join(directory, filepath.FromSlash(declaration.Source)))
	if err != nil {
		return Sound{}, fmt.Errorf("read sound %q: %w", declaration.Name, err)
	}

	cacheRecipe, err := NewRecipe("sound", string(asset.ProcessorQOAv1), audioProcessorRevisions(), recipe)
	if err != nil {
		return Sound{}, fmt.Errorf("sound %q recipe: %w", declaration.Name, err)
	}

	artifact, err := cache.Resolve(ctx, snapshot, cacheRecipe, func(_ context.Context, source SourceSnapshot, _ Recipe) (Processed, error) {
		processed, processErr := ProcessAudio(source.Bytes(), recipe)
		if processErr != nil {
			return Processed{}, processErr
		}

		return Processed{Encoding: processed.Encoding, Payload: processed.EncodedQOA, Metadata: processed.Metadata}, nil
	})
	if err != nil {
		return Sound{}, fmt.Errorf("process sound %q: %w", declaration.Name, err)
	}

	var metadata AudioMetadata
	if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
		return Sound{}, fmt.Errorf("decode sound %q cache metadata: %w", declaration.Name, err)
	}

	return Sound{
		ID: soundID, Name: declaration.Name, Kind: "sound", Profile: declaration.Profile,
		Source: filepath.ToSlash(declaration.Source), Output: artifact.OutputDigest + ".qoa",
		SourceSHA256: artifact.SourceDigest, OutputSHA256: artifact.OutputDigest,
		CacheKey: artifact.CacheKey, Processor: string(asset.ProcessorQOAv1), Encoding: artifact.Encoding,
		Transform: recipe, Metadata: metadata, SourceBytes: snapshot.Size(), OutputBytes: artifact.OutputBytes,
		EstimatedDecodedBytes: int64(metadata.DecodedBytes), CacheHit: artifact.CacheHit,
		SourcePath: artifact.PayloadPath,
	}, nil
}

//nolint:wsl_v5,nlreturn // Processing steps stay aligned with the one-shot QOA path.
func processQOAAudioStream(
	ctx context.Context,
	directory string,
	cache *Cache,
	manifest sdk.Manifest,
	declaration project.AudioStream,
	kind string,
	streamID uint32,
) (AudioStream, error) {
	profile, exists := manifest.Assets.SoundProfiles[declaration.Profile]
	if !exists {
		return AudioStream{}, fmt.Errorf("%s %q profile %q: %w", kind, declaration.Name, declaration.Profile, ErrUnknownSoundProfile)
	}
	if profile.Processor != asset.ProcessorQOAv1 {
		return AudioStream{}, fmt.Errorf("%s %q processor %q: %w", kind, declaration.Name, profile.Processor, ErrSoundProcessor)
	}
	recipe := profile.Transform
	if declaration.Transform.SampleRate != 0 {
		recipe.SampleRate = declaration.Transform.SampleRate
	}
	if declaration.Transform.Channels != "" {
		recipe.ChannelMode = asset.ChannelMode(declaration.Transform.Channels)
	}
	if err := recipe.Validate(); err != nil {
		return AudioStream{}, fmt.Errorf("%s %q transform: %w", kind, declaration.Name, err)
	}
	snapshot, err := SnapshotFile(filepath.Join(directory, filepath.FromSlash(declaration.Source)))
	if err != nil {
		return AudioStream{}, fmt.Errorf("read %s %q: %w", kind, declaration.Name, err)
	}
	cacheRecipe, err := NewRecipe("audio-stream-"+kind, string(asset.ProcessorQOAv1), audioProcessorRevisions(), recipe)
	if err != nil {
		return AudioStream{}, fmt.Errorf("%s %q recipe: %w", kind, declaration.Name, err)
	}
	artifact, err := cache.Resolve(ctx, snapshot, cacheRecipe, func(_ context.Context, source SourceSnapshot, _ Recipe) (Processed, error) {
		processed, processErr := ProcessStreamAudio(source.Bytes(), recipe)
		if processErr != nil {
			return Processed{}, processErr
		}
		return Processed{Encoding: processed.Encoding, Payload: processed.EncodedQOA, Metadata: processed.Metadata}, nil
	})
	if err != nil {
		return AudioStream{}, fmt.Errorf("process %s %q: %w", kind, declaration.Name, err)
	}
	var metadata AudioMetadata
	if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
		return AudioStream{}, fmt.Errorf("decode %s %q cache metadata: %w", kind, declaration.Name, err)
	}
	return AudioStream{
		ID: streamID, Name: declaration.Name, Kind: kind, Profile: declaration.Profile,
		Source: filepath.ToSlash(declaration.Source), Output: artifact.OutputDigest + ".kaud",
		SourceSHA256: artifact.SourceDigest, OutputSHA256: artifact.OutputDigest, CacheKey: artifact.CacheKey,
		Processor: string(asset.ProcessorQOAv1), Encoding: artifact.Encoding, Transform: recipe, Metadata: metadata,
		SourceBytes: snapshot.Size(), OutputBytes: artifact.OutputBytes, CacheHit: artifact.CacheHit, SourcePath: artifact.PayloadPath,
	}, nil
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

		texture, err := analyzeTexture(directory, manifest.Version, string(profile.Processor), declaration)
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
