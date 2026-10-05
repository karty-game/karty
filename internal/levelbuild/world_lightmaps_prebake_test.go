package levelbuild

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty/internal/sdk"
)

func prebakeLevelFixture(t *testing.T) (string, sdk.Manifest) {
	t.Helper()
	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldLightmapsPrebakedV1)

	enableFixtureLightmap(t, root, "enabled = true\nlights = [\"blue\",\"red\"]\npage_size = 512\nshadow_size = 128\n")

	artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}

	envelope := executeLightmapEnvelope(t, artifacts[0].Bytes)
	worldData, _, _ := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(worldData)
	if err != nil {
		t.Fatal(err)
	}

	layoutData, _, _ := envelope.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)

	layout, err := worldlightmap.Decode(layoutData, &document)
	if err != nil {
		t.Fatal(err)
	}

	pixels := image.NewNRGBA(image.Rect(0, 0, 1536, 512))
	for index := 0; index < len(pixels.Pix); index += 4 {
		pixels.Pix[index], pixels.Pix[index+3] = 200, 1
	}

	encoded, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil {
		t.Fatal(err)
	}

	pair, err := worldlightmap.NewPrebake(layout, &document, encoded)
	if err != nil {
		t.Fatal(err)
	}

	manifest, err := worldlightmap.EncodePrebake(pair, layout, &document)
	if err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(root, "levels", "lighting")
	writePrebakeFixture(t, filepath.Join(directory, "prebake.json"), manifest)
	writePrebakeFixture(t, filepath.Join(directory, "prebake.qoi"), encoded)

	return root, selected
}

func writePrebakeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	// #nosec G703 -- all callers construct paths inside isolated t.TempDir fixtures.
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func addPrebakeControls(t *testing.T, root, controls string) {
	t.Helper()

	path := filepath.Join(root, "levels", "lighting", "level.toml")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(t, path, append(data, []byte(controls)...))
}

func TestPrebakePackagedWASMAndDeterminism(t *testing.T) {
	t.Parallel()
	root, selected := prebakeLevelFixture(t)
	addPrebakeControls(t, root, "prebake = \"prebake.json\"\nprebake_image = \"prebake.qoi\"\n")

	first, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}

	second, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || !bytes.Equal(first[0].Bytes, second[0].Bytes) {
		t.Fatalf("nondeterministic prebake packaging: %v", err)
	}

	if !slices.Contains(first[0].Features, worldlightmap.PrebakeFeature) {
		t.Fatal("missing prebake feature")
	}

	envelope := executeLightmapEnvelope(t, first[0].Bytes)
	worldData, _, _ := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(worldData)
	if err != nil {
		t.Fatal(err)
	}

	layoutData, _, _ := envelope.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)

	layout, err := worldlightmap.Decode(layoutData, &document)
	if err != nil {
		t.Fatal(err)
	}

	manifest, _, found := envelope.Read(worldlightmap.PrebakeEntryName, 0, worldlightmap.MaxPrebakeManifestSize)
	if !found {
		t.Fatal("missing prebake manifest")
	}

	encoded, _, found := envelope.Read(worldlightmap.PrebakeImageEntryName, 0, worldlightmap.MaxPrebakeImageSize)
	if !found {
		t.Fatal("missing prebake image")
	}

	pair, err := worldlightmap.DecodePrebake(manifest, encoded, layout, &document)
	if err != nil || pair.Manifest.Width != 1536 || pair.Manifest.RGBMRange != 6 {
		t.Fatalf("packaged prebake: %v", err)
	}

	if !bytes.Contains(envelope.Metadata, []byte(`"kartyWorldLightmapPrebake":"karty.world-lightmap-prebake@1"`)) {
		t.Fatal("missing prebake marker")
	}
}

func TestPrebakeRejectsMalformedAndUnconfinedSources(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"manifest only", "image only", "empty path", "missing capability", "noncanonical", "stale lighting", "invalid image", "escaping path", "escaping symlink"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, selected := prebakeLevelFixture(t)
			directory := filepath.Join(root, "levels", "lighting")
			controls := "prebake = \"prebake.json\"\nprebake_image = \"prebake.qoi\"\n"

			switch name {
			case "manifest only":
				controls = "prebake = \"prebake.json\"\n"
			case "image only":
				controls = "prebake_image = \"prebake.qoi\"\n"
			case "empty path":
				controls = "prebake = \"\"\nprebake_image = \"prebake.qoi\"\n"
			case "missing capability":
				selected.Assets.Capabilities.Runtime = slices.DeleteFunc(
					selected.Assets.Capabilities.Runtime,
					func(c asset.Capability) bool { return c == asset.CapabilityWorldLightmapsPrebakedV1 },
				)
			case "noncanonical":
				data, err := os.ReadFile(filepath.Join(directory, "prebake.json"))
				if err != nil {
					t.Fatal(err)
				}

				writePrebakeFixture(t, filepath.Join(directory, "prebake.json"), append(data, ' '))
			case "stale lighting":
				data, err := os.ReadFile(filepath.Join(directory, "world.yaml"))
				if err != nil {
					t.Fatal(err)
				}

				writePrebakeFixture(
					t,
					filepath.Join(directory, "world.yaml"),
					[]byte(strings.Replace(string(data), "color: {x: 1, y: 0, z: 0}", "color: {x: 0.8, y: 0, z: 0}", 1)),
				)
			case "invalid image":
				writePrebakeFixture(t, filepath.Join(directory, "prebake.qoi"), []byte("broken"))
			case "escaping path":
				writePrebakeFixture(t, filepath.Join(root, "outside.json"), []byte("{}"))

				controls = "prebake = \"../../outside.json\"\nprebake_image = \"prebake.qoi\"\n"
			case "escaping symlink":
				outside := filepath.Join(t.TempDir(), "image.qoi")
				writePrebakeFixture(t, outside, []byte("broken"))

				if err := os.Symlink(outside, filepath.Join(directory, "escape.qoi")); err != nil {
					t.Fatal(err)
				}

				controls = "prebake = \"prebake.json\"\nprebake_image = \"escape.qoi\"\n"
			}

			addPrebakeControls(t, root, controls)

			if artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected); err == nil || artifacts != nil {
				t.Fatal("invalid prebake published level artifacts")
			}
		})
	}
}
