// Package assetpipeline inspects and describes deterministic runtime assets.
package assetpipeline

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-sdk/format/textureatlas"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/pelletier/go-toml/v2"
)

const (
	ReportSchemaVersion = 2
)

var errUnknownTextureProfile = errors.New("texture profile is not defined by the selected SDK")
var errUnsupportedTextureProcessor = errors.New("texture processor is not supported by this CLI")

var (
	// ErrAssetResourceLimits reports an aggregate decoded or packaged asset limit.
	ErrAssetResourceLimits = errors.New("processed assets exceed aggregate resource limits")
	// ErrUnknownSoundProfile reports a sound profile absent from the selected SDK.
	ErrUnknownSoundProfile = errors.New("sound profile is not defined by the selected SDK")
	// ErrSoundProcessor reports a sound processor unsupported by this CLI.
	ErrSoundProcessor = errors.New("sound processor is not supported by this CLI")
)

// GeneratedNotice makes build-report ownership explicit without relying on
// the file name alone.
type GeneratedNotice struct {
	Warning       string `toml:"warning"`
	DoNotEdit     string `toml:"doNotEdit"`
	SourceOfTruth string `toml:"sourceOfTruth"`
	Regenerate    string `toml:"regenerate"`
}

// Texture describes a validated texture and its deterministic output identity.
type Texture struct {
	Kind                  string             `toml:"kind,omitempty"`
	Name                  string             `toml:"name"`
	Profile               string             `toml:"profile"`
	Source                string             `toml:"source"`
	Output                string             `toml:"cacheOutput"`
	ContentSHA256         string             `toml:"contentSha256"`
	SourceSHA256          string             `toml:"sourceSha256,omitempty"`
	OutputSHA256          string             `toml:"outputSha256,omitempty"`
	CacheKey              string             `toml:"cacheKey"`
	Processor             string             `toml:"processor"`
	Encoding              string             `toml:"encoding,omitempty"`
	Transform             *asset.ImageRecipe `toml:"transform,omitempty"`
	Metadata              *ImageMetadata     `toml:"metadata,omitempty"`
	Width                 int                `toml:"width"`
	Height                int                `toml:"height"`
	SourceBytes           int64              `toml:"sourceBytes"`
	OutputBytes           int64              `toml:"outputBytes"`
	EstimatedDecodedBytes int64              `toml:"estimatedDecodedBytes"`
	CacheHit              bool               `toml:"cacheHit"`
	SourcePath            string             `toml:"-"`
}

// Sound describes a processed game sound and its stable, name-sorted catalog ID.
type Sound struct {
	ID                    uint32            `toml:"id"`
	Name                  string            `toml:"name"`
	Kind                  string            `toml:"kind"`
	Profile               string            `toml:"profile"`
	Source                string            `toml:"source"`
	Output                string            `toml:"cacheOutput"`
	SourceSHA256          string            `toml:"sourceSha256"`
	OutputSHA256          string            `toml:"outputSha256"`
	CacheKey              string            `toml:"cacheKey"`
	Processor             string            `toml:"processor"`
	Encoding              string            `toml:"encoding"`
	Transform             asset.AudioRecipe `toml:"transform"`
	Metadata              AudioMetadata     `toml:"metadata"`
	SourceBytes           int64             `toml:"sourceBytes"`
	OutputBytes           int64             `toml:"outputBytes"`
	EstimatedDecodedBytes int64             `toml:"estimatedDecodedBytes"`
	CacheHit              bool              `toml:"cacheHit"`
	SourcePath            string            `toml:"-"`
}

// AudioStream describes a processed long-form QOA sidecar.
type AudioStream struct {
	ID           uint32            `toml:"id"`
	Name         string            `toml:"name"`
	Kind         string            `toml:"kind"`
	Profile      string            `toml:"profile"`
	Source       string            `toml:"source"`
	Output       string            `toml:"cacheOutput"`
	SourceSHA256 string            `toml:"sourceSha256"`
	OutputSHA256 string            `toml:"outputSha256"`
	CacheKey     string            `toml:"cacheKey"`
	Processor    string            `toml:"processor"`
	Encoding     string            `toml:"encoding"`
	Transform    asset.AudioRecipe `toml:"transform"`
	Metadata     AudioMetadata     `toml:"metadata"`
	SourceBytes  int64             `toml:"sourceBytes"`
	OutputBytes  int64             `toml:"outputBytes"`
	CacheHit     bool              `toml:"cacheHit"`
	SourcePath   string            `toml:"-"`
}

// LevelTexture identifies a processed texture packaged into one level.
type LevelTexture struct {
	Level   string  `toml:"level"`
	Texture Texture `toml:"texture"`
}

