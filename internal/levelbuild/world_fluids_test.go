package levelbuild

import (
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/sdk"
)

func TestFluidSDKGate(t *testing.T) {
	t.Parallel()

	document := world.Document{Fluids: &world.Fluids{Version: 1}, Animations: &world.Animations{Version: 1}}
	build := &assetBuild{manifest: sdk.Manifest{}}

	if _, err := worldAnimationFeatures(build, document); err == nil {
		t.Fatal("fluid capability bypassed")
	}

	build.manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityWorldFluidsV1, asset.CapabilityWorldAnimationsV1}
	features, err := worldAnimationFeatures(build, document)

	if err != nil || !slices.Contains(features, world.FeatureFluids) || !slices.Contains(features, world.FeatureAnimations) {
		t.Fatal("fluid capability selection", features, err)
	}
}
