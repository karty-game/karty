// Package project loads the project metadata required by CLI commands.
package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	assetcontract "github.com/karty-game/karty-sdk/format/asset"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type staticError string

func (err staticError) Error() string {
	return string(err)
}

const errIncompleteProject staticError = "project must define project.name and sdk.version"
const errInvalidResolution staticError = "project resolution must be positive"
const errInvalidCompiler staticError = "project compiler must be tinygo or go"
const errInvalidTexture staticError = "project texture must define a relative source and logical name"
const errDuplicateTexture staticError = "project texture logical name is duplicated"
const errInvalidSound staticError = "project sound must define a relative WAV source and logical name"
const errDuplicateSound staticError = "project sound logical name is duplicated"
const errInvalidAudioStream staticError = "project music/environment must define a relative WAV source and logical name"
const errDuplicateAudioStream staticError = "project music/environment logical name is duplicated within its kind"
const errInvalidVideo staticError = "project video requires a unique name and confined .mpg source"
const errMissingModule staticError = "project go.mod must define a module path"
const errInvalidTheme staticError = "project theme must define a confined regular source"
const errInvalidLayout staticError = "project layout must define a unique confined regular source"
const errInvalidFont staticError = "project font must define a unique body, display, or mono role and a confined TTF/OTF source"

const DefaultTextureProfile = "sprite"
const DefaultSoundProfile = "effect"
const DefaultMusicProfile = "music"
const DefaultEnvironmentProfile = "environment"

// Config is the supported subset of karty.toml.
type Config struct {
	Project struct {
		Name       string `koanf:"name"`
		Compiler   string `koanf:"compiler"`
		Resolution struct {
			Width  int `koanf:"width"`
			Height int `koanf:"height"`
		} `koanf:"resolution"`
		Camera CameraConfig `koanf:"camera"`
		Debug  struct {
			Renderer bool `koanf:"renderer"`
		} `koanf:"debug"`
	} `koanf:"project"`
	SDK struct {
		Version string `koanf:"version"`
	} `koanf:"sdk"`
	Assets Assets `koanf:"assets"`
}

// Assets contains convention-discovered entries plus optional manifest overrides.
type Assets struct {
	Textures     []Texture     `koanf:"texture"`
	Sounds       []Sound       `koanf:"sound"`
	Music        []AudioStream `koanf:"music"`
	Environments []AudioStream `koanf:"environment"`
	Videos       []Video       `koanf:"video"`
	Fonts        []Font        `koanf:"font"`
	UI           []Texture     `koanf:"ui"`
	Layouts      []Layout      `koanf:"layout"`
	Theme        Theme         `koanf:"theme"`
}

type Theme struct {
	Source string `koanf:"source"`
}

// Layout identifies one build-time-only KartUI layout source.
type Layout struct {
	Source string `koanf:"source"`
}

// Texture describes a logical texture source in a project manifest.
type Texture struct {
	Name      string           `koanf:"name"`
	Source    string           `koanf:"source"`
	Profile   string           `koanf:"profile"`
	Keep      bool             `koanf:"keep"`
	Transform TextureTransform `koanf:"transform"`
	Inferred  bool             `koanf:"-"`
}

// TextureTransform contains sparse per-asset overrides for the SDK profile.
// Zero values inherit the selected profile.
type TextureTransform struct {
	MaxWidth  uint32 `koanf:"max_width"`
	MaxHeight uint32 `koanf:"max_height"`
	Filter    string `koanf:"filter"`
	BitDepth  uint8  `koanf:"bit_depth"`
}

// Sound describes a game-scoped one-shot WAV source.
// Video is an explicitly declared MPEG-PS source copied without transcoding.
type Video struct {
	Name   string `koanf:"name"`
	Source string `koanf:"source"`
}

type Sound struct {
	Name      string         `koanf:"name"`
	Source    string         `koanf:"source"`
	Profile   string         `koanf:"profile"`
	Transform SoundTransform `koanf:"transform"`
	Inferred  bool           `koanf:"-"`
}

// AudioStream describes a long-form WAV source encoded as a staged QOA sidecar.
type AudioStream struct {
	Name      string         `koanf:"name"`
	Source    string         `koanf:"source"`
	Profile   string         `koanf:"profile"`
	Transform SoundTransform `koanf:"transform"`
}