// Summary records encoded and estimated decoded texture costs separately.
type Summary struct {
	TextureCount               int   `toml:"textureCount"`
	DeclaredTextureCount       int   `toml:"declaredTextureCount"`
	UsedTextureCount           int   `toml:"usedTextureCount"`
	KeptTextureCount           int   `toml:"keptTextureCount"`
	StrippedTextureCount       int   `toml:"strippedTextureCount"`
	SourceBytes                int64 `toml:"sourceBytes"`
	OutputBytes                int64 `toml:"outputBytes"`
	StrippedSourceBytes        int64 `toml:"strippedSourceBytes"`
	EstimatedDecodedBytes      int64 `toml:"estimatedDecodedBytes"`
	SoundCount                 int   `toml:"soundCount"`
	SoundSourceBytes           int64 `toml:"soundSourceBytes"`
	SoundOutputBytes           int64 `toml:"soundOutputBytes"`
	AudioStreamCount           int   `toml:"audioStreamCount"`
	AudioStreamSourceBytes     int64 `toml:"audioStreamSourceBytes"`
	AudioStreamOutputBytes     int64 `toml:"audioStreamOutputBytes"`
	EstimatedDecodedSoundBytes int64 `toml:"estimatedDecodedSoundBytes"`
	LevelTextureCount          int   `toml:"levelTextureCount"`
	LevelSourceBytes           int64 `toml:"levelSourceBytes"`
	LevelOutputBytes           int64 `toml:"levelOutputBytes"`
	EstimatedDecodedLevelBytes int64 `toml:"estimatedDecodedLevelBytes"`
	AssetSourceBytes           int64 `toml:"assetSourceBytes"`
	AssetOutputBytes           int64 `toml:"assetOutputBytes"`
	DeclaredFontCount          int   `toml:"declaredFontCount"`
	UsedFontCount              int   `toml:"usedFontCount"`
	StrippedFontCount          int   `toml:"strippedFontCount"`
	FontSourceBytes            int64 `toml:"fontSourceBytes"`
	StrippedFontBytes          int64 `toml:"strippedFontBytes"`
}

// Usage explains why each declared texture was retained or stripped.
type Usage struct {
	Name        string `toml:"name"`
	Status      string `toml:"status"`
	Reason      string `toml:"reason"`
	SourceBytes int64  `toml:"sourceBytes"`
}

// FontUsage explains whether a declared semantic font role was embedded.
type FontUsage struct {
	Role        string `toml:"role"`
	Source      string `toml:"source"`
	Status      string `toml:"status"`
	Reason      string `toml:"reason"`
	SourceBytes int64  `toml:"sourceBytes"`
}

// Report is the deterministic asset inspection output for a build.
type Report struct {
	Atlas          *textureatlas.Bundle `toml:"-"`
	TextureSources []Texture            `toml:"textureSources,omitempty"`
	Generated      GeneratedNotice      `toml:"generated"`
	Schema         int                  `toml:"schema"`
	SDK            string               `toml:"sdk"`
	Features       []string             `toml:"features,omitempty"`
	Summary        Summary              `toml:"summary"`
	Usage          []Usage              `toml:"usage"`
	Fonts          []FontUsage          `toml:"fonts"`
	Textures       []Texture            `toml:"textures"`
	Sounds         []Sound              `toml:"sounds,omitempty"`
	AudioStreams   []AudioStream        `toml:"audioStreams,omitempty"`
	Levels         []LevelTexture       `toml:"levelTextures,omitempty"`
}

// ProcessAudioStreams produces name-sorted, independently numbered music and
// environment sidecars. The returned files remain in the verified asset cache
// until the build stages them by content hash.
//
//nolint:wsl_v5,nlreturn // Both independently numbered stream kinds deliberately share one pass.
func ProcessAudioStreams(
	ctx context.Context,
	directory string,
	manifest sdk.Manifest,
	music, environments []project.AudioStream,
) ([]AudioStream, error) {
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

		texture, err := processTexture(ctx, directory, cache, profile, declaration)
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
	projectRoot string,
	profile sdk.AssetProfile,
	declaration project.Texture,
) (Texture, error) {
	return processTexture(ctx, projectRoot, NewProjectCache(projectRoot), profile, declaration)
}

func processTexture(
	ctx context.Context,
	projectRoot string,
	cache *Cache,
	profile sdk.AssetProfile,
	declaration project.Texture,
) (Texture, error) {
	switch profile.Processor {
	case asset.ProcessorQOIv1:
		return processQOITexture(ctx, projectRoot, cache, profile.Transform, declaration)
	case asset.ProcessorQOAv1, asset.ProcessorCopyPNGv1:
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
	if err := toml.Unmarshal(artifact.Metadata, &metadata); err != nil {
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
	if err := toml.Unmarshal(artifact.Metadata, &metadata); err != nil {
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
	if err := toml.Unmarshal(artifact.Metadata, &metadata); err != nil {
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
