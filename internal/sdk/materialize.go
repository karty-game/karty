package sdk

import (
	"regexp"
	"strings"
)

// MaterialToolArtifact pins a tool archive and its confined executable entry.
type MaterialToolArtifact struct {
	URL        string `toml:"url"`
	SHA256     string `toml:"sha256"`
	Format     string `toml:"format"`
	Executable string `toml:"executable"`
}

// ValidateMaterialize accepts an absent pin or a complete released tools pin.
// Keep this contract aligned with the engine's versioned SDK schema.
func ValidateMaterialize(manifest Manifest) error {
	version, revision := manifest.Tools.Materialize, manifest.Tools.MaterializeRevision
	artifacts := manifest.Artifacts.Materialize

	if version == "" && revision == "" && len(artifacts) == 0 {
		return nil
	}

	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) ||
		!materialLowerHex(revision, 40) || len(artifacts) != 4 {
		return errIncompleteSDK
	}

	for _, platform := range []string{"darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64"} {
		artifact := artifacts[platform]
		executable := "bin/materialize-cli"

		if strings.HasPrefix(platform, "windows-") {
			executable += ".exe"
		}

		url := "https://github.com/karty-game/karty-tools/releases/download/materialize-v" + version +
			"/materialize-" + version + "-" + platform + ".zip"
		if artifact.URL != url || !materialLowerHex(artifact.SHA256, 64) ||
			artifact.Format != "zip" || artifact.Executable != executable {
			return errIncompleteSDK
		}
	}

	return nil
}

func materialLowerHex(value string, size int) bool {
	return len(value) == size && strings.Trim(value, "0123456789abcdef") == ""
}
