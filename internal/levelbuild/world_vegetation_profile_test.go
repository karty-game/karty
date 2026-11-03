package levelbuild

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	worldlightmapbake "github.com/karty-game/karty-sdk/bake/worldlightmap"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/pelletier/go-toml/v2"
)

func vegetationProfileFixture(t *testing.T) (string, string, manifest) {
	t.Helper()

	rooms := strings.Replace(lightingRoomYAML, "    ceiling:",
		"    vegetation_layers: [{preset: grass, density: 4, seed: 42, surface: floor}]\n    ceiling:", 1)
	root := lightingLevelFixture(t, "version: 13\n"+rooms+levelVegetationYAML+directLightmapYAML)
	directory := filepath.Join(root, "levels", "lighting")

	encoded, err := os.ReadFile(filepath.Join(directory, "level.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var definition manifest
	if err := toml.Unmarshal(encoded, &definition); err != nil {
		t.Fatal(err)
	}

	return root, directory, definition
}

func TestCurrentGPUVegetationOmissionPreservesAuthoredSource(t *testing.T) {
	t.Parallel()

	root, directory, _ := vegetationProfileFixture(t)
	path := filepath.Join(directory, "world.yaml")

	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	selected, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	selected = withoutRuntimeCapabilities(currentWorldSDK(selected), asset.Capability(worldvegetation.Feature))

	artifacts, err := buildAllWithFixtureMaterials(t, root, 4, "", selected)
	if err != nil || len(artifacts) != 1 {
		t.Fatal("WebGPU V1 omission requires no vegetation runtime capability", err)
	}

	for _, feature := range artifacts[0].Features {
		if strings.HasPrefix(feature, "world/vegetation@") {
			t.Fatal("omitted procedural vegetation is still advertised", feature)
		}
	}

	envelope := executeLightmapEnvelope(t, artifacts[0].Bytes)
	if _, _, found := envelope.Read(worldvegetation.EntryName, 0, worldvegetation.MaxEncodedSize); found {
		t.Fatal("WebGPU V1 packaged procedural vegetation")
	}

	second, err := buildAllWithFixtureMaterials(t, root, 4, "", selected)
	if err != nil || len(second) != 1 || !bytes.Equal(artifacts[0].Bytes, second[0].Bytes) {
		t.Fatal("warm current profile changed prepared output", err)
	}

	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, current) {
		t.Fatal("private profile modified authored source", err)
	}
}

func TestWebGPUVegetationBakeTransportAndStaleImports(t *testing.T) {
	t.Parallel()

	_, directory, definition := vegetationProfileFixture(t)

	metadata, materials, err := loadBakeMaterials(directory, definition)
	if err != nil {
		t.Fatal(err)
	}

	document, _, err := compileMaterialWorld(directory, definition, metadata, nil)
	if err != nil {
		t.Fatal(err)
	}

	legacy, err := prepareAuthoredVegetationBake(directory, definition, &document, metadata, materials)
	if err != nil || legacy == nil {
		t.Fatal("legacy bake lost authored plant transport", err)
	}

	profile := vegetationBuildProfile(definition, &assetBuild{})

	omitted, err := prepareAuthoredVegetationBake(directory, profile, &document, metadata, materials)
	if err != nil || omitted != nil || definition.omitVegetation {
		t.Fatal("WebGPU retained invisible plant casters or mutated legacy definition", err)
	}

	layout, err := worldlightmap.Compile(document, worldlightmap.Options{Lights: []string{"red", "blue"}, PageSize: 512, TexelsPerUnit: 1})
	if err != nil {
		t.Fatal(err)
	}

	digest, err := worldlightmapbake.ReflectanceDigestForLayout(document, layout, materials,
		worldlightmapbake.Options{Samples: 4, Bounces: 1, Seed: 1, Vegetation: legacy})
	if err != nil {
		t.Fatal(err)
	}

	prebake := worldlightmap.PrebakeManifest{ReflectanceSHA256: digest, Samples: 4, Bounces: 1, Seed: 1}
	if err := validateOfflineReflectance(directory, definition, document, layout, prebake); err != nil {
		t.Fatal("current legacy transport digest rejected", err)
	}

	if err := validateOfflineReflectance(directory, profile, document, layout, prebake); !errors.Is(err, worldlightmap.ErrLayout) {
		t.Fatal("WebGPU accepted a stale prebake containing invisible vegetation shadows", err)
	}
}

func TestWebGPUVegetationOmissionRetainsSourceValidation(t *testing.T) {
	t.Parallel()

	_, directory, definition := vegetationProfileFixture(t)
	path := filepath.Join(directory, "world.yaml")

	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	invalid := bytes.Replace(encoded, []byte("density: 4"), []byte("density: -1"), 1)
	// #nosec G703 -- path is a fixed fixture filename beneath t.TempDir.
	if err := os.WriteFile(path, invalid, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := compileWorldVegetation(directory, definition, &world.Document{}, nil, &assetBuild{}); err == nil {
		t.Fatal("omission bypassed authored vegetation source validation")
	}
}

func TestWebGPUVegetationPolicyPrecedesScreenshotBake(t *testing.T) {
	t.Parallel()

	root, directory, _ := vegetationProfileFixture(t)
	enableFixtureLightmap(t, root, "enabled = true\nlights = [\"red\",\"blue\"]\npage_size = 512\ndensity = 1\nshadow_size = 32\n")

	encoded, err := os.ReadFile(filepath.Join(directory, "level.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var definition manifest
	if err := toml.Unmarshal(encoded, &definition); err != nil {
		t.Fatal(err)
	}

	profile, err := prepareLevelWorld(t.Context(), directory, definition,
		&assetBuild{manifest: directLightmapSDK(t), screenshot: true, projectRoot: root})
	if err != nil || !profile.omitVegetation || definition.omitVegetation {
		t.Fatal("screenshot bake did not receive its private vegetation policy", err)
	}

	metadata, _, err := loadBakeMaterials(directory, profile)
	if err != nil {
		t.Fatal(err)
	}

	document, _, err := compileMaterialWorld(directory, profile, metadata, nil)
	if err != nil {
		t.Fatal(err)
	}

	options, err := profile.Lightmap.options()
	if err != nil {
		t.Fatal(err)
	}

	layout, err := worldlightmap.Compile(document, options)
	if err != nil {
		t.Fatal(err)
	}

	if len(readAutomaticPrebake(root, directory, profile, layout, document)) == 0 {
		t.Fatal("profile screenshot bake did not publish validated vegetation-free transport")
	}

	profile.omitVegetation = false
	if len(readAutomaticPrebake(root, directory, profile, layout, document)) != 0 {
		t.Fatal("vegetation-free screenshot bake was reused as legacy plant-shadow transport")
	}
}
