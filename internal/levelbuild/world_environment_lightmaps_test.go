package levelbuild

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
)

func TestEnvironmentVariantsPackagedModulesWarmReuseAndCorruptCache(t *testing.T) {
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
		"enabled = true\noffline = true\ndensity = 2\npage_size = 512\nbake_samples = 2\nbake_bounces = 1\nbake_denoise = 'low'\n[lightmap.environment]\ntimes = [0,15]\ndefault_time = 15\nweather_variants = true\n",
	)
	enableFixtureChunks(t, root, "")
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldLightmapsEnvironmentVariantsV1)

	first, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}

	if len(first) != 1 || len(first[0].Chunks) != 5 || !slices.Contains(first[0].Features, worldlightmap.EnvironmentVariantsFeature) ||
		slices.Contains(first[0].Features, worldlightmap.PrebakeFeature) {
		t.Fatalf("render/condition resources or capabilities lost: %+v", first)
	}

	envelope := decodePassiveEnvelope(t, first[0].Bytes)
	worldBytes, _, _ := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(worldBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootCatalogBytes, _, _ := envelope.Read(worldchunk.CatalogEntryName, 0, worldchunk.MaxEncodedSize)

	rootCatalog, err := worldchunk.DecodeCatalog(rootCatalogBytes, &document)
	if err != nil || len(rootCatalog.Chunks) != 1 {
		t.Fatalf("render chunk catalog: %v", err)
	}

	child := executeLightmapEnvelope(t, first[0].Chunks[0].Bytes)
	layoutBytes, _, _ := child.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)

	layout, err := worldlightmap.Decode(layoutBytes, &document)
	if err != nil || layout.RuntimeBake != nil {
		t.Fatalf("variant chart recipe: %v", err)
	}

	catalogBytes, _, _ := child.Read(worldlightmap.EnvironmentVariantsEntryName, 0, worldlightmap.MaxEnvironmentCatalogSize)

	catalog, err := worldlightmap.DecodeEnvironmentVariants(catalogBytes, layout, &document)
	if err != nil {
		t.Fatal(err)
	}

	defaultImage, _, _ := child.Read(worldlightmap.EnvironmentImageEntryName, 0, worldlightmap.MaxPrebakeImageSize)
	if catalog.Default != 2 || worldlightmap.ValidateEnvironmentImage(catalog.Variants[catalog.Default], defaultImage, layout) != nil {
		t.Fatal("default 15:00 clear map missing")
	}

	for variantIndex, variant := range catalog.Variants {
		artifact := first[0].Chunks[variantIndex+1]

		digest := sha256.Sum256(artifact.Bytes)
		if hex.EncodeToString(digest[:]) != variant.ContentSHA256 || uint64(len(artifact.Bytes)) != variant.Size {
			t.Fatal("condition module hash or size mismatch")
		}

		resource := decodePassiveEnvelope(t, artifact.Bytes)
		if len(resource.Entries) != 1 {
			t.Fatal("condition module carries unrelated entries")
		}

		image, _, _ := resource.Read(worldlightmap.EnvironmentImageEntryName, 0, worldlightmap.MaxPrebakeImageSize)
		if worldlightmap.ValidateEnvironmentImage(variant, image, layout) != nil {
			t.Fatal("condition image failed validation")
		}
	}

	warm, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || !bytes.Equal(warm[0].Bytes, first[0].Bytes) {
		t.Fatalf("warm build differs: %v", err)
	}

	for i, artifact := range first[0].Chunks {
		if !bytes.Equal(artifact.Bytes, warm[0].Chunks[i].Bytes) {
			t.Fatal("warm resource differs")
		}
	}

	cache := filepath.Join(root, ".karty", "bakes", "lighting", chunkPixelCacheDirectory)

	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 4 {
		t.Fatalf("pixel cache resources=%d: %v", len(entries), err)
	}

	writePrebakeFixture(t, filepath.Join(cache, entries[0].Name()), []byte("corrupt"))

	repaired, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || !bytes.Equal(repaired[0].Bytes, first[0].Bytes) {
		t.Fatalf("corrupt cache not safely rebuilt: %v", err)
	}

	without := withoutRuntimeCapabilities(selected, asset.CapabilityWorldLightmapsEnvironmentVariantsV1)
	if _, err := BuildAllWithAssets(t.Context(), root, 4, "", without); !errors.Is(err, ErrManifest) {
		t.Fatalf("missing variant capability accepted: %v", err)
	}
}

func TestEnvironmentTimeOnlyBuildDefaultsToExactlyFourImages(t *testing.T) {
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

	artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}

	if len(artifacts) != 1 || len(artifacts[0].Chunks) != 5 {
		t.Fatal("time-only build must contain one render chunk and four images")
	}

	rootEnvelope := decodePassiveEnvelope(t, artifacts[0].Bytes)
	worldBytes, _, _ := rootEnvelope.Read(world.EntryName, 0, world.MaxEncodedSize)

	document, err := world.Decode(worldBytes)
	if err != nil {
		t.Fatal(err)
	}

	child := decodePassiveEnvelope(t, artifacts[0].Chunks[0].Bytes)
	layoutBytes, _, _ := child.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)

	layout, err := worldlightmap.Decode(layoutBytes, &document)
	if err != nil {
		t.Fatal(err)
	}

	catalogBytes, _, _ := child.Read(worldlightmap.EnvironmentVariantsEntryName, 0, worldlightmap.MaxEnvironmentCatalogSize)

	catalog, err := worldlightmap.DecodeEnvironmentVariants(catalogBytes, layout, &document)
	if err != nil || len(catalog.Variants) != 4 || catalog.Default != 2 {
		t.Fatalf("time-only catalog/default: %v", err)
	}

	for variantIndex, variant := range catalog.Variants {
		if variant.TimeOfDay != []float64{0, 9, 12, 17}[variantIndex] || variant.CloudAmount != 0 {
			t.Fatal("unexpected weather bake or clock")
		}

		resource := decodePassiveEnvelope(t, artifacts[0].Chunks[variantIndex+1].Bytes)

		image, _, _ := resource.Read(worldlightmap.EnvironmentImageEntryName, 0, worldlightmap.MaxPrebakeImageSize)
		if len(resource.Entries) != 1 || worldlightmap.ValidateEnvironmentImage(variant, image, layout) != nil {
			t.Fatal("time-only passive image resource failed validation")
		}

		for _, cloud := range []float64{0, .35, .7, 1} {
			first, second, blend, err := worldlightmap.SelectEnvironmentVariants(
				&catalog,
				world.EnvironmentSettings{TimeOfDay: variant.TimeOfDay, CloudAmount: cloud},
			)
			if err != nil || first != variantIndex || second != first || blend != 0 {
				t.Fatal("cloud control selected another image")
			}
		}
	}

	defaultImage, _, _ := child.Read(worldlightmap.EnvironmentImageEntryName, 0, worldlightmap.MaxPrebakeImageSize)
	if worldlightmap.ValidateEnvironmentImage(catalog.Variants[2], defaultImage, layout) != nil {
		t.Fatal("default noon image missing")
	}

	cache, err := os.ReadDir(filepath.Join(root, ".karty", "bakes", "lighting", chunkPixelCacheDirectory))
	if err != nil || len(cache) != 4 {
		t.Fatalf("expected four time-only bake cache files: count=%d err=%v", len(cache), err)
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
