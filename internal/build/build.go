// Package build produces self-describing game and level cartridges.
package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-ui/codegen"
	"github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty-ui/schema"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/assetusage"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
	"golang.org/x/image/font/sfnt"
)

type staticError string

func (err staticError) Error() string {
	return string(err)
}

const (
	errUnsupportedTarget      staticError = "unsupported build target"
	errHostNotFile            staticError = "host artifact is not a file"
	errWebHostRequired        staticError = "web target requires the SDK-pinned public host; install the selected SDK or pass --host /path/to/karty-host.wasm"
	errGoWebCompiler          staticError = "compiler=go is not supported for web targets yet; use compiler=tinygo"
	errRegistrationCollision  staticError = "legacy registration file conflicts with the user-owned karty_register export; remove or rename it before building"
	errSoundIdentifier        staticError = "sound names produce the same generated Go identifier"
	errCachedAssetChanged     staticError = "cached processor output changed after validation"
	errVideoSDKRequired       staticError = "videos require an SDK with video/mpeg1@1 support (SDK 0.0.5+)"
	errAudioStreamSDKRequired staticError = "music/environment require an SDK with audio-stream/qoa@1 support (SDK 0.0.5+)"
)

// Options configures build tool overrides.
type Options struct {
	Go        string
	TinyGo    string
	WasmTools string
	Host      string
	Target    string
	Platform  string
	AirProxy  bool
}

// Run validates a project and compiles its self-describing cartridges.
func Run(ctx context.Context, directory string) error {
	return RunWithOptions(ctx, directory, Options{})
}

