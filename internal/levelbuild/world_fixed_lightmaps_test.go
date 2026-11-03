package levelbuild

import (
	"errors"
	"testing"
)

const fixedBakeSettings = "enabled = true\noffline = true\nfixed = true\ndensity = 1\npage_size = 512\nbake_samples = 1\nbake_bounces = 0\nbake_denoise = 'off'\n"

func TestFixedLightingRejectsLegacyOrPartialRecipes(t *testing.T) {
	t.Parallel()

	for name, settings := range map[string]lightmapBuildSettings{
		"disabled":  {Fixed: true, Offline: true},
		"live":      {Fixed: true, Enabled: true},
		"variants":  {Fixed: true, Enabled: true, Offline: true, Environment: &environmentLightmapSettings{Times: []float64{15}}},
		"selection": {Fixed: true, Enabled: true, Offline: true, Lights: []string{"lamp"}},
		"import":    {Fixed: true, Enabled: true, Offline: true, Prebake: new("bake"), PrebakeImage: new("image")},
		"cutoff":    {Fixed: true, Enabled: true, Offline: true, BakeMaxRayDistance: new(float64(4))},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := settings.options(); !errors.Is(err, ErrManifest) {
				t.Fatal("invalid fixed recipe accepted", err)
			}
		})
	}
}
