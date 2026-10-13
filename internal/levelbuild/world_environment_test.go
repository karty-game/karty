package levelbuild

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestEnvironmentCapabilityAndPackagedWASM(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	manifest = withoutRuntimeCapabilities(
		geometrySDK(manifest),
		asset.CapabilityWorldEnvironmentV1,
		asset.CapabilityWorldEnvironmentBackdropV1,
	)
	input := "version: 11\nenvironment: {preset: fantasy, palette: violet, backdrop: mountains, time_of_day: 0, cloud_amount: 0.45}\n" + strings.Replace(
		lightingRoomYAML,
		"    floor:",
		"    sky: true\n    floor:",
		1,
	)

	root := lightingLevelFixture(t, input)
	if artifacts, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest); !errors.Is(err, ErrManifest) || artifacts != nil {
		t.Fatalf("SDK gate lost: %v %+v", err, artifacts)
	}

	manifest.Assets.Capabilities.Runtime = append(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldEnvironmentV1)
	if artifacts, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest); !errors.Is(err, ErrManifest) || artifacts != nil {
		t.Fatalf("backdrop SDK gate lost: %v %+v", err, artifacts)
	}

	manifest.Assets.Capabilities.Runtime = append(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldEnvironmentBackdropV1)

	first, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
	if err != nil {
		t.Fatal(err)
	}

	second, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
	if err != nil {
		t.Fatal(err)
	}

	if len(first) != 1 || !slices.Contains(first[0].Features, world.FeatureEnvironment) ||
		!slices.Contains(first[0].Features, world.FeatureEnvironmentBackdrop) || !bytes.Equal(first[0].Bytes, second[0].Bytes) {
		t.Fatal("environment packaging is not deterministic or lost feature")
	}

	document := executePackagedLightingWorld(t, first[0].Bytes)
	if document.Environment == nil || document.Environment.Preset != world.EnvironmentFantasy ||
		document.Environment.Palette != world.EnvironmentPaletteViolet ||
		document.Environment.Backdrop != world.EnvironmentBackdropMountains ||
		document.Environment.TimeOfDay == nil || *document.Environment.TimeOfDay != 0 ||
		document.Environment.CloudAmount == nil || *document.Environment.CloudAmount != .45 ||
		!document.Sectors[0].Sky {
		t.Fatalf("WASM environment lost: %+v", document.Environment)
	}
}
