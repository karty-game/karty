package levelbuild

import (
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func TestGPUFixedEnvironmentLightingWithoutPointLights(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	document.Environment = &world.Environment{Version: 1, Preset: world.EnvironmentDaylight}

	for _, pointLights := range []bool{false, true} {
		t.Run(map[bool]string{false: "environment-only", true: "environment-and-point-lights"}[pointLights], func(t *testing.T) {
			t.Parallel()

			selected := document
			if !pointLights {
				selected.Lighting = nil
			}

			definition := manifest{
				Chunks: &chunkBuildSettings{Enabled: true},
				Lightmap: &lightmapBuildSettings{Enabled: true, Fixed: true, Offline: true,
					Density: new(float64(1)), PageSize: new(int(512))},
			}

			options, _, err := gpuLightingOptions(definition, &selected, nil)
			if err != nil {
				t.Fatal(err)
			}

			if options.AllStatic != pointLights {
				t.Fatal("fixed GPU lighting lost automatic authored-light selection", options.AllStatic)
			}

			loading, err := gpuLoadingChunkOptions(definition, &selected)
			if err != nil {
				t.Fatal(err)
			}

			if loading.Lightmap.AllStatic != pointLights {
				t.Fatal("spatial loading-unit grouping lost fixed lighting selection")
			}

			if _, err := worldlightmap.CompileReceivers(selected, loading.Lightmap, []int{0}, nil); err != nil {
				t.Fatal("fixed spatial loading-unit planning failed", err)
			}

			layout, err := worldlightmap.CompileReceivers(selected, options, []int{0}, nil)
			if err != nil {
				t.Fatal("fixed environment receiver planning failed", err)
			}

			if len(layout.Pages) != 1 {
				t.Fatal("missing GPU receiver page")
			}

			if pointLights && (layout.RuntimeBake == nil || len(layout.RuntimeBake.LightIDs) != 2) {
				t.Fatal("authored point lights were omitted from fixed bake", layout.RuntimeBake)
			}
		})
	}
}