// SoundTransform contains sparse per-asset overrides for the SDK profile.
type SoundTransform struct {
	SampleRate uint32 `koanf:"sample_rate"`
	Channels   string `koanf:"channels"`
}

// Font binds one project font file to a semantic UI typography role.
type Font struct {
	Role   string `koanf:"role"`
	Source string `koanf:"source"`
}

// ModulePath returns the module path used to import the generated engine package.
func ModulePath(directory string) (string, error) {
	contents, err := os.ReadFile(filepath.Join(directory, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read project module: %w", err)
	}

	for line := range strings.SplitSeq(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}

	return "", errMissingModule
}

// Load reads and validates the project configuration in directory.
//
//nolint:wsl_v5 // Validation remains in declaration order for actionable errors.
func Load(directory string) (Config, error) {
	path := filepath.Join(directory, "karty.toml")

	configFile := koanf.New(".")
	if err := configFile.Load(file.Provider(path), toml.Parser()); err != nil {
		return Config{}, fmt.Errorf("load %s: %w", path, err)
	}

	var config Config
	if err := configFile.UnmarshalWithConf("", &config, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", path, err)
	}

	if config.Project.Name == "" || config.SDK.Version == "" {
		return Config{}, fmt.Errorf("%s: %w", path, errIncompleteProject)
	}

	if config.Project.Compiler == "" {
		config.Project.Compiler = "tinygo"
	}

	if config.Project.Compiler != "tinygo" && config.Project.Compiler != "go" {
		return Config{}, fmt.Errorf("%s: %w", path, errInvalidCompiler)
	}

	if config.Project.Resolution.Width == 0 {
		config.Project.Resolution.Width = 960
	}

	if config.Project.Resolution.Height == 0 {
		config.Project.Resolution.Height = 540
	}

	if config.Project.Resolution.Width < 1 || config.Project.Resolution.Height < 1 {
		return Config{}, fmt.Errorf("%s: %w", path, errInvalidResolution)
	}

	if err := loadCameraConfig(&config, configFile); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}

	seenVideos := make(map[string]bool)
	for _, video := range config.Assets.Videos {
		if video.Name == "" || seenVideos[video.Name] || validateConfinedFile(directory, video.Source) != nil ||
			strings.ToLower(filepath.Ext(video.Source)) != ".mpg" {
			return Config{}, fmt.Errorf("video %q: %w", video.Name, errInvalidVideo)
		}
		seenVideos[video.Name] = true
	}
	if err := resolveAssets(directory, &config.Assets); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}

	return config, nil
}

func validateTheme(directory, source string) error {
	if source == "" {
		return nil
	}

	if validateConfinedFile(directory, source) != nil {
		return errInvalidTheme
	}

	return nil
}

func validateConfinedFile(directory, source string) error {
	relative, err := filepath.Rel(".", filepath.Clean(source))
	if err != nil || filepath.IsAbs(source) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errInvalidLayout
	}

	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return errInvalidLayout
	}

	resolved, err := filepath.EvalSymlinks(filepath.Join(directory, source))
	if err != nil {
		return errInvalidLayout
	}

	confined, err := filepath.Rel(root, resolved)
	if err != nil || confined == ".." || strings.HasPrefix(confined, ".."+string(filepath.Separator)) {
		return errInvalidLayout
	}

	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return errInvalidLayout
	}

	return nil
}