// RunWithOptions builds a project using its SDK-pinned toolchain.
//
//nolint:gocognit,gocyclo,maintidx,golines,wsl_v5 // This function intentionally keeps the transactional build sequence visible.
func RunWithOptions(ctx context.Context, directory string, options Options) error {
	if options.Platform != "" {
		if options.Target == "web" {
			return fmt.Errorf("--platform applies only to native builds: %w", os.ErrInvalid)
		}

		if _, err := toolchain.NativePlatform(options.Platform); err != nil {
			return err
		}

		if options.Target == "" {
			options.Target = "native"
		}
	}

	config, err := project.Load(directory)
	if err != nil {
		return err
	}

	manifest, err := sdk.Resolve(config.SDK.Version)
	if err != nil {
		return err
	}
	if manifest.Assets.TextureProfiles[project.DefaultTextureProfile].Processor == asset.ProcessorCopyPNGv1 {
		config.Assets.Textures = slices.DeleteFunc(config.Assets.Textures, func(texture project.Texture) bool {
			return texture.Inferred && strings.ToLower(filepath.Ext(texture.Source)) != ".png"
		})
	}

	if len(config.Assets.Videos) > 0 && !slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityVideoMPEG1v1) {
		return errVideoSDKRequired
	}
	if (len(config.Assets.Music) > 0 || len(config.Assets.Environments) > 0) &&
		!slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityAudioStreamQOAv1) {
		return errAudioStreamSDKRequired
	}
	if config.Project.Compiler == "go" && options.Target == "web" {
		return errGoWebCompiler
	}

	_, err = toolchain.Ensure(ctx, manifest, toolchain.EnsureOptions{
		GoOverride:        options.Go,
		TinyGoOverride:    options.TinyGo,
		WasmToolsOverride: options.WasmTools,
		NeedGo:            config.Project.Compiler == "go",
		NeedTinyGo:        config.Project.Compiler != "go",
		NeedWasmTools:     true,
	})
	if err != nil {
		return fmt.Errorf("prepare SDK toolchain: %w", err)
	}

	var (
		compiler     string
		compilerPath string
	)

	compiler, compilerPath, err = resolveCompiler(ctx, config.Project.Compiler, manifest, options)
	if err != nil {
		return err
	}

	wasmTools := options.WasmTools
	if wasmTools == "" {
		wasmTools, err = toolchain.WasmTools(manifest.Tools.WasmTools)
		if err != nil {
			return fmt.Errorf("resolve wasm-tools: %w", err)
		}
	}

	sourceDirectory := filepath.Join(directory, "src")
	if _, statErr := os.Stat(sourceDirectory); statErr != nil {
		return fmt.Errorf("client source: %w", statErr)
	}

	modulePath, err := project.ModulePath(directory)
	if err != nil {
		return err
	}

	generated, err := sdk.ClientFiles(manifest, compiler)
	if err != nil {
		return fmt.Errorf("generate client API: %w", err)
	}

	textureNames := make([]string, 0, len(config.Assets.Textures))
	for _, texture := range config.Assets.Textures {
		textureNames = append(textureNames, texture.Name)
	}

	engineImport := modulePath + "/.karty/engine"
	assetFile, err := codegen.TextureAssetPackageFile(textureNames, engineImport)
	if err != nil {
		return fmt.Errorf("generate typed assets: %w", err)
	}

	generated["assets/textures.go"] = assetFile
	if manifest.API.Version == "0.0.1" {
		generated["engine/assets.go"], err = codegen.TextureAssetFile(textureNames)
		if err != nil {
			return fmt.Errorf("generate legacy typed assets: %w", err)
		}
	}

	generated["assets/sounds.go"], err = soundAssetFile(config.Assets.Sounds, engineImport)
	if err != nil {
		return fmt.Errorf("generate typed sounds: %w", err)
	}

	if len(config.Assets.Videos) > 0 {
		generated["assets/videos.go"], err = videoAssetFile(config.Assets.Videos, engineImport)
		if err != nil {
			return err
		}
	}
	if len(config.Assets.Music) > 0 || len(config.Assets.Environments) > 0 {
		generated["assets/music.go"], err = audioStreamAssetFile("MusicID", "Music", "music", engineImport, config.Assets.Music)
		if err != nil {
			return err
		}
		generated["assets/environments.go"], err = audioStreamAssetFile("EnvironmentID", "Environment", "environment", engineImport, config.Assets.Environments)
		if err != nil {
			return err
		}
	}

	var views []uicompiler.Component

	names := make([]string, 0, len(config.Assets.UI))
	for _, asset := range config.Assets.UI {
		names = append(names, asset.Name)
	}

	generated["assets/ui.go"], err = codegen.UIAssetPackageFile(names, engineImport)
	if err != nil {
		return err
	}
	if manifest.API.Version == "0.0.1" {
		generated["engine/ui-assets.go"], err = codegen.UIAssetFile(names)
		if err != nil {
			return err
		}
	}

	views, err = project.CompileUI(
		directory, config.Assets.UI, config.Assets.Layouts, config.Assets.Theme.Source,
	)
	if err != nil {
		return err
	}

	generated["engine/ui-views.go"], err = codegen.UIViewFile(views)
	if err != nil {
		return err
	}

	if err := writeGeneratedClientAPI(directory, generated); err != nil {
		return err
	}

	if err := sdk.SyncDocs(directory, manifest); err != nil {
		return err
	}

	if err := writeUIPackage(directory, modulePath, views); err != nil {
		return err
	}

	if err := retireLegacyGeneratedRegistration(directory); err != nil {
		return err
	}

	if err := removeLegacyGeneratedClientAPI(directory); err != nil {
		return err
	}

	theme := uicompiler.DefaultTheme()

	if config.Assets.Theme.Source != "" {
		data, readErr := uicompiler.ReadSource(directory, config.Assets.Theme.Source)
		if readErr != nil {
			return readErr
		}

		theme, err = uicompiler.ParseTheme(config.Assets.Theme.Source, data)
		if err != nil {
			return err
		}
	}

	assetReport, err := analyzeProjectAssets(
		ctx, directory, modulePath, manifest,
		config.Assets.Textures, config.Assets.Sounds, textureNames, theme,
	)
	if err != nil {
		return err
	}
	assetReport.AudioStreams, err = assetpipeline.ProcessAudioStreams(ctx, directory, manifest, config.Assets.Music, config.Assets.Environments)
	if err != nil {
		return fmt.Errorf("process streaming audio: %w", err)
	}

	for _, stream := range assetReport.AudioStreams {
		assetReport.Summary.AudioStreamSourceBytes += stream.SourceBytes
		assetReport.Summary.AudioStreamOutputBytes += stream.OutputBytes
	}
	assetReport.Summary.AudioStreamCount = len(assetReport.AudioStreams)
	assetReport.Summary.AssetSourceBytes += assetReport.Summary.AudioStreamSourceBytes
	assetReport.Summary.AssetOutputBytes += assetReport.Summary.AudioStreamOutputBytes

	fontRoles := usedFontRoles(views)
	if err := analyzeProjectFonts(directory, config.Assets.Fonts, fontRoles, &assetReport); err != nil {
		return err
	}

	distributionDirectory := filepath.Join(directory, "dist")
	rawDirectory := filepath.Join(distributionDirectory, "raw")

	mkdirErr := os.MkdirAll(rawDirectory, 0o750)
	if mkdirErr != nil {
		return fmt.Errorf("create raw distribution directory: %w", mkdirErr)
	}

	artifact := filepath.Join(rawDirectory, "game.kart")

	var clientSource string

	var cleanup func()

	clientSource, cleanup, err = stageUIClient(directory, modulePath, views)
	if err != nil {
		return err
	}
	defer cleanup()

	// Go can treat an existing output with the same build ID as current. Karty
	// postprocesses that output with custom sections, so retaining it would make
	// a repeated build try to embed those sections twice.
	if err := os.Remove(artifact); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove previous raw client artifact: %w", err)
	}

	arguments := compilerArguments(compiler, artifact)
	arguments[len(arguments)-1] = clientSource
	command := exec.CommandContext(ctx, compilerPath, arguments...)
	command.Dir = directory
	command.Env = compilerEnvironment(compiler)

	if output, commandErr := command.CombinedOutput(); commandErr != nil {
		return fmt.Errorf("compile client: %w\n%s", commandErr, output)
	}

	if err := embedProjectAssets(
		directory, artifact, assetReport.Textures, assetReport.Sounds, config.Assets.Fonts, fontRoles,
	); err != nil {
		return err
	}

	if err := embedUIAssets(
		directory, artifact, config.Assets.UI, config.Assets.Layouts, config.Assets.Theme.Source,
	); err != nil {
		return err
	}

	if err := validateClientArtifact(ctx, directory, wasmTools, artifact); err != nil {
		return err
	}

	if err := removeStagedAssets(rawDirectory); err != nil {
		return err
	}

	uiSchema := ui.SchemaInteractionPolish

	levels, err := buildAndStageLevels(ctx, directory, rawDirectory, uiSchema, config.Assets.Theme.Source, manifest)
	if err != nil {
		return err
	}

	if err := includeLevelAssets(&assetReport, levels); err != nil {
		return err
	}

	if len(config.Assets.Videos) > 0 {
		if err := stageProjectVideos(directory, rawDirectory, artifact, config.Assets.Videos); err != nil {
			return err
		}
		assetReport.Features = append(assetReport.Features, cartridge.FeatureVideoMPEG1v1)
		slices.Sort(assetReport.Features)
	}
	if len(assetReport.AudioStreams) > 0 {
		if err := stageProjectAudioStreams(rawDirectory, artifact, assetReport.AudioStreams); err != nil {
			return err
		}
		assetReport.Features = append(assetReport.Features, cartridge.FeatureAudioStreamQOAv1)
		slices.Sort(assetReport.Features)
	}
	if err := writeAssetReport(rawDirectory, assetReport); err != nil {
		return err
	}

	if err := embedProjectManifest(artifact, config, compiler, levels, assetReport.Features); err != nil {
		return err
	}

	if err := removeLegacyRawCatalog(rawDirectory); err != nil {
		return err
	}

	if err := validateClientArtifact(ctx, directory, wasmTools, artifact); err != nil {
		return err
	}

	if err := stageRequestedTarget(ctx, manifest, distributionDirectory, rawDirectory, artifact, levels, options); err != nil {
		return err
	}

	if err := removeLegacyRootOutput(distributionDirectory); err != nil {
		return err
	}

	return nil
}

