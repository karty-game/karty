package bake

import (
	"errors"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty/internal/sdk"
)

func TestBakeSDKUsesAdvertisedCapability(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"0.0.8", "0.0.9", "0.0.10"} {
		manifest := sdk.Manifest{Version: version}

		manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityWorldLightmapsPrebakedV1}
		if err := validateBakeSDK(manifest); err != nil {
			t.Fatal(err)
		}
		// Basic runtime lightmaps alone do not promise prebaked payload support,
		// even if the bundle claims a version that previously passed the gate.
		manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityWorldLightmapsV1}
		if err := validateBakeSDK(manifest); !errors.Is(err, ErrSDK) {
			t.Fatalf("%s: unsupported SDK accepted: %v", version, err)
		}
	}
}
