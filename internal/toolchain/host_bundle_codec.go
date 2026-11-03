package toolchain

import (
	"fmt"
	"os"

	"github.com/karty-game/karty/internal/sdk"
)

// WebGPUTextureCodecFiles is the complete optional browser codec module family.
func WebGPUTextureCodecFiles() []string {
	return []string{"texture-codec.mjs", "texture-worker.mjs", "transcoder.wasm"}
}

// WebGPUTextureCodecLicenses accompany the bundled Basis and Zstandard code.
func WebGPUTextureCodecLicenses() []string {
	return []string{"licenses/Basis-LICENSE.txt", "licenses/Zstd-LICENSE.txt"}
}

func (bundle *HostBundle) validateWebTextureCodec() error {
	present := 0

	for _, name := range WebGPUTextureCodecFiles() {
		if _, ok := bundle.Files[name]; ok {
			present++
		}
	}

	if present != len(WebGPUTextureCodecFiles()) {
		return fmt.Errorf("web host bundle requires the complete texture codec family: %w", os.ErrInvalid)
	}

	for _, name := range WebGPUTextureCodecLicenses() {
		if _, ok := bundle.Files[name]; !ok {
			return fmt.Errorf("texture codec host bundle is missing %s: %w", name, os.ErrInvalid)
		}
	}

	return nil
}

// ValidateWebHostSDK requires the complete verified browser texture codec family.
func ValidateWebHostSDK(manifest sdk.Manifest, bundle *HostBundle) error {
	if bundle == nil || bundle.Target != "web" {
		return fmt.Errorf("SDK %s requires a verified web texture codec host bundle: %w", manifest.Version, os.ErrInvalid)
	}

	return bundle.validateWebTextureCodec()
}