func removeLegacyRawCatalog(rawDirectory string) error {
	if err := os.Remove(filepath.Join(rawDirectory, "catalog.json")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy raw catalog: %w", err)
	}

	return nil
}

func stageRequestedTarget(
	ctx context.Context, manifest sdk.Manifest,
	distributionDirectory, rawDirectory, artifact string,
	levels []levelbuild.Artifact,
	options Options,
) error {
	if options.Target != "" && options.Target != "native" && options.Target != "web" {
		return fmt.Errorf("%q: %w", options.Target, errUnsupportedTarget)
	}

	if options.Host == "" && options.Target == "" {
		return nil
	}

	localHost := options.Host != ""

	hostTarget := options.Target
	if options.Platform != "" {
		hostTarget = options.Platform
	}

	if options.Host == "" {
		var (
			host string
			err  error
		)
		if options.Target == "web" {
			host, localHost, err = resolveWebHost(ctx, manifest)
		} else {
			host, err = toolchain.EnsureHost(ctx, manifest, hostTarget)
		}

		if err != nil {
			return err
		}

		options.Host = host
	}

	var webRuntime string

	if options.Target == "web" {
		var err error

		webRuntime, err = resolveWebRuntime(ctx, manifest, options.Host, localHost)
		if err != nil {
			return err
		}
	}

	return stageTarget(
		distributionDirectory, rawDirectory, artifact, levels,
		options.Host, options.Target, webRuntime, options.Platform, options.AirProxy,
	)
}

func resolveWebRuntime(ctx context.Context, manifest sdk.Manifest, host string, local bool) (string, error) {
	if local {
		path := filepath.Join(filepath.Dir(host), "wasm_exec.js")
		if _, err := inspectHostArtifact(path); err != nil {
			return "", fmt.Errorf("local web host requires matching wasm_exec.js alongside it: %w", err)
		}

		return path, nil
	}

	path, err := toolchain.EnsureHost(ctx, manifest, "web-runtime")
	if err != nil {
		return "", fmt.Errorf("resolve SDK web runtime: %w", err)
	}

	return path, nil
}

func resolveWebHost(ctx context.Context, manifest sdk.Manifest) (string, bool, error) {
	host, err := findWebHostArtifact()
	if err == nil {
		return host, true, nil
	}

	host, err = toolchain.EnsureHost(ctx, manifest, "web")
	if err != nil {
		return "", false, fmt.Errorf("web target: %w", errWebHostRequired)
	}

	return host, false, nil
}

//nolint:wsl_v5,nlreturn // The bounded host search is intentionally sequential.
func findWebHostArtifact() (string, error) {
	if environment := os.Getenv("KARTY_HOST_WEB"); environment != "" {
		if mode, err := inspectHostArtifact(environment); err == nil && mode.IsRegular() {
			return environment, nil
		}
	}
	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), "..", "..", "host", "web", "karty-host.wasm")
		if mode, inspectErr := inspectHostArtifact(candidate); inspectErr == nil && mode.IsRegular() {
			return candidate, nil
		}
	}
	return "", errWebHostRequired
}

func removeLegacyRootOutput(distributionDirectory string) error {
	for _, name := range []string{"game.kart", "client.wasm", "catalog.json", "asset-report.json", "assets", "content"} {
		path := filepath.Join(distributionDirectory, name)
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove legacy root output %s: %w", name, err)
		}
	}

	return nil
}

func buildAndStageLevels(
	ctx context.Context,
	directory, buildDirectory string,
	uiSchema uint32,
	themeSource string,
	manifest sdk.Manifest,
) ([]levelbuild.Artifact, error) {
	levels, err := levelbuild.BuildAllWithAssets(ctx, directory, uiSchema, themeSource, manifest)
	if err != nil {
		return nil, fmt.Errorf("build levels: %w", err)
	}

	if err := stageLevels(buildDirectory, levels, ""); err != nil {
		return nil, err
	}

	return levels, nil
}

