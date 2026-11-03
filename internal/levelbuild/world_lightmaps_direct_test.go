package levelbuild

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

const directLightmapYAML = `lighting:
  version: 1
  ambient: {x: 0.1, y: 0.1, z: 0.1}
  lights:
    - {id: red, position: {x: 0.5, y: 1, z: 2}, color: {x: 1, y: 0, z: 0}, radius: 6}
    - {id: blue, position: {x: 1.5, y: 1, z: 2}, color: {x: 0, y: 0, z: 1}, radius: 6}
`

func directLightmapSDK(t *testing.T) sdk.Manifest {
	t.Helper()

	selected, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	selected = currentWorldSDK(selected)

	return selected
}

func TestDirectLightmapInvalidAuthoring(t *testing.T) {
	t.Parallel()
	selected := directLightmapSDK(t)

	for name, settings := range map[string]string{
		"empty list":             `lights = []`,
		"empty single":           `light = ""`,
		"both":                   "light = \"red\"\nlights = [\"blue\"]",
		"empty single plus list": "light = \"\"\nlights = [\"blue\"]",
		"duplicate":              `lights = ["red", "blue", "red"]`,
		"last unknown":           `lights = ["red", "missing"]`,
		"last empty":             `lights = ["red", ""]`,
		"unknown extra IDs":      `lights = ["red", "blue", "a", "b", "c", "d", "e", "f", "g"]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
			enableFixtureLightmap(t, root, "enabled = true\n"+settings+"\n")

			if artifacts, err := buildAllWithFixtureMaterials(t, root, 4, "", selected); err == nil || artifacts != nil {
				t.Fatal("malformed complete direct selection produced artifacts")
			}
		})
	}

	t.Run("last emitter outside", func(t *testing.T) {
		root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+
			strings.Replace(directLightmapYAML, "x: 1.5, y: 1, z: 2", "x: 1.5, y: 1, z: 20", 1))
		enableFixtureLightmap(t, root, "enabled = true\nlights = [\"red\", \"blue\"]\n")

		if artifacts, err := buildAllWithFixtureMaterials(t, root, 4, "", selected); err == nil || artifacts != nil {
			t.Fatal("outside final emitter produced artifacts")
		}
	})
}