// resolveAssets merges ordinary convention-discovered sources with sparse
// manifest overrides. The resulting list is still explicit for build validation.
//
//nolint:wsl_v5,nlreturn // Discovery follows one deliberate resolution sequence.
func resolveAssets(directory string, assets *Assets) error {
	textures, err := discoverTextures(directory)
	if err != nil {
		return err
	}
	assets.Textures, err = mergeTextures(directory, "assets/textures", textures, assets.Textures)
	if err != nil {
		return err
	}

	sounds, err := discoverSounds(directory)
	if err != nil {
		return err
	}
	assets.Sounds, err = mergeSounds(directory, sounds, assets.Sounds)
	if err != nil {
		return err
	}

	assets.Music, err = mergeAudioStreams(directory, "assets/music", assets.Music)
	if err != nil {
		return err
	}
	assets.Environments, err = mergeAudioStreams(directory, "assets/environment", assets.Environments)
	if err != nil {
		return err
	}
	streamSources := make(map[string]bool, len(assets.Music)+len(assets.Environments))
	for _, stream := range append(slices.Clone(assets.Music), assets.Environments...) {
		streamSources[stream.Source] = true
	}
	assets.Sounds = slices.DeleteFunc(assets.Sounds, func(sound Sound) bool {
		return sound.Inferred && streamSources[sound.Source]
	})

	uiEntries, err := discoverUI(directory)
	if err != nil {
		return err
	}
	assets.UI, err = mergeTextures(directory, "ui", uiEntries, assets.UI)
	if err != nil {
		return err
	}

	layouts, err := discoverLayouts(directory)
	if err != nil {
		return err
	}
	assets.Layouts, err = mergeLayouts(directory, layouts, assets.Layouts)
	if err != nil {
		return err
	}

	fonts, err := discoverFonts(directory)
	if err != nil {
		return err
	}
	assets.Fonts, err = mergeFonts(directory, fonts, assets.Fonts)
	if err != nil {
		return err
	}

	if assets.Theme.Source == "" {
		for _, candidate := range []string{"ui/theme.toml", "assets/ui/theme.toml"} {
			if validateConfinedFile(directory, candidate) == nil {
				assets.Theme.Source = candidate
				break
			}
		}
	}

	return validateTheme(directory, assets.Theme.Source)
}

//nolint:wsl_v5,nlreturn // Discovery helpers keep filesystem steps together.
func discoverTextures(directory string) ([]Texture, error) {
	files, err := discoverFiles(directory, "assets/textures", "")
	if err != nil {
		return nil, err
	}
	result := make([]Texture, 0, len(files))
	for _, source := range files {
		if !slices.Contains([]string{".png", ".jpg", ".jpeg", ".webp"}, strings.ToLower(filepath.Ext(source))) {
			continue
		}
		result = append(
			result,
			Texture{Name: inferredName(source, "assets/textures"), Source: source, Profile: DefaultTextureProfile, Inferred: true},
		)
	}
	return result, nil
}

func discoverSounds(directory string) ([]Sound, error) {
	files, err := discoverFiles(directory, "assets/sounds", ".wav")
	if err != nil {
		return nil, err
	}

	result := make([]Sound, 0, len(files))
	for _, source := range files {
		result = append(
			result,
			Sound{Name: inferredName(source, "assets/sounds"), Source: source, Profile: DefaultSoundProfile, Inferred: true},
		)
	}

	return result, nil
}

//nolint:wsl_v5,nlreturn // Discovery helpers keep filesystem steps together.
func discoverUI(directory string) ([]Texture, error) {
	result := make([]Texture, 0)
	for _, root := range []string{"ui", "assets/ui"} {
		files, err := discoverFiles(directory, root, ".ui")
		if err != nil {
			return nil, err
		}
		for _, source := range files {
			if strings.HasPrefix(source, root+"/layouts/") {
				continue
			}
			result = append(result, Texture{Name: "ui." + strings.TrimSuffix(filepath.Base(source), ".ui"), Source: source})
		}
	}
	slices.SortFunc(result, func(left, right Texture) int { return strings.Compare(left.Source, right.Source) })
	return result, nil
}

//nolint:wsl_v5,nlreturn // Discovery helpers keep filesystem steps together.
func discoverLayouts(directory string) ([]Layout, error) {
	files, err := discoverFiles(directory, "ui/layouts", ".ui")
	if err != nil {
		return nil, err
	}
	result := make([]Layout, 0, len(files))
	for _, source := range files {
		result = append(result, Layout{Source: source})
	}
	return result, nil
}

//nolint:wsl_v5,nlreturn // Discovery helpers keep filesystem steps together.
func discoverFonts(directory string) ([]Font, error) {
	files, err := discoverFiles(directory, "assets/fonts", "")
	if err != nil {
		return nil, err
	}
	result := make([]Font, 0, len(files))
	for _, source := range files {
		role := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
		if role == "body" || role == "display" || role == "mono" {
			result = append(result, Font{Role: role, Source: source})
		}
	}
	return result, nil
}

