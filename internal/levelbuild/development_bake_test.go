package levelbuild

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func TestDevelopmentBakeRefreshesAndReusesAuthoredLighting(t *testing.T) {
	t.Parallel()
	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(t, root, automaticBakeSettings)

	run := func(reused bool) BakeReport {
		t.Helper()

		reports, err := BakeDevelopment(t.Context(), root, nil)
		if err != nil || len(reports) != 1 || reports[0].Reused != reused {
			t.Fatalf("development bake reports=%+v, want reused=%v: %v", reports, reused, err)
		}

		artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", directLightmapSDK(t))
		if err != nil || len(artifacts) != 1 || !slices.Contains(artifacts[0].Features, worldlightmap.PrebakeFeature) {
			t.Fatalf("development bake was not packaged: %v", err)
		}

		return reports[0]
	}
	first := run(false)

	oldManifest, err := os.ReadFile(first.Manifest)
	if err != nil {
		t.Fatal(err)
	}

	run(true) // A game-code-only edit has unchanged bake inputs.

	path := filepath.Join(root, "levels", "lighting", "world.yaml")

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	changed := strings.Replace(string(contents), "color: {x: 1, y: 0, z: 0}", "color: {x: 0.5, y: 0, z: 0}", 1)
	if changed == string(contents) {
		t.Fatal("light change did not match fixture")
	}

	writePrebakeFixture(t, path, []byte(changed))

	updated := run(false)

	newManifest, err := os.ReadFile(updated.Manifest)
	if err != nil || string(oldManifest) == string(newManifest) {
		t.Fatalf("authored light change kept stale baked pixels: %v", err)
	}

	run(true)
	writePrebakeFixture(t, updated.Image, []byte("incomplete cached pixels"))
	run(false)
}

func TestDevelopmentBakeSkipsLevelsOwnedByOtherBuildPaths(t *testing.T) {
	t.Parallel()

	for name, settings := range map[string]string{
		"none":        "",
		"disabled":    "[lightmap]\nenabled = false\noffline = true\n",
		"runtime":     "[lightmap]\nenabled = true\nlights = ['red']\n",
		"imported":    "[lightmap]\nenabled = true\nprebake = 'pair.bin'\nprebake_image = 'pair.qoi'\n",
		"chunked":     "[lightmap]\nenabled = true\noffline = true\n[chunks]\nenabled = true\n",
		"environment": "[lightmap]\nenabled = true\noffline = true\n[lightmap.environment]\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()

			directory := filepath.Join(root, "levels", "room")

			if err := os.MkdirAll(directory, 0o750); err != nil {
				t.Fatal(err)
			}

			writePrebakeFixture(t, filepath.Join(directory, "level.toml"), []byte("[level]\nname = 'room'\n"+settings))

			reports, err := BakeDevelopment(t.Context(), root, nil)
			if err != nil || len(reports) != 0 {
				t.Fatalf("skipped level attempted baking: %+v, %v", reports, err)
			}

			if _, err := os.Stat(filepath.Join(root, ".karty")); !os.IsNotExist(err) {
				t.Fatalf("skipped level wrote generated output: %v", err)
			}
		})
	}

	if reports, err := BakeDevelopment(t.Context(), t.TempDir(), nil); err != nil || len(reports) != 0 {
		t.Fatalf("project without levels: %+v, %v", reports, err)
	}
}

func TestDevelopmentBakeCancellationLeavesNoOutput(t *testing.T) {
	t.Parallel()
	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(t, root, automaticBakeSettings)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := BakeDevelopment(ctx, root, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled development bake: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".karty")); !os.IsNotExist(err) {
		t.Fatalf("canceled development bake published output: %v", err)
	}
}
