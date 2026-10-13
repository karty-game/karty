package levelbuild

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestAlphaParticlesRequireExactSDKCapability(t *testing.T) {
	t.Parallel()

	for _, blend := range []string{"alpha", "premultiplied"} {
		t.Run(blend, func(t *testing.T) {
			t.Parallel()
			testParticlesRequireExactSDKCapability(t, blend)
		})
	}
}

func testParticlesRequireExactSDKCapability(t *testing.T, blend string) {
	t.Helper()

	selected, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	selected = withoutRuntimeCapabilities(geometrySDK(selected), asset.CapabilityWorldParticlesV1, asset.CapabilityWorldParticlesV2)
	input := "version: 11\n" + lightingRoomYAML + strings.Replace(
		strings.Replace(levelParticleYAML, "  version: 1", "  version: 2", 1),
		"blend: additive",
		"blend: "+blend,
		1,
	)
	root := lightingLevelFixture(t, input)

	for _, runtime := range [][]asset.Capability{nil, {asset.CapabilityWorldParticlesV1}} {
		candidate := selected

		candidate.Assets.Capabilities.Runtime = append(slices.Clone(selected.Assets.Capabilities.Runtime), runtime...)
		if artifacts, err := BuildAllWithAssets(
			context.Background(),
			root,
			4,
			"",
			candidate,
		); !errors.Is(err, ErrManifest) ||
			artifacts != nil {
			t.Fatalf("v1-only SDK accepted alpha2: %v", err)
		}
	}

	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldParticlesV2)

	artifacts, err := BuildAllWithAssets(context.Background(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}

	if len(artifacts) != 1 || !slices.Contains(artifacts[0].Features, worldparticles.FeatureV2) ||
		slices.Contains(artifacts[0].Features, worldparticles.Feature) {
		t.Fatal("payload capability declaration is not exact")
	}

	envelope, err := level.DecodePassiveModule(artifacts[0].Bytes)
	if err != nil {
		t.Fatal(err)
	}

	encoded, _, found := envelope.Read(worldparticles.EntryName, 0, worldparticles.MaxEncodedSize)
	if !found {
		t.Fatal("alpha payload absent")
	}

	particles, err := worldparticles.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if particles.Version != worldparticles.AlphaVersion || particles.Presets[0].Sprite.Blend != blend {
		t.Fatal("staged alpha contract changed")
	}
}
