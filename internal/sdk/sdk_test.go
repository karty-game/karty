package sdk_test

import (
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if manifest.Version != release.SDKVersion() || manifest.Host.Version != release.SDKVersion() {
		t.Fatalf("current SDK resolved unexpected versions: %+v", manifest)
	}

	if manifest.Templates.Game != "0.0.1" {
		t.Errorf("game template = %q, want %q", manifest.Templates.Game, "0.0.1")
	}

	for _, capability := range []asset.Capability{
		asset.CapabilityWorldMaterialAtlasV1, asset.CapabilityWorldMaterialMappingV1,
		asset.CapabilityWorldLightingV1, asset.CapabilityWorldStaticSolidsV1,
		asset.CapabilityWorldLightmapsV1, asset.CapabilityWorldLightmapsPrebakedV1,
	} {
		if !slices.Contains(manifest.Assets.Capabilities.Runtime, capability) {
			t.Errorf("released default SDK omits %s", capability)
		}
	}

	for _, profile := range []string{"sprite", "interface", "environment"} {
		if processor := manifest.Assets.TextureProfiles[profile].Processor; processor != "qoi@1" {
			t.Errorf("texture profile %q processor = %q, want qoi@1", profile, processor)
		}
	}
}

func TestResolveRejectsUnknownVersion(t *testing.T) {
	t.Parallel()

	if _, err := sdk.Resolve("9.9.9"); err == nil {
		t.Fatal("Resolve() error = nil, want an unknown SDK error")
	}
}
