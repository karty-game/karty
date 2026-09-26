// Package sdk resolves versioned Karty SDK manifests embedded in core.
package sdk

import (
	"fmt"
	"io/fs"
)

type staticError string

func (err staticError) Error() string {
	return string(err)
}

const errIncompleteSDK staticError = "SDK is incomplete"

// Manifest is the subset of an SDK manifest required by the initial CLI.
type Manifest struct {
	resources fs.FS
	Bundle    ToolArtifact `toml:"bundle"`
	Version   string       `toml:"version"`
	API       struct {
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
		TextureProfiles map[string]AssetProfile `toml:"texture-profiles"`
	} `toml:"assets"`
	Tools struct {
		Air       string `toml:"air"`
		Go        string `toml:"go"`
		TinyGo    string `toml:"tinygo"`
		WasmTools string `toml:"wasm-tools"`
	} `toml:"tools"`
	Artifacts struct {
		Air       map[string]ToolArtifact `toml:"air"`
		TinyGo    map[string]ToolArtifact `toml:"tinygo"`
		WasmTools map[string]ToolArtifact `toml:"wasm-tools"`
		Host      struct {
			Native       map[string]ToolArtifact `toml:"native"`
			Web          ToolArtifact            `toml:"web"`
			WebRuntime   ToolArtifact            `toml:"web-runtime"`
			WebOptimized ToolArtifact            `toml:"web-optimized"`
			WebBrotli    ToolArtifact            `toml:"web-brotli"`
		} `toml:"host"`
	} `toml:"artifacts"`
}

// AssetProfile pins the processor selected for an SDK asset profile.
type AssetProfile struct {
	Processor string `toml:"processor"`
}

// ToolArtifact identifies a verified tool release archive.
type ToolArtifact struct {
	URL    string `toml:"url"`
	SHA256 string `toml:"sha256"`
}

func validateManifest(version string, manifest Manifest) error {
	if manifest.Version != version || manifest.API.Version == "" || manifest.Host.Version == "" ||
		manifest.Templates.Game == "" ||
		len(manifest.Assets.TextureProfiles) == 0 || manifest.Assets.TextureProfiles["sprite"].Processor == "" ||
		manifest.Tools.Air == "" || manifest.Tools.Go == "" || manifest.Tools.TinyGo == "" ||
		manifest.Tools.WasmTools == "" {
		return fmt.Errorf("SDK %s: %w", version, errIncompleteSDK)
	}

	return nil
}