//nolint:wsl_v5,nlreturn // Walking a confined conventional root is one operation.
func discoverFiles(directory, root, extension string) ([]string, error) {
	path := filepath.Join(directory, filepath.FromSlash(root))
	entries := make([]string, 0)
	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if os.IsNotExist(walkErr) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || (extension != "" && strings.ToLower(filepath.Ext(current)) != extension) {
			return nil
		}
		relative, err := filepath.Rel(directory, current)
		if err != nil {
			return err
		}
		entries = append(entries, filepath.ToSlash(relative))
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", root, err)
	}
	slices.Sort(entries)
	return entries, nil
}

func inferredName(source, root string) string {
	name := strings.TrimPrefix(filepath.ToSlash(source), root+"/")
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.ReplaceAll(name, "/", ".")

	return strings.ReplaceAll(name, "-", ".")
}

//nolint:gocognit,wsl_v5,nlreturn // Override merging keeps precedence and validation explicit.
func mergeTextures(directory, inferredRoot string, discovered, overrides []Texture) ([]Texture, error) {
	result := append([]Texture(nil), discovered...)
	bySource := make(map[string]int, len(result))
	for index, entry := range result {
		bySource[entry.Source] = index
	}
	declaredSources := make(map[string]bool, len(overrides))
	declaredNames := make(map[string]bool, len(overrides))
	for _, override := range overrides {
		if override.Source == "" || validateConfinedFile(directory, override.Source) != nil {
			return nil, errInvalidTexture
		}
		if declaredSources[override.Source] || (override.Name != "" && declaredNames[override.Name]) {
			return nil, errDuplicateTexture
		}
		declaredSources[override.Source] = true
		if override.Name != "" {
			declaredNames[override.Name] = true
		}
		if override.Name == "" {
			override.Name = inferredName(override.Source, inferredRoot)
		}
		if override.Profile == "" {
			override.Profile = DefaultTextureProfile
		}
		if strings.HasPrefix(override.Name, "karty.") {
			return nil, errInvalidTexture
		}
		if !validTextureTransform(override.Transform) {
			return nil, errInvalidTexture
		}
		if index, exists := bySource[override.Source]; exists {
			result[index].Name = override.Name
			if override.Profile != DefaultTextureProfile || result[index].Profile == "" {
				result[index].Profile = override.Profile
			}
			result[index].Keep = result[index].Keep || override.Keep
			result[index].Transform = override.Transform
			result[index].Inferred = false
			continue
		}
		bySource[override.Source] = len(result)
		result = append(result, override)
	}
	names := make(map[string]struct{}, len(result))
	for index := range result {
		if result[index].Name == "" || result[index].Source == "" {
			return nil, errInvalidTexture
		}
		if result[index].Profile == "" {
			result[index].Profile = DefaultTextureProfile
		}
		if _, exists := names[result[index].Name]; exists {
			return nil, fmt.Errorf("texture %q: %w", result[index].Name, errDuplicateTexture)
		}
		names[result[index].Name] = struct{}{}
	}
	return result, nil
}

func validTextureTransform(transform TextureTransform) bool {
	return transform.MaxWidth <= assetcontract.MaxTextureDimension && transform.MaxHeight <= assetcontract.MaxTextureDimension &&
		(transform.Filter == "" || transform.Filter == string(assetcontract.ImageFilterNearest) ||
			transform.Filter == string(assetcontract.ImageFilterSmooth)) &&
		(transform.BitDepth == 0 || transform.BitDepth == 8)
}

func mergeSounds(directory string, discovered, overrides []Sound) ([]Sound, error) {
	result := append([]Sound(nil), discovered...)

	bySource := make(map[string]int, len(result))
	for index, entry := range result {
		bySource[entry.Source] = index
	}

	declaredSources := make(map[string]bool, len(overrides))

	declaredNames := make(map[string]bool, len(overrides))
	for _, override := range overrides {
		if override.Source == "" || strings.ToLower(filepath.Ext(override.Source)) != ".wav" ||
			validateConfinedFile(directory, override.Source) != nil || declaredSources[override.Source] ||
			(override.Name != "" && declaredNames[override.Name]) || !validSoundTransform(override.Transform) {
			return nil, errInvalidSound
		}

		declaredSources[override.Source] = true
		if override.Name == "" {
			override.Name = inferredName(override.Source, "assets/sounds")
		}

		declaredNames[override.Name] = true
		if override.Profile == "" {
			override.Profile = DefaultSoundProfile
		}

		if strings.HasPrefix(override.Name, "karty.") {
			return nil, errInvalidSound
		}

		if index, exists := bySource[override.Source]; exists {
			result[index] = override

			continue
		}

		bySource[override.Source] = len(result)
		result = append(result, override)
	}

	names := make(map[string]struct{}, len(result))
	for _, sound := range result {
		if sound.Name == "" || sound.Source == "" || sound.Profile == "" {
			return nil, errInvalidSound
		}

		if _, exists := names[sound.Name]; exists {
			return nil, fmt.Errorf("sound %q: %w", sound.Name, errDuplicateSound)
		}

		names[sound.Name] = struct{}{}
	}

	return result, nil
}

