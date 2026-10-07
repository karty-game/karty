package levelbuild

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
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

	if want := filepath.Join(root, ".karty", "bakes", "lighting", automaticBakeManifest); reports[0].Manifest != want {
		t.Fatalf("manifest path=%q want=%q", reports[0].Manifest, want)
	}

	if entries, err := os.ReadDir(filepath.Join(root, "levels", "lighting")); err != nil || len(entries) != 3 {
		t.Fatalf("bake changed authored level directory: %v, %v", entries, err)
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

	// Explicit imports remain confined to the level; they cannot reach the
	// project cache using ../../ paths. Copy a pair to test stale import errors.
	importDirectory := filepath.Join(directory, ".karty", "imports")
	if err := os.MkdirAll(importDirectory, 0o750); err != nil {
		t.Fatal(err)
	}

	imageBytes, err := os.ReadFile(reports[0].Image)
	if err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(t, filepath.Join(importDirectory, automaticBakeManifest), first)
	writePrebakeFixture(t, filepath.Join(importDirectory, "lightmap.qoi"), imageBytes)

	manualControls := "prebake = \".karty/imports/lightmap-prebake.json\"\nprebake_image = \".karty/imports/lightmap.qoi\""
	writePrebakeFixture(t, manifestPath, []byte(strings.Replace(string(definitionBytes), "offline = true", manualControls, 1)))

	if _, err := BuildAllWithAssets(t.Context(), root, 4, "", selected); !errors.Is(err, worldlightmap.ErrLayout) {
		t.Fatalf("explicit offline import did not reject stale original albedo: %v", err)
	}

	writePrebakeFixture(t, manifestPath, definitionBytes)
	writePrebakeFixture(t, texturePath, originalTexture)
	build(true)
	// A denoiser change must not package a stale atlas with another preset.
	writePrebakeFixture(t, manifestPath, append(bytes.Clone(definitionBytes), []byte("bake_denoise = \"off\"\n")...))
	build(false)
	writePrebakeFixture(t, manifestPath, definitionBytes)
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

	writePrebakeFixture(t, reports[0].Image, imageBytes)

	escapingCache := filepath.Join(t.TempDir(), "bake")
	cache := filepath.Dir(reports[0].Manifest)

	if err := os.Rename(cache, escapingCache); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(escapingCache, cache); err != nil {
		t.Fatal(err)
	}

	build(false)
}

func TestOfflineBakeRejectsCacheSymlinkAndCancelledWrite(t *testing.T) {
	t.Parallel()

	for _, relative := range []string{".karty", ".karty/bakes", ".karty/bakes/lighting"} {
		t.Run(relative, func(t *testing.T) {
			t.Parallel()
			root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
			directory := filepath.Join(root, "levels", "lighting")
			path := filepath.Join(root, filepath.FromSlash(relative))

			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}

			outside := t.TempDir()
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}

			if _, err := prepareBakeDirectory(root, directory); !errors.Is(err, ErrSource) {
				t.Fatalf("bake followed escaping symlink: %v", err)
			}

			if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
				t.Fatal("bake wrote outside project")
			}
		})
	}

	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(t, root, "enabled = true\noffline = true\nlights = [\"red\"]\npage_size = 512\ndensity = 1\n")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := BakeAll(ctx, root, BakeOptions{}, nil); err == nil {
		t.Fatal("canceled bake succeeded")
	}

	if _, err := os.Stat(filepath.Join(root, ".karty")); !os.IsNotExist(err) {
		t.Fatalf("canceled bake created output: %v", err)
	}
}

func TestOfflineBakesAndPackagedLightsAreLevelLocal(t *testing.T) {
	t.Parallel()
	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(t, root,
		"enabled = true\noffline = true\nlights = [\"red\",\"blue\"]\npage_size = 512\ndensity = 1\nshadow_size = 32\n",
	)
	firstDirectory := filepath.Join(root, "levels", "lighting")
	secondDirectory := filepath.Join(root, "levels", "other")

	if err := os.CopyFS(secondDirectory, os.DirFS(firstDirectory)); err != nil {
		t.Fatal(err)
	}

	definition, err := os.ReadFile(filepath.Join(secondDirectory, "level.toml"))
	if err != nil {
		t.Fatal(err)
	}

	writePrebakeFixture(t,
		filepath.Join(secondDirectory, "level.toml"),
		[]byte(strings.Replace(string(definition), "levels.lighting", "levels.other", 1)),
	)
	writePrebakeFixture(t, filepath.Join(secondDirectory, "world.yaml"), []byte("version: 4\n"+lightingRoomYAML+
		strings.Replace(directLightmapYAML, "color: {x: 1, y: 0, z: 0}", "color: {x: 0.5, y: 0, z: 0}", 1)))
	// The obsolete project-global sidecar must never override either level.
	writePrebakeFixture(t, filepath.Join(root, "lighting.yaml"), []byte("invalid legacy global lighting"))
	selected := directLightmapSDK(t)

	build := func(wantFirst, wantSecond bool) {
		t.Helper()

		artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
		if err != nil || len(artifacts) != 2 {
			t.Fatalf("build levels: %v, %v", artifacts, err)
		}

		for _, artifact := range artifacts {
			wantBake, wantRed := wantFirst, 1.0
			if artifact.Name == "levels.other" {
				wantBake, wantRed = wantSecond, 0.5
			}

			envelope := executeLightmapEnvelope(t, artifact.Bytes)
			data, _, found := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)

			if !found {
				t.Fatal("missing packaged world")
			}

			document, err := world.Decode(data)
			if err != nil || document.Lighting == nil || len(document.Lighting.Lights) != 2 ||
				document.Lighting.Lights[0].Color.X != wantRed {
				t.Fatalf("%s lost its authored lights: %+v, %v", artifact.Name, document.Lighting, err)
			}

			if _, _, found := envelope.Read(worldlightmap.PrebakeEntryName, 0, worldlightmap.MaxPrebakeManifestSize); found != wantBake {
				t.Fatalf("%s prebake entry want=%v", artifact.Name, wantBake)
			}
		}
	}

	samples, bounces := 1, 0
	options := BakeOptions{Level: "levels.other", Samples: &samples, Bounces: &bounces, Workers: 1}

	reports, err := BakeAll(t.Context(), root, options, nil)
	if err != nil || len(reports) != 1 || reports[0].Manifest != filepath.Join(root, ".karty", "bakes", "other", automaticBakeManifest) {
		t.Fatalf("logical level selection: %v, %v", reports, err)
	}

	build(false, true)

	options.Level = "lighting"

	if reports, err := BakeAll(t.Context(), root, options, nil); err != nil || len(reports) != 1 {
		t.Fatalf("directory level selection: %v, %v", reports, err)
	}

	build(true, true)
	// Even identical light IDs and geometry cannot reuse another level's pair.
	entries, err := os.ReadDir(filepath.Join(root, ".karty", "bakes", "other"))
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(root, ".karty", "bakes", "other", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}

		writePrebakeFixture(t, filepath.Join(root, ".karty", "bakes", "lighting", entry.Name()), data)
	}

	build(false, true)
}