func includeLevelAssets(report *assetpipeline.Report, levels []levelbuild.Artifact) error {
	features := slices.Clone(report.Features)
	for _, builtLevel := range levels {
		features = append(features, builtLevel.Features...)

		var levelDecodedBytes int64

		for _, texture := range builtLevel.Textures {
			report.Levels = append(report.Levels, assetpipeline.LevelTexture{Level: builtLevel.Name, Texture: texture})
			report.Summary.LevelTextureCount++
			report.Summary.LevelSourceBytes += texture.SourceBytes
			report.Summary.LevelOutputBytes += texture.OutputBytes
			report.Summary.EstimatedDecodedLevelBytes += texture.EstimatedDecodedBytes
			levelDecodedBytes += texture.EstimatedDecodedBytes
		}

		if report.Summary.EstimatedDecodedBytes+levelDecodedBytes > asset.MaxDecodedTextures {
			return fmt.Errorf("level %q and game textures: %w", builtLevel.Name, assetpipeline.ErrAssetResourceLimits)
		}
	}

	slices.Sort(features)
	report.Features = slices.Compact(features)
	report.Summary.AssetSourceBytes += report.Summary.LevelSourceBytes
	report.Summary.AssetOutputBytes += report.Summary.LevelOutputBytes

	return nil
}

func analyzeProjectAssets(
	ctx context.Context,
	directory, modulePath string,
	manifest sdk.Manifest,
	textures []project.Texture,
	sounds []project.Sound,
	names []string,
	theme uicompiler.Theme,
) (assetpipeline.Report, error) {
	usage := assetusage.Analyze(directory, modulePath, names)
	for _, name := range theme.ImageNames() {
		usage.Live[name] = "theme image reference"
	}

	liveTextures, usageReport, err := selectTextures(directory, textures, usage)
	if err != nil {
		return assetpipeline.Report{}, err
	}

	report, err := assetpipeline.ProcessGameAssets(ctx, directory, manifest, liveTextures, sounds)
	if err != nil {
		return assetpipeline.Report{}, fmt.Errorf("analyze assets: %w", err)
	}

	dimensions := make(map[string][2]int, len(report.Textures))
	for _, texture := range report.Textures {
		dimensions[texture.Name] = [2]int{texture.Width, texture.Height}
	}

	if err := theme.ValidateGameImages(dimensions); err != nil {
		return assetpipeline.Report{}, err
	}

	report.Usage = usageReport
	applyUsageSummary(&report, usageReport)

	return report, nil
}

func selectTextures(
	directory string,
	textures []project.Texture,
	usage assetusage.Result,
) ([]project.Texture, []assetpipeline.Usage, error) {
	live := make([]project.Texture, 0, len(textures))
	report := make([]assetpipeline.Usage, 0, len(textures))

	for _, texture := range textures {
		info, err := os.Stat(filepath.Join(directory, filepath.FromSlash(texture.Source)))
		if err != nil {
			return nil, nil, fmt.Errorf("inspect texture %q for usage report: %w", texture.Name, err)
		}

		reason, used := usage.Live[texture.Name]
		entry := assetpipeline.Usage{Name: texture.Name, SourceBytes: info.Size()}

		switch {
		case used:
			entry.Status = "used"
			entry.Reason = reason

			live = append(live, texture)
		case texture.Keep:
			entry.Status = "kept"
			entry.Reason = "explicit keep declaration"

			live = append(live, texture)
		case usage.KeepAll:
			entry.Status = "kept"
			entry.Reason = usage.Diagnostic

			live = append(live, texture)
		default:
			entry.Status = "stripped"
			entry.Reason = "no reachable typed or static reference"
		}

		report = append(report, entry)
	}

	return live, report, nil
}

func applyUsageSummary(report *assetpipeline.Report, usage []assetpipeline.Usage) {
	report.Summary.DeclaredTextureCount = len(usage)
	for _, entry := range usage {
		switch entry.Status {
		case "used":
			report.Summary.UsedTextureCount++
		case "kept":
			report.Summary.KeptTextureCount++
		case "stripped":
			report.Summary.StrippedTextureCount++
			report.Summary.StrippedSourceBytes += entry.SourceBytes
		}
	}
}

func analyzeProjectFonts(
	directory string,
	fonts []project.Font,
	used map[string]bool,
	report *assetpipeline.Report,
) error {
	report.Fonts = make([]assetpipeline.FontUsage, 0, len(fonts))
	report.Summary.DeclaredFontCount = len(fonts)

	for _, font := range fonts {
		path := filepath.Join(directory, filepath.FromSlash(font.Source))

		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s font for validation: %w", font.Role, err)
		}

		if _, err := sfnt.Parse(contents); err != nil {
			return fmt.Errorf("decode %s font %q: %w", font.Role, font.Source, err)
		}

		entry := assetpipeline.FontUsage{
			Role: font.Role, Source: font.Source, SourceBytes: int64(len(contents)),
		}
		report.Summary.FontSourceBytes += entry.SourceBytes

		if used[font.Role] {
			entry.Status = "used"
			entry.Reason = "reachable semantic font-family role"
			report.Summary.UsedFontCount++
		} else {
			entry.Status = "stripped"
			entry.Reason = "no reachable semantic font-family role"
			report.Summary.StrippedFontCount++
			report.Summary.StrippedFontBytes += entry.SourceBytes
		}

		report.Fonts = append(report.Fonts, entry)
	}

	return nil
}

func compilerArguments(compiler, artifact string) []string {
	if compiler == "tinygo" {
		// Lifecycle callbacks are synchronous. The default TinyGo scheduler
		// allocates a task/stack for each WASM export invocation.
		return []string{"build", "-target=wasi", "-buildmode=c-shared", "-scheduler=none", "-o", artifact, "./src"}
	}

	return []string{"build", "-buildmode=c-shared", "-o", artifact, "./src"}
}

