package levelbuild

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func TestOfflineBakeAutomaticPackageAndStaleFallback(t *testing.T) {
	t.Parallel()
	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(
		t,
		root,
		"enabled = true\noffline = true\nlights = [\"red\",\"blue\"]\npage_size = 512\ndensity = 1\nshadow_size = 32\n",
	)
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldLightmapsPrebakedV1)

	build := func(wantBake bool) {
		t.Helper()

		artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
		if err != nil {
			t.Fatal(err)
		}

		if slices.Contains(artifacts[0].Features, worldlightmap.PrebakeFeature) != wantBake {
			t.Fatalf("packaged prebake=%v want=%v", !wantBake, wantBake)
		}

		envelope := executeLightmapEnvelope(t, artifacts[0].Bytes)

		_, _, hasBake := envelope.Read(worldlightmap.PrebakeEntryName, 0, worldlightmap.MaxPrebakeManifestSize)
		if hasBake != wantBake {
			t.Fatalf("prebake entry=%v want=%v", hasBake, wantBake)
		}

		if _, _, found := envelope.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize); !found {
			t.Fatal("fallback lost runtime recipe")
		}
	}
	build(false)

	samples, bounces := 4, 1

	reports, err := BakeAll(t.Context(), root, BakeOptions{Samples: &samples, Bounces: &bounces, Workers: 1}, nil)
	if err != nil || len(reports) != 1 {
		t.Fatalf("bake reports=%v: %v", reports, err)
	}

	build(true)

	first, err := os.ReadFile(reports[0].Manifest)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := BakeAll(t.Context(), root, BakeOptions{Samples: &samples, Bounces: &bounces, Workers: 2}, nil); err != nil {
		t.Fatal(err)
	}

	second, err := os.ReadFile(reports[0].Manifest)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("worker count changed deterministic bake: %v", err)
	}

	directory := filepath.Join(root, "levels", "lighting")
	worldPath := filepath.Join(directory, "world.yaml")

	worldBytes, err := os.ReadFile(worldPath)
	if err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(
		t,
		worldPath,
		[]byte(strings.Replace(string(worldBytes), "color: {x: 1, y: 0, z: 0}", "color: {x: 0.8, y: 0, z: 0}", 1)),
	)
	build(false)
	writePrebakeFixture(t, worldPath, worldBytes)
	build(true)

	texturePath := filepath.Join(directory, "wall.png")

	originalTexture, err := os.ReadFile(texturePath)
	if err != nil {
		t.Fatal(err)
	}

	changed := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	changed.SetNRGBA(0, 0, color.NRGBA{B: 180, A: 255})

	var texture bytes.Buffer
	if err := png.Encode(&texture, changed); err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(t, texturePath, texture.Bytes())
	build(false)

	manifestPath := filepath.Join(directory, "level.toml")

	definitionBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	imageRelative, err := filepath.Rel(directory, reports[0].Image)
	if err != nil {
		t.Fatal(err)
	}

	manualControls := "prebake = \"" + automaticBakeManifest + "\"\nprebake_image = \"" + filepath.ToSlash(imageRelative) + "\""
	writePrebakeFixture(t, manifestPath, []byte(strings.Replace(string(definitionBytes), "offline = true", manualControls, 1)))

	if _, err := BuildAllWithAssets(t.Context(), root, 4, "", selected); err == nil {
		t.Fatal("explicit offline import accepted stale original albedo")
	}

	writePrebakeFixture(t, manifestPath, definitionBytes)
	writePrebakeFixture(t, texturePath, originalTexture)
	build(true)
	addPrebakeControls(t, root, "bake_samples = 8\n")
	build(false)

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(t, manifestPath, []byte(strings.Replace(string(manifestData), "bake_samples = 8\n", "", 1)))
	writePrebakeFixture(t, reports[0].Image, []byte("corrupt generated cache"))
	build(false)
}

func TestOfflineBakeRejectsCacheSymlinkAndCancelledWrite(t *testing.T) {
	t.Parallel()
	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(t, root, "enabled = true\noffline = true\nlights = [\"red\"]\npage_size = 512\ndensity = 1\n")
	directory := filepath.Join(root, "levels", "lighting")

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(directory, ".karty")); err != nil {
		t.Fatal(err)
	}

	if _, err := prepareBakeDirectory(directory); err == nil {
		t.Fatal("bake followed escaping symlink")
	}

	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatal("bake wrote outside level")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := BakeAll(ctx, root, BakeOptions{}, nil); err == nil {
		t.Fatal("canceled bake succeeded")
	}
}
