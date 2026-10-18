package levelbuild

import (
	"errors"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestBrightDaylightRequiresSkylightCapability(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	document := world.Document{Environment: &world.Environment{Version: 1, Preset: world.EnvironmentDaylightBright}}

	assets := &assetBuild{manifest: manifest}
	assets.manifest.Assets.Capabilities.Runtime = slices.DeleteFunc(
		slices.Clone(assets.manifest.Assets.Capabilities.Runtime),
		func(capability asset.Capability) bool {
			return capability == asset.CapabilityWorldEnvironmentSkylightV1
		},
	)

	if _, err := worldEnvironmentFeatures(assets, document); !errors.Is(err, ErrManifest) {
		t.Fatal("SDK without skylight capability accepted bright daylight", err)
	}

	assets.manifest.Assets.Capabilities.Runtime = append(
		slices.Clone(assets.manifest.Assets.Capabilities.Runtime),
		asset.CapabilityWorldEnvironmentSkylightV1,
	)

	features, err := worldEnvironmentFeatures(assets, document)
	if err != nil || !slices.Contains(features, world.FeatureEnvironmentSkylight) {
		t.Fatal("SDK with skylight capability rejected bright daylight", err)
	}
}