func validSoundTransform(transform SoundTransform) bool {
	validRate := transform.SampleRate == 0 || slices.Contains([]uint32{22_050, 24_000, 44_100, 48_000}, transform.SampleRate)
	validChannels := transform.Channels == "" || transform.Channels == string(assetcontract.ChannelPreserve) ||
		transform.Channels == string(assetcontract.ChannelMono)

	return validRate && validChannels
}

//nolint:wsl_v5,nlreturn // Declaration validation is one compact pass.
func mergeAudioStreams(directory, inferredRoot string, declarations []AudioStream) ([]AudioStream, error) {
	result := slices.Clone(declarations)
	names := make(map[string]bool, len(result))
	sources := make(map[string]bool, len(result))
	defaultProfile := DefaultMusicProfile
	if inferredRoot == "assets/environment" {
		defaultProfile = DefaultEnvironmentProfile
	}
	for index := range result {
		entry := &result[index]
		if entry.Source == "" || strings.ToLower(filepath.Ext(entry.Source)) != ".wav" ||
			validateConfinedFile(directory, entry.Source) != nil || sources[entry.Source] ||
			!validSoundTransform(entry.Transform) {
			return nil, errInvalidAudioStream
		}
		if entry.Name == "" {
			entry.Name = inferredName(entry.Source, inferredRoot)
		}
		if entry.Profile == "" {
			entry.Profile = defaultProfile
		}
		if entry.Name == "" || strings.HasPrefix(entry.Name, "karty.") || names[entry.Name] {
			return nil, errDuplicateAudioStream
		}
		names[entry.Name] = true
		sources[entry.Source] = true
	}
	return result, nil
}

//nolint:wsl_v5,nlreturn // Layout merging keeps duplicate handling explicit.
func mergeLayouts(directory string, discovered, overrides []Layout) ([]Layout, error) {
	result := append([]Layout(nil), discovered...)
	seen := make(map[string]bool, len(result))
	for _, entry := range result {
		seen[entry.Source] = true
	}
	declared := make(map[string]bool, len(overrides))
	for _, entry := range overrides {
		if entry.Source == "" || validateConfinedFile(directory, entry.Source) != nil {
			return nil, errInvalidLayout
		}
		if declared[entry.Source] {
			return nil, errInvalidLayout
		}
		declared[entry.Source] = true
		if seen[entry.Source] {
			continue
		}
		seen[entry.Source] = true
		result = append(result, entry)
	}
	return result, nil
}

//nolint:wsl_v5,nlreturn // Font merging keeps role precedence and validation explicit.
func mergeFonts(directory string, discovered, overrides []Font) ([]Font, error) {
	result := append([]Font(nil), discovered...)
	byRole := make(map[string]int, len(result))
	for index, entry := range result {
		byRole[entry.Role] = index
	}
	for _, entry := range overrides {
		if index, exists := byRole[entry.Role]; exists {
			result[index] = entry
		} else {
			byRole[entry.Role] = len(result)
			result = append(result, entry)
		}
	}
	for _, font := range result {
		extension := strings.ToLower(filepath.Ext(font.Source))
		validRole := font.Role == "body" || font.Role == "display" || font.Role == "mono"
		if !validRole || (extension != ".ttf" && extension != ".otf") || validateConfinedFile(directory, font.Source) != nil {
			return nil, fmt.Errorf("font role %q: %w", font.Role, errInvalidFont)
		}
	}
	return result, nil
}
