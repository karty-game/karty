package sdk

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/pelletier/go-toml/v2"
)

func TestHostBundleDeliveryDeclarationRejectsAmbiguity(t *testing.T) {
	t.Parallel()

	pin := HostBundleArtifact{URL: "https://example.invalid/host.zip", SHA256: strings.Repeat("a", 64), Format: "zip"}
	if err := validateHostDelivery(HostBundleFormat, HostBundleRenderer, map[string]HostBundleArtifact{"web": pin}, false); err != nil {
		t.Fatal(err)
	}

	if err := validateHostDelivery(HostBundleFormat, HostBundleRenderer, nil, false); err != nil {
		t.Fatal("source declaration before release pins", err)
	}

	if err := validateHostDelivery("", "", nil, true); err != nil {
		t.Fatal("legacy raw declaration", err)
	}

	for _, testCase := range []struct {
		format, renderer string
		target           string
		pin              HostBundleArtifact
		raw              bool
	}{
		{"", HostBundleRenderer, "web", pin, false},
		{HostBundleFormat, "", "web", pin, false},
		{"host-bundle@2", HostBundleRenderer, "web", pin, false},
		{HostBundleFormat, HostBundleRenderer, "web", pin, true},
		{HostBundleFormat, HostBundleRenderer, "../outside", pin, false},
		{HostBundleFormat, HostBundleRenderer, "web", HostBundleArtifact{URL: pin.URL, SHA256: pin.SHA256, Format: "tar.gz"}, false},
	} {
		if validateHostDelivery(
			testCase.format,
			testCase.renderer,
			map[string]HostBundleArtifact{testCase.target: testCase.pin},
			testCase.raw,
		) == nil {
			t.Fatal("invalid delivery accepted", testCase)
		}
	}
}

func TestSignedHostBundlePinsAreAuthenticatedAndPreserved(t *testing.T) {
	t.Parallel()

	manifest := Manifest{Version: "0.0.21"}
	manifest.API.Version, manifest.Host.Version, manifest.Docs.Version, manifest.Templates.Game = "0.0.15", "0.0.21", "0.0.18", "0.0.11"
	manifest.Host.Format, manifest.Host.Renderer = HostBundleFormat, HostBundleRenderer
	manifest.Tools.Go, manifest.Tools.TinyGo, manifest.Tools.WasmTools = "1.27.3", "0.44.0", "1.262.0"
	manifest.Assets.Capabilities.Processors = []asset.Processor{asset.ProcessorQOIv1}
	manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityTextureQOIv1}
	manifest.Assets.TextureProfiles = map[string]AssetProfile{
		"sprite": {
			Processor: asset.ProcessorQOIv1,
			Transform: asset.ImageRecipe{MaxWidth: 256, MaxHeight: 256, Filter: asset.ImageFilterNearest, BitDepth: 8},
		},
	}
	manifest.Artifacts.Host.Bundles = map[string]HostBundleArtifact{
		"web": {URL: "https://example.invalid/host.zip", SHA256: strings.Repeat("b", 64), Format: "zip"},
	}

	encoded, err := toml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	signature := ed25519.Sign(private, encoded)

	got, err := parsePublishedManifest(manifest.Version, encoded, signature, public)
	if err != nil || got.Host.Format != HostBundleFormat || got.Artifacts.Host.Bundles["web"] != manifest.Artifacts.Host.Bundles["web"] {
		t.Fatal("signed bundle declaration lost", err)
	}

	encoded[len(encoded)-2] ^= 1
	if _, err := parsePublishedManifest(manifest.Version, encoded, signature, public); !errors.Is(err, errPublishedSDKSignature) {
		t.Fatal("tampered bundle declaration accepted", err)
	}
}
