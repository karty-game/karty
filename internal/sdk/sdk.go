// Package sdk resolves installed versioned Karty SDK bundles.
package sdk

import (
	"fmt"
	"io/fs"
	"slices"

	assetcontract "github.com/karty-game/karty-sdk/format/asset"
)

type staticError string

func (err staticError) Error() string {
	return string(err)
}

const errIncompleteSDK staticError = "SDK is incomplete"

// Manifest is the subset of an SDK manifest required by the initial CLI.
type Manifest struct {
	resources     fs.FS
	Compatibility Compatibility `toml:"-"`
	Bundle        ToolArtifact  `toml:"bundle"`
	Version       string        `toml:"version"`
	API           struct {
		Version string `toml:"version"`
	} `toml:"api"`
	Host struct {
		Version string `toml:"version"`
	} `toml:"host"`
	Docs struct {
		Version string `toml:"version"`
	} `toml:"docs"`
	Templates struct {
		Game string `toml:"game"`
	} `toml:"templates"`
	Assets struct {
		Capabilities    assetcontract.Capabilities `toml:"capabilities"`
		TextureProfiles map[string]AssetProfile    `toml:"texture-profiles"`
		SoundProfiles   map[string]SoundProfile    `toml:"sound-profiles"`
	} `toml:"assets"`
	Tools struct {
		Air                 string `toml:"air"`
		Go                  string `toml:"go"`
		Materialize         string `toml:"materialize"`
		MaterializeRevision string `toml:"materialize-revision"`
		TinyGo              string `toml:"tinygo"`
		WasmTools           string `toml:"wasm-tools"`
	} `toml:"tools"`
	Artifacts struct {
		Air         map[string]ToolArtifact         `toml:"air"`
		Materialize map[string]MaterialToolArtifact `toml:"materialize"`
		TinyGo      map[string]ToolArtifact         `toml:"tinygo"`
		WasmTools   map[string]ToolArtifact         `toml:"wasm-tools"`
		Host        struct {
			Native       map[string]ToolArtifact `toml:"native"`
			Web          ToolArtifact            `toml:"web"`
			WebRuntime   ToolArtifact            `toml:"web-runtime"`
			WebOptimized ToolArtifact            `toml:"web-optimized"`
			WebBrotli    ToolArtifact            `toml:"web-brotli"`
		} `toml:"host"`
	} `toml:"artifacts"`
}

// Compatibility describes supported formats declared by the SDK's bundle.json.
// These are format revisions, independent of SDK and API release numbers.
//
//nolint:tagliatelle // The SDK archive defines snake_case compatibility keys.
type Compatibility struct {
	Format         int    `json:"format"`
	UISchema       uint32 `json:"ui_schema"`
	ProjectCodegen int    `json:"project_codegen"`
}

// SupportsAuthoredActions reports whether the bundle supplies the action generator contract.
func (manifest Manifest) SupportsAuthoredActions() bool {
	return manifest.Compatibility.ProjectCodegen == authoredActionProjectCodegen
}

// AssetProfile pins the processor selected for an SDK asset profile.
type AssetProfile struct {
	Processor assetcontract.Processor   `toml:"processor"`
	Transform assetcontract.ImageRecipe `toml:"transform"`
}

// SoundProfile pins one deterministic WAV-to-QOA processing recipe.
type SoundProfile struct {
	Processor assetcontract.Processor   `toml:"processor"`
	Transform assetcontract.AudioRecipe `toml:"transform"`
}

// ToolArtifact identifies a verified tool release archive.
type ToolArtifact struct {
	URL    string `toml:"url"`
	SHA256 string `toml:"sha256"`
}

func validateManifest(version string, manifest Manifest) error {
	if err := validateVersion(version); err != nil {
		return err
	}

	if manifest.Version != version || manifest.API.Version == "" || manifest.Host.Version == "" ||
		manifest.Templates.Game == "" ||
		len(manifest.Assets.TextureProfiles) == 0 || manifest.Assets.TextureProfiles["sprite"].Processor == "" ||
		manifest.Tools.Air == "" || manifest.Tools.Go == "" || manifest.Tools.TinyGo == "" ||
		manifest.Tools.WasmTools == "" {
		return fmt.Errorf("SDK %s: %w", version, errIncompleteSDK)
	}

	if err := validateAssets(manifest); err != nil {
		return fmt.Errorf("SDK %s assets: %w", version, err)
	}

	if err := ValidateMaterialize(manifest); err != nil {
		return fmt.Errorf("SDK %s Materialize: %w", version, err)
	}

	if slices.Contains(manifest.Assets.Capabilities.Runtime, assetcontract.CapabilityWorldMaterialAtlasV1) &&
		manifest.Tools.Materialize == "" {
		return fmt.Errorf("SDK %s world material atlas requires Materialize: %w", version, errIncompleteSDK)
	}

	return nil
}

func validateAssets(manifest Manifest) error {
	capabilities := manifest.Assets.Capabilities
	if err := capabilities.Validate(); err != nil {
		return err
	}

	for _, profile := range manifest.Assets.TextureProfiles {
		switch profile.Processor {
		case assetcontract.ProcessorQOIv1:
			if profile.Transform.Validate() != nil ||
				!slices.Contains(capabilities.Processors, profile.Processor) ||
				!slices.Contains(capabilities.Runtime, assetcontract.CapabilityTextureQOIv1) {
				return errIncompleteSDK
			}
		case assetcontract.ProcessorQOAv1, assetcontract.ProcessorCopyPNGv1:
			return errIncompleteSDK
		default:
			return errIncompleteSDK
		}
	}

	for _, profile := range manifest.Assets.SoundProfiles {
		if profile.Processor != assetcontract.ProcessorQOAv1 || profile.Transform.Validate() != nil ||
			!slices.Contains(capabilities.Processors, profile.Processor) ||
			!slices.Contains(capabilities.Runtime, assetcontract.CapabilitySoundQOAv1) {
			return errIncompleteSDK
		}
	}

	return nil
}
