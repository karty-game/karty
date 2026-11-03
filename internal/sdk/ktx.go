package sdk

import (
	"regexp"
	"strings"
)

// ValidateKTX accepts absent pins for historical SDKs or a complete managed
// KTX tool release. Candidate builds may explicitly select a local executable;
// unpublished archives are never represented as available releases.
func ValidateKTX(manifest Manifest) error {
	version, revision, artifacts := manifest.Tools.KTX, manifest.Tools.KTXRevision, manifest.Artifacts.KTX
	if version == "" && revision == "" && len(artifacts) == 0 {
		return nil
	}

	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) ||
		!materialLowerHex(revision, 40) || len(artifacts) != 4 {
		return errIncompleteSDK
	}

	for _, platform := range []struct{ name, suffix, format string }{
		{"darwin-arm64", "Darwin-arm64.pkg", "pkg"},
		{"linux-amd64", "Linux-x86_64.tar.bz2", "tar.bz2"},
		{"linux-arm64", "Linux-arm64.tar.bz2", "tar.bz2"},
		{"windows-amd64", "Windows-x64.exe", "nsis"},
	} {
		entry := artifacts[platform.name]

		executable := "bin/ktx"
		if strings.HasPrefix(platform.name, "windows-") {
			executable += ".exe"
		}

		url := "https://github.com/KhronosGroup/KTX-Software/releases/download/v" + version +
			"/KTX-Software-" + version + "-" + platform.suffix
		if entry.URL != url || !materialLowerHex(entry.SHA256, 64) || entry.Format != platform.format || entry.Executable != executable {
			return errIncompleteSDK
		}
	}

	return nil
}