func compilerEnvironment(compiler string) []string {
	environment := setEnvironmentValue(os.Environ(), "GOWORK", "off")

	goFlags := strings.Fields(os.Getenv("GOFLAGS"))
	if !slices.Contains(goFlags, "-buildvcs=false") {
		goFlags = append(goFlags, "-buildvcs=false")
	}

	environment = setEnvironmentValue(environment, "GOFLAGS", strings.Join(goFlags, " "))
	if compiler == "go" {
		// The CLI can run under a different Go release than the SDK-pinned
		// compiler. Let that compiler discover its own GOROOT instead of
		// inheriting the CLI toolchain's standard library.
		environment = removeEnvironmentValue(environment, "GOROOT")
		environment = setEnvironmentValue(environment, "GOOS", "wasip1")
		environment = setEnvironmentValue(environment, "GOARCH", "wasm")
	}

	return environment
}

func removeEnvironmentValue(environment []string, name string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment))

	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}

	return result
}

func setEnvironmentValue(environment []string, name, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment)+1)

	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}

	return append(result, prefix+value)
}

func writeGeneratedClientAPI(directory string, generated map[string][]byte) error {
	root := filepath.Join(directory, ".karty")
	if err := os.RemoveAll(filepath.Join(root, "assets")); err != nil {
		return fmt.Errorf("replace generated assets package: %w", err)
	}

	for _, name := range []string{"assets.go", "sounds.go", "videos.go", "music.go", "environments.go", "ui-assets.go"} {
		if err := os.Remove(filepath.Join(root, "engine", name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove legacy generated asset file %s: %w", name, err)
		}
	}

	for relativePath, contents := range generated {
		path := filepath.Join(root, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("create generated client API directory: %w", err)
		}

		if err := os.WriteFile(path, contents, 0o600); err != nil {
			return fmt.Errorf("write generated client API: %w", err)
		}
	}

	return nil
}

func soundAssetFile(sounds []project.Sound, engineImport string) ([]byte, error) {
	sorted := slices.Clone(sounds)
	slices.SortFunc(sorted, func(left, right project.Sound) int { return strings.Compare(left.Name, right.Name) })

	var source strings.Builder
	source.WriteString("// Code generated by Karty. DO NOT EDIT.\n" +
		"// WARNING: DO NOT MODIFY THIS FILE DIRECTLY.\n" +
		"// WARNING: MANUAL CHANGES WILL BE OVERWRITTEN.\n" +
		"// Source of truth: sound declarations in karty.toml.\n" +
		"// Change the sound declarations, then regenerate with: karty build\n\n" +
		"package assets\n")

	if len(sorted) > 0 {
		fmt.Fprintf(&source, "\nimport engine %q\n\nconst (\n", engineImport)
	}

	identifiers := make(map[string]string, len(sorted))
	for index, sound := range sorted {
		identifier := "Sound" + exportedAssetIdentifier(sound.Name)
		if previous, exists := identifiers[identifier]; exists {
			return nil, fmt.Errorf("%q and %q -> %s: %w", previous, sound.Name, identifier, errSoundIdentifier)
		}

		identifiers[identifier] = sound.Name
		fmt.Fprintf(&source, "\t%s engine.SoundID = %d\n", identifier, index+1)
	}

	if len(sorted) > 0 {
		source.WriteString(")\n")
	}

	formatted, err := format.Source([]byte(source.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated sound constants: %w", err)
	}

	return formatted, nil
}

func exportedAssetIdentifier(name string) string {
	var (
		result    strings.Builder
		upperNext = true
	)

	for _, character := range name {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			upperNext = true

			continue
		}

		if upperNext {
			result.WriteRune(unicode.ToUpper(character))

			upperNext = false
		} else {
			result.WriteRune(character)
		}
	}

	return result.String()
}

func retireLegacyGeneratedRegistration(directory string) error {
	registered, err := sourceExportsRegistration(filepath.Join(directory, "src"))
	if err != nil || !registered {
		return err
	}

	path := filepath.Join(directory, "src", "karty_runtime.gen.go")

	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("inspect generated registration: %w", err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s: %w", path, errRegistrationCollision)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read generated registration: %w", err)
	}

	if !strings.HasPrefix(string(contents), "// Code generated by Karty. DO NOT EDIT.\n") {
		return fmt.Errorf("%s: %w", path, errRegistrationCollision)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove legacy generated registration: %w", err)
	}

	return nil
}

func sourceExportsRegistration(directory string) (bool, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false, fmt.Errorf("read client source directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || entry.Name() == "karty_runtime.gen.go" {
			continue
		}

		path := filepath.Join(directory, entry.Name())

		contents, err := os.ReadFile(path)
		if err != nil {
			return false, fmt.Errorf("read client source %s: %w", entry.Name(), err)
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), path, contents, parser.ParseComments)
		if err != nil {
			return false, fmt.Errorf("parse client source %s: %w", entry.Name(), err)
		}

		object := parsed.Scope.Lookup("main")
		if object == nil {
			continue
		}

		mainFunction, ok := object.Decl.(*ast.FuncDecl)
		if ok && hasRegistrationDirective(mainFunction.Doc) {
			return true, nil
		}
	}

	return false, nil
}

