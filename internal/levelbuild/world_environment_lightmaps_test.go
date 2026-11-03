package levelbuild

import (
	"errors"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
)

func TestCurrentGPUProfileRejectsEnvironmentVariantLighting(t *testing.T) {
	t.Parallel()

	input := "version: 11\nenvironment: {preset: daylight}\n" + strings.Replace(
		lightingRoomYAML,
		"    floor:",
		"    sky: true\n    floor:",
		1,
	)
	root := lightingLevelFixture(t, input)
	enableFixtureLightmap(
		t,
		root,
		"enabled = true\noffline = true\ndensity = 2\npage_size = 512\nbake_samples = 2\nbake_bounces = 1\n[lightmap.environment]\ntimes = [0,9,12,17]\ndefault_time = 12\n",
	)
	enableFixtureChunks(t, root, "")
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldLightmapsEnvironmentVariantsV1)

	artifacts, err := buildAllWithFixtureMaterials(t, root, 4, "", selected)
	if !errors.Is(err, ErrManifest) || len(artifacts) != 0 {
		t.Fatal("unsupported environment-variant publication accepted", err)
	}
}

func TestEnvironmentVariantAuthoringRejectsMalformedRecipes(t *testing.T) {
	t.Parallel()

	for _, settings := range []environmentLightmapSettings{{}, {Times: []float64{15, 9}}, {Times: []float64{15, 15}}, {Times: []float64{24}}, {Times: []float64{9}}} {
		if settings.validate() == nil {
			t.Fatalf("malformed conditions accepted: %+v", settings)
		}
	}

	if validateEnvironmentLightmapKeys([]byte("[lightmap.environment]\ntimes=[15]\nfuture=true\n")) == nil {
		t.Fatal("unknown environment bake option accepted")
	}

	for _, recipe := range []lightmapBuildSettings{
		{Environment: &environmentLightmapSettings{Times: []float64{15}}},
		{Enabled: true, Offline: true, Light: new("point"), Environment: &environmentLightmapSettings{Times: []float64{15}}},
		{Enabled: true, Offline: true, BakeMaxRayDistance: new(float64(4)), Environment: &environmentLightmapSettings{Times: []float64{15}}},
	} {
		if _, err := recipe.options(); err == nil {
			t.Fatal("mixed or disabled environment recipe accepted")
		}
	}
}
