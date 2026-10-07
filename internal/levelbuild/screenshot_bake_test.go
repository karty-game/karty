package levelbuild

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func TestScreenshotQuickBakeRepairAndReuse(t *testing.T) {
	t.Parallel()
	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(
		t,
		root,
		"enabled = true\nlights = [\"red\",\"blue\"]\npage_size = 512\ndensity = 1\nshadow_size = 32\nbake_samples = 16\nbake_bounces = 2\n",
	)
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldLightmapsPrebakedV1)
	manifestPath := filepath.Join(root, "levels", "lighting", "level.toml")

	original, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	build := func() {
		t.Helper()

		artifact, err := BuildNamedForScreenshot(t.Context(), root, "lighting", 4, "", selected)
		if err != nil || !slices.Contains(artifact.Features, worldlightmap.PrebakeFeature) {
			t.Fatal("screenshot did not package a bake", err)
		}

		data, err := os.ReadFile(manifestPath)
		if err != nil || !bytes.Equal(data, original) {
			t.Fatal("screenshot changed authored settings", err)
		}
	}
	cache := filepath.Join(root, ".karty", "bakes", "lighting", automaticBakeManifest)
	read := func() worldlightmap.PrebakeManifest {
		t.Helper()

		data, err := os.ReadFile(cache)
		if err != nil {
			t.Fatal(err)
		}

		var header worldlightmap.PrebakeManifest
		if err := json.Unmarshal(data, &header); err != nil {
			t.Fatal(err)
		}

		return header
	}

	build()

	if header := read(); header.Samples != 4 || header.Bounces != 1 {
		t.Fatal("quick bake used wrong settings", header)
	}

	samples, bounces := 8, 2
	if _, err := BakeAll(
		t.Context(),
		root,
		BakeOptions{Level: "lighting", Samples: &samples, Bounces: &bounces, Workers: 1},
		nil,
	); err != nil {
		t.Fatal(err)
	}

	stamp := time.Unix(123, 0)
	if err := os.Chtimes(cache, stamp, stamp); err != nil {
		t.Fatal(err)
	}

	build()

	info, err := os.Stat(cache)
	if err != nil || !info.ModTime().Equal(stamp) || read().Samples != 8 {
		t.Fatal("existing bake was needlessly regenerated", err)
	}

	header := read()

	image := filepath.Join(filepath.Dir(cache), "lightmap-"+header.ImageSHA256+".qoi")
	if err := os.WriteFile(image, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}

	build()

	if read().Samples != 4 {
		t.Fatal("corrupt bake was not repaired")
	}
	// The screenshot-only override never enables offline lighting in normal builds.
	artifact, err := BuildNamedWithAssets(t.Context(), root, "lighting", 4, "", selected)
	if err != nil || slices.Contains(artifact.Features, worldlightmap.PrebakeFeature) {
		t.Fatal("screenshot settings leaked into normal builds", err)
	}
}