func hasRegistrationDirective(documentation *ast.CommentGroup) bool {
	if documentation == nil {
		return false
	}

	for _, comment := range documentation.List {
		if strings.TrimSpace(comment.Text) == "//go:wasmexport karty_register" {
			return true
		}
	}

	return false
}

func removeLegacyGeneratedClientAPI(directory string) error {
	paths := make([]string, 0, 5)

	paths = append(paths,
		filepath.Join(directory, "api_codegen.go"),
		filepath.Join(directory, "src", "api_codegen.go"),
	)
	for _, root := range []string{
		filepath.Join(directory, "src", "engine"),
		filepath.Join(directory, ".karty", "engine"),
	} {
		for _, name := range []string{"game_codegen.go", "components_codegen.go"} {
			paths = append(paths, filepath.Join(root, name))
		}
	}

	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove legacy generated client API: %w", err)
		}
	}

	return nil
}

func resolveCompiler(ctx context.Context, compiler string, manifest sdk.Manifest, options Options) (string, string, error) {
	if compiler == "go" {
		path, err := toolchain.Go(ctx, toolchain.GoOptions{Override: options.Go, Version: manifest.Tools.Go})
		if err != nil {
			return "", "", fmt.Errorf("resolve Go: %w", err)
		}

		return "go", path, nil
	}

	path := options.TinyGo
	if path == "" {
		var err error

		path, err = toolchain.TinyGo(toolchain.TinyGoOptions{Version: manifest.Tools.TinyGo})
		if err != nil {
			return "", "", fmt.Errorf("resolve TinyGo: %w", err)
		}
	}

	return "tinygo", path, nil
}

func stageTarget(
	distributionDirectory, rawDirectory, clientArtifact string,
	levels []levelbuild.Artifact,
	hostArtifact, target, webRuntime, platform string,
	airProxy bool,
) error {
	if target == "" {
		target = "native"
	}

	if target != "native" && target != "web" {
		return fmt.Errorf("%q: %w", target, errUnsupportedTarget)
	}

	hostMode, err := inspectHostArtifact(hostArtifact)
	if err != nil {
		return err
	}

	if target == "native" && hostArtifact == "" {
		return errHostNotFile
	}

	targetDirectory := filepath.Join(distributionDirectory, target)
	if platform != "" {
		targetDirectory = filepath.Join(targetDirectory, platform)
	}

	if err := os.MkdirAll(targetDirectory, 0o750); err != nil {
		return fmt.Errorf("create %s target directory: %w", target, err)
	}

	if err := stageTargetAssets(targetDirectory, clientArtifact); err != nil {
		return err
	}

	if err := stageLevels(targetDirectory, levels, rawDirectory); err != nil {
		return err
	}

	if err := stageVideoFiles(targetDirectory, clientArtifact, rawDirectory); err != nil {
		return err
	}

	if err := stageAudioStreamFiles(targetDirectory, clientArtifact, rawDirectory); err != nil {
		return err
	}

	if err := stageHostArtifact(targetDirectory, hostArtifact, target, platform, hostMode); err != nil {
		return err
	}

	if err := os.Remove(filepath.Join(targetDirectory, "catalog.json")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy target catalog: %w", err)
	}

	if target == "web" {
		for _, stale := range []string{"wasm_exec.js"} {
			if err := os.Remove(filepath.Join(targetDirectory, stale)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove stale web artifact: %w", err)
			}
		}

		if err := stageWebShell(targetDirectory, webRuntime, airProxy); err != nil {
			return err
		}
	}

	return nil
}

//nolint:gosec // The caller supplies an explicit host path or a fixed packaged location.
func inspectHostArtifact(path string) (os.FileMode, error) {
	if path == "" {
		return 0, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("inspect host artifact: %w", err)
	}

	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("%s: %w", path, errHostNotFile)
	}

	return info.Mode().Perm(), nil
}

func stageTargetAssets(targetDirectory, clientArtifact string) error {
	if err := copyFile(clientArtifact, filepath.Join(targetDirectory, "game.kart"), 0o600); err != nil {
		return fmt.Errorf("stage client artifact: %w", err)
	}

	if err := removeLegacyClientArtifact(targetDirectory); err != nil {
		return err
	}

	return removeStagedAssets(targetDirectory)
}

func validateClientArtifact(ctx context.Context, directory, wasmTools, artifact string) error {
	validate := exec.CommandContext(ctx, wasmTools, "validate", artifact)
	validate.Dir = directory

	if output, err := validate.CombinedOutput(); err != nil {
		return fmt.Errorf("validate client artifact: %w\n%s", err, output)
	}

	return removeLegacyClientArtifact(filepath.Dir(artifact))
}

func removeLegacyClientArtifact(directory string) error {
	if err := os.Remove(filepath.Join(directory, "client.wasm")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy client artifact: %w", err)
	}

	return nil
}

func stageHostArtifact(targetDirectory, hostArtifact, target, platform string, mode os.FileMode) error {
	if hostArtifact == "" {
		return nil
	}

	name := toolchain.NativeHostName(platform)
	if target == "web" {
		name = "karty-host.wasm"
	}

	if err := copyFile(hostArtifact, filepath.Join(targetDirectory, name), mode); err != nil {
		return fmt.Errorf("stage host artifact: %w", err)
	}

	return nil
}

