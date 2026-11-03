package sdk

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// HostBundleFormat declares signed complete host archives instead of raw files.
const HostBundleFormat = "host-bundle@1"

// HostBundleRenderer is the renderer supported by the version 1 inventory.
const HostBundleRenderer = "webgpu"

// HostBundleArtifact pins a complete ZIP with a schema 1 host-bundle.toml.
// Format is explicit: URL suffixes never select archive decoders.
type HostBundleArtifact struct {
	URL    string `toml:"url"`
	SHA256 string `toml:"sha256"`
	Format string `toml:"format"`
}

// ValidateHostBundles checks only the explicitly declared host delivery contract.
// It does not authenticate caller-created manifests; published metadata still
// passes through the pinned signature verifier before installation.
func ValidateHostBundles(m Manifest) error {
	if err := validateVersion(m.Host.Version); err != nil {
		return err
	}

	return validateHostDelivery(m.Host.Format, m.Host.Renderer, m.Artifacts.Host.Bundles, hasRawHostArtifacts(m))
}

func validateHostDelivery(format, renderer string, bundles map[string]HostBundleArtifact, raw bool) error {
	if format == "" {
		if renderer != "" || len(bundles) != 0 {
			return fmt.Errorf("host bundle pins need explicit %s declaration: %w", HostBundleFormat, errIncompleteSDK)
		}

		return nil
	}

	if format != HostBundleFormat || renderer != HostBundleRenderer || raw {
		return fmt.Errorf("unsupported or mixed host delivery format/renderer: %w", errIncompleteSDK)
	}
	// A source/local SDK may declare delivery before release artifacts exist.
	// Managed installation must resolve matching signed pins or fail closed.
	for target, a := range bundles {
		if !hostBundleTarget(target) || !validHostBundleArtifact(a) {
			return fmt.Errorf("host bundle artifact %q is invalid: %w", target, errIncompleteSDK)
		}
	}

	return nil
}

func hostBundleTarget(target string) bool {
	switch target {
	case "web", "linux-amd64", "linux-arm64", "darwin-arm64", "windows-amd64", "windows-arm64":
		return true
	default:
		return false
	}
}

func validHostBundleArtifact(artifact HostBundleArtifact) bool {
	u, err := url.Parse(artifact.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.Path == "" || artifact.Format != "zip" {
		return false
	}

	digest, err := hex.DecodeString(artifact.SHA256)

	return err == nil && len(digest) == 32 && artifact.SHA256 == strings.ToLower(artifact.SHA256)
}

func hasRawHostArtifacts(m Manifest) bool {
	h := m.Artifacts.Host
	if len(h.Native) != 0 {
		return true
	}

	for _, a := range []ToolArtifact{h.Web, h.WebRuntime, h.WebOptimized, h.WebBrotli} {
		if a.URL != "" || a.SHA256 != "" {
			return true
		}
	}

	return false
}
