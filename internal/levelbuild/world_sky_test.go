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
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldsky"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestWorldSkyPackagingCapabilityAndDeterminism(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	manifest = withoutRuntimeCapabilities(geometrySDK(manifest), asset.Capability(worldsky.Feature))
	input := "version: 12\nenvironment: {preset: daylight, sky: {cloud_sparse: cloud.png, horizon: cloud.png, settings: {horizon_haze: 0.4, horizon_uv_scale: 1.5}}}\n" + lightingRoomYAML
	root := lightingLevelFixture(t, input)

	var pixels bytes.Buffer

	texture := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	texture.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})

	if err := png.Encode(&pixels, texture); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "levels", "lighting", "cloud.png"), pixels.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	if artifacts, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest); !errors.Is(err, ErrManifest) || artifacts != nil {
		t.Fatalf("sky capability gate lost: %v", err)
	}

	manifest.Assets.Capabilities.Runtime = append(manifest.Assets.Capabilities.Runtime, asset.Capability(worldsky.Feature))

	first, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
	if err != nil {
		t.Fatal(err)
	}

	second, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Contains(first[0].Features, worldsky.Feature) || !bytes.Equal(first[0].Bytes, second[0].Bytes) {
		t.Fatal("sky packaging lost feature or determinism")
	}

	envelope, err := level.DecodePassiveModule(first[0].Bytes)
	if err != nil {
		t.Fatal(err)
	}

	encoded, _, found := envelope.Read(worldsky.EntryName, 0, level.MaxEntrySize)
	if !found {
		t.Fatal("sky sidecar missing")
	}

	sky, err := worldsky.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if sky.CloudSparse == nil || sky.Horizon == nil || sky.CloudDense != nil || sky.Background != nil || sky.Settings.HorizonHaze != .4 ||
		sky.Settings.HorizonUVScale != 1.5 ||
		sky.Settings.CloudUVScale != [2]float64{1, 1} {
		t.Fatalf("authored assets or partial settings changed: %+v", sky)
	}
}

func TestWorldSkySourcesRejectSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := lightingLevelFixture(t, "version: 12\nenvironment: {preset: daylight, sky: {horizon: escape.png}}\n"+lightingRoomYAML)

	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, []byte("outside PNG"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outside, filepath.Join(root, "levels", "lighting", "escape.png")); err != nil {
		t.Skip(err)
	}

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	manifest.Assets.Capabilities.Runtime = append(manifest.Assets.Capabilities.Runtime, asset.Capability(worldsky.Feature))
	if artifacts, err := BuildAllWithAssets(
		context.Background(),
		root,
		4,
		"",
		geometrySDK(manifest),
	); !errors.Is(err, ErrSource) ||
		artifacts != nil {
		t.Fatalf("escaping sky source accepted: %v", err)
	}
}

func TestWorldSkyAbsentKeepsLegacyLevelAssetFree(t *testing.T) {
	t.Parallel()

	root := lightingLevelFixture(t, "version: 9\nenvironment: {preset: daylight}\n"+lightingRoomYAML)

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	manifest = withoutRuntimeCapabilities(geometrySDK(manifest), asset.Capability(worldsky.Feature))

	artifacts, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
	if err != nil {
		t.Fatal(err)
	}

	if slices.Contains(artifacts[0].Features, worldsky.Feature) {
		t.Fatal("unauthored sky added a runtime requirement")
	}

	envelope, err := level.DecodePassiveModule(artifacts[0].Bytes)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, found := envelope.Read(worldsky.EntryName, 0, level.MaxEntrySize); found {
		t.Fatal("unauthored sky packaged artwork")
	}
}