func embedProjectAssets(
	directory, artifact string,
	textures []assetpipeline.Texture,
	sounds []assetpipeline.Sound,
	fonts []project.Font,
	usedFonts map[string]bool,
) error {
	assets := make([]cartridge.Asset, 0, len(textures)+len(fonts))
	for _, texture := range textures {
		contents, err := readVerifiedProcessedAsset(texture.SourcePath, texture.OutputSHA256, texture.OutputBytes)
		if err != nil {
			return fmt.Errorf("read texture %q for cartridge: %w", texture.Name, err)
		}

		assets = append(assets, cartridge.Asset{Name: texture.Name, Bytes: contents})
	}

	for _, font := range fonts {
		if !usedFonts[font.Role] {
			continue
		}

		contents, err := os.ReadFile(filepath.Join(directory, font.Source))
		if err != nil {
			return fmt.Errorf("read %s font for cartridge: %w", font.Role, err)
		}

		assets = append(assets, cartridge.Asset{Name: ui.FontAssetPrefix + font.Role, Bytes: contents})
	}

	bundle, err := cartridge.EncodeAssets(assets)
	if err != nil {
		return fmt.Errorf("encode game assets: %w", err)
	}

	wasm, err := os.ReadFile(artifact)
	if err != nil {
		return fmt.Errorf("read game cartridge: %w", err)
	}

	embedded, err := cartridge.EmbedAssets(wasm, bundle)
	if err != nil {
		return fmt.Errorf("embed game assets: %w", err)
	}

	if len(sounds) > 0 {
		catalog := make([]cartridge.Sound, 0, len(sounds))
		for _, sound := range sounds {
			contents, readErr := readVerifiedProcessedAsset(sound.SourcePath, sound.OutputSHA256, sound.OutputBytes)
			if readErr != nil {
				return fmt.Errorf("read sound %q for cartridge: %w", sound.Name, readErr)
			}

			catalog = append(catalog, cartridge.Sound{
				ID: sound.ID, Name: sound.Name, Codec: cartridge.SoundCodecQOA,
				Channels: sound.Metadata.Channels, SampleRate: sound.Metadata.SampleRate,
				Frames: sound.Metadata.Frames, Bytes: contents,
			})
		}

		soundBundle, encodeErr := cartridge.EncodeSounds(catalog)
		if encodeErr != nil {
			return fmt.Errorf("encode game sounds: %w", encodeErr)
		}

		embedded, err = cartridge.EmbedSounds(embedded, soundBundle)
		if err != nil {
			return fmt.Errorf("embed game sounds: %w", err)
		}
	}
	//nolint:gosec // artifact is the fixed dist/raw/game.kart path owned by this build.
	if err := os.WriteFile(artifact, embedded, 0o600); err != nil {
		return fmt.Errorf("write game cartridge assets: %w", err)
	}

	return nil
}

func readVerifiedProcessedAsset(path, expectedDigest string, expectedBytes int64) ([]byte, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	digest := sha256.Sum256(contents)
	if int64(len(contents)) != expectedBytes || hex.EncodeToString(digest[:]) != expectedDigest {
		return nil, errCachedAssetChanged
	}

	return contents, nil
}

func usedFontRoles(views []uicompiler.Component) map[string]bool {
	used := make(map[string]bool, 3)

	for _, view := range views {
		for _, element := range view.Template.Elements {
			if element.Kind != "label" && element.Kind != "button" && element.Kind != "list" {
				continue
			}

			used[fontRoleName(element.Style)] = true
			if element.ResponsiveStyle.Set|element.ResponsiveStyle.Set2 != 0 {
				used[fontRoleName(element.ResponsiveStyle)] = true
			}
		}
	}

	return used
}

func fontRoleName(style ui.Style) string {
	if style.Set&ui.StyleFontFamily == 0 {
		return "body"
	}

	switch style.FontFamily {
	case ui.FontFamilyDisplay:
		return "display"
	case ui.FontFamilyMono:
		return "mono"
	default:
		return "body"
	}
}

func embedProjectManifest(
	artifact string,
	config project.Config,
	compiler string,
	levels []levelbuild.Artifact,
	features []string,
) error {
	dependencies := make([]cartridge.LevelDependency, 0, len(levels))
	for _, level := range levels {
		dependencies = append(dependencies, cartridge.LevelDependency{
			Name:            level.Name,
			Kind:            level.Kind,
			ContentSHA256:   level.ContentSHA256,
			Size:            uint64(len(level.Bytes)),
			EnvelopeVersion: level.EnvelopeVersion,
		})
	}

	slices.SortFunc(dependencies, func(left, right cartridge.LevelDependency) int {
		return strings.Compare(left.Name, right.Name)
	})

	manifest, err := cartridge.EncodeManifest(cartridge.Manifest{
		ProjectName: config.Project.Name,
		Compiler:    compiler,
		Width:       uint32(config.Project.Resolution.Width),
		Height:      uint32(config.Project.Resolution.Height),
		Features:    slices.Clone(features),
		Levels:      dependencies,
	})
	if err != nil {
		return fmt.Errorf("encode game manifest: %w", err)
	}

	wasm, err := os.ReadFile(artifact)
	if err != nil {
		return fmt.Errorf("read game cartridge for manifest: %w", err)
	}

	embedded, err := cartridge.EmbedSection(wasm, cartridge.ManifestSectionName, manifest)
	if err != nil {
		return fmt.Errorf("embed game manifest: %w", err)
	}
	//nolint:gosec // artifact is the fixed dist/raw/game.kart path owned by this build.
	if err := os.WriteFile(artifact, embedded, 0o600); err != nil {
		return fmt.Errorf("write game cartridge manifest: %w", err)
	}

	return nil
}

func removeStagedAssets(root string) error {
	for _, directory := range []string{"assets", "audio", "video"} {
		if err := os.RemoveAll(filepath.Join(root, directory)); err != nil {
			return fmt.Errorf("remove separately staged %s: %w", directory, err)
		}
	}

	return nil
}

func stageLevels(root string, artifacts []levelbuild.Artifact, sourceRoot string) error {
	staging, err := os.MkdirTemp(root, ".content-stage-")
	if err != nil {
		return fmt.Errorf("create level staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	for _, artifact := range artifacts {
		name := artifact.ContentSHA256 + ".kld"

		path := filepath.Join(staging, name)
		if sourceRoot == "" {
			if err := os.WriteFile(path, artifact.Bytes, 0o600); err != nil {
				return fmt.Errorf("stage level %q: %w", artifact.Name, err)
			}
		} else if err := copyFile(filepath.Join(sourceRoot, "content", name), path, 0o600); err != nil {
			return fmt.Errorf("stage level %q: %w", artifact.Name, err)
		}
	}

	destination := filepath.Join(root, "content")

	backup := filepath.Join(root, ".content-backup")
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("prepare level backup: %w", err)
	}

	hadPrevious := false

	if _, err := os.Stat(destination); err == nil {
		if err := os.Rename(destination, backup); err != nil {
			return fmt.Errorf("backup level directory: %w", err)
		}

		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect level directory: %w", err)
	}

	if err := os.Rename(staging, destination); err != nil {
		if hadPrevious {
			_ = os.Rename(backup, destination)
		}

		return fmt.Errorf("publish level directory: %w", err)
	}

	if hadPrevious {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove replaced level directory: %w", err)
		}
	}

	return nil
}

func writeAssetReport(buildDirectory string, report assetpipeline.Report) error {
	contents, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode asset report: %w", err)
	}

	if err := os.WriteFile(filepath.Join(buildDirectory, "asset-report.json"), append(contents, '\n'), 0o600); err != nil {
		return fmt.Errorf("write asset report: %w", err)
	}

	return nil
}

func stageWebShell(targetDirectory, wasmExec string, airProxy bool) error {
	if err := copyFile(wasmExec, filepath.Join(targetDirectory, "wasm_exec.js"), 0o600); err != nil {
		return fmt.Errorf("stage wasm_exec.js: %w", err)
	}

	clientHash, err := fileHash(filepath.Join(targetDirectory, "game.kart"))
	if err != nil {
		return fmt.Errorf("hash staged client: %w", err)
	}

	hostHash, err := fileHash(filepath.Join(targetDirectory, "karty-host.wasm"))
	if err != nil {
		return fmt.Errorf("hash staged host: %w", err)
	}

	launcher := strings.NewReplacer("@@CLIENT_HASH@@", clientHash, "@@HOST_HASH@@", hostHash).Replace(webLauncherTemplate)
	if err := os.WriteFile(filepath.Join(targetDirectory, "karty.js"), []byte(launcher), 0o600); err != nil {
		return fmt.Errorf("write web launcher: %w", err)
	}

	launcherHash, err := fileHash(filepath.Join(targetDirectory, "karty.js"))
	if err != nil {
		return fmt.Errorf("hash staged launcher: %w", err)
	}

	runtimeHash, err := fileHash(filepath.Join(targetDirectory, "wasm_exec.js"))
	if err != nil {
		return fmt.Errorf("hash staged runtime: %w", err)
	}

	airProxyBootstrap := ""
	if airProxy {
		airProxyBootstrap = "<script>\n" +
			"    // Air's SharedWorker also opens a root-relative SSE URL internally,\n" +
			"    // so use its page-level EventSource fallback and preserve proxy prefixes.\n" +
			"    const kartyAirURL = value => typeof value === \"string\" && value.startsWith(\"/__air_internal/\")\n" +
			"      ? new URL(\".\" + value, window.location.href).href : value;\n" +
			"    try {\n" +
			"      Object.defineProperty(window, \"SharedWorker\", { configurable: true, value: undefined });\n" +
			"    } catch (_) {\n" +
			"      window.SharedWorker = undefined;\n" +
			"    }\n" +
			"    if (typeof window.EventSource === \"function\") {\n" +
			"      window.EventSource = new Proxy(window.EventSource, {\n" +
			"        construct(target, args) {\n" +
			"          args[0] = kartyAirURL(args[0]);\n" +
			"          return Reflect.construct(target, args);\n" +
			"        },\n" +
			"      });\n" +
			"    }\n" +
			"    window.KARTY_AIR_BASE_PATH_PATCHED = true;\n" +
			"  </script>"
	}

	index := strings.NewReplacer(
		"@@LAUNCHER_HASH@@", launcherHash,
		"@@RUNTIME_HASH@@", runtimeHash,
		"@@AIR_PROXY_BOOTSTRAP@@", airProxyBootstrap,
	).Replace(webIndexTemplate)

	if err := os.WriteFile(filepath.Join(targetDirectory, "index.html"), []byte(index), 0o600); err != nil {
		return fmt.Errorf("write web shell: %w", err)
	}

	return nil
}

func fileHash(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	digest := sha256.Sum256(contents)

	return hex.EncodeToString(digest[:8]), nil
}

func copyFile(source, destination string, mode os.FileMode) error {
	//nolint:gosec // source is a validated build-owned path or pinned tool artifact.
	contents, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	return os.WriteFile(destination, contents, mode) //nolint:gosec // destinations are fixed build artifacts.
}
