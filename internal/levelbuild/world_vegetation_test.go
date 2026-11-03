package levelbuild

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

const levelVegetationYAML = `vegetation:
  version: 1
  presets:
    - {name: grass, texture: wall, width: {min: 0.2, max: 0.4}, height: {min: 0.2, max: 0.5}, wind_amplitude: 0.08, wind_frequency: 0.7, draw_distance: 32}
`

func TestCurrentGPUProfileOmitsProceduralVegetationShapes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		source   uint16
		version  uint16
		geometry uint8
		feature  string
	}{
		{"cards", 13, worldvegetation.Version, worldvegetation.GeometryCrossedCards, worldvegetation.Feature},
		{"shrub", 14, worldvegetation.GeometryVersion, worldvegetation.GeometryShrub, worldvegetation.FeatureV2},
		{"normal map", 15, worldvegetation.NormalVersion, worldvegetation.GeometryShrub, worldvegetation.FeatureV3},
		{"tree", 16, worldvegetation.TreeVersion, worldvegetation.GeometryTree, worldvegetation.FeatureV4},
		{"palm", 20, worldvegetation.PalmVersion, worldvegetation.GeometryPalm, worldvegetation.FeatureV5},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			testOmittedVegetationWASM(t, test.source, test.version, test.geometry, test.feature)
		})
	}
}

func testOmittedVegetationWASM(t *testing.T, sourceVersion, version uint16, geometry uint8, feature string) {
	t.Helper()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	manifest = withoutRuntimeCapabilities(currentWorldSDK(manifest), asset.Capability(feature))
	rooms := strings.Replace(
		lightingRoomYAML,
		"    ceiling:",
		"    vegetation_layers: [{preset: grass, density: 4, seed: 42, surface: floor}]\n    ceiling:",
		1,
	)

	presets := levelVegetationYAML
	if version >= worldvegetation.GeometryVersion {
		presets = strings.Replace(presets, "version: 1", fmt.Sprintf("version: %d", version), 1)
		presets = strings.Replace(presets, "name: grass,", "name: grass, geometry: shrub,", 1)
	}

	if geometry == worldvegetation.GeometryTree {
		presets = strings.Replace(presets, "geometry: shrub", "geometry: tree", 1)
	}

	if geometry == worldvegetation.GeometryPalm {
		presets = strings.Replace(presets, "geometry: shrub", "geometry: palm", 1)
	}

	if version >= worldvegetation.NormalVersion {
		presets = strings.Replace(presets, "texture: wall,", "texture: wall, normal_texture: wall,", 1)
	}

	root := lightingLevelFixture(t, fmt.Sprintf("version: %d\n", sourceVersion)+rooms+presets)

	first, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
	if err != nil {
		t.Fatal(err)
	}

	second, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
	if err != nil {
		t.Fatal(err)
	}

	if len(first) != 1 || !bytes.Equal(first[0].Bytes, second[0].Bytes) || slices.Contains(first[0].Features, feature) ||
		!slices.Contains(first[0].Features, worldgpuchunk.Feature) {
		t.Fatal("current vegetation omission is not deterministic or lost GPU cells")
	}

	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "particles.kld")

	if err := os.WriteFile(path, first[0].Bytes, 0o600); err != nil {
		t.Fatal(err)
	}

	const script = `import fs from 'node:fs';const {instance}=await WebAssembly.instantiate(fs.readFileSync(process.argv[1]));const e=instance.exports;if(e.karty_level_abi_version()!==2)throw new Error('ABI');process.stdout.write(Buffer.from(e.memory.buffer,e.karty_level_data_pointer(),e.karty_level_data_length()));`

	encoded, err := exec.CommandContext(t.Context(), node, "--input-type=module", "-e", script, path).Output()
	if err != nil {
		t.Fatal(err)
	}

	envelope, err := level.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, found := envelope.Read(worldvegetation.EntryName, 0, worldvegetation.MaxEncodedSize); found {
		t.Fatal("unsupported vegetation published in current GPU module")
	}

	data, _, found := envelope.Read(worldgpuchunk.CatalogEntry, 0, level.MaxEntrySize)

	catalog, err := worldgpuchunk.DecodeCatalog(data)
	if !found || err != nil || len(catalog.Cells) == 0 {
		t.Fatal("executed WASM lost current geometry catalog", err)
	}

	source, err := os.ReadFile(filepath.Join(root, "levels", "lighting", "world.yaml"))
	if err != nil || !strings.Contains(string(source), "density: 4, seed: 42") ||
		!strings.Contains(string(source), "wind_amplitude: 0.08") {
		t.Fatal("omission altered authored vegetation", err)
	}
}

func TestVegetationOwnedEntry(t *testing.T) {
	t.Parallel()

	if _, err := loadData(t.TempDir(), []dataEntry{{Name: worldvegetation.EntryName, Source: "manual.bin"}}); !errors.Is(err, ErrSource) {
		t.Fatalf("reserved sidecar accepted: %v", err)
	}
}

func TestVegetationNormalDimensions(t *testing.T) {
	t.Parallel()

	document := &worldvegetation.Document{Presets: []worldvegetation.Preset{{Texture: 1, NormalTexture: 2}}}

	textures := []metadataTexture{{ID: 1, Width: 256, Height: 256}, {ID: 2, Width: 128, Height: 256}}
	if err := validateVegetationNormalDimensions(document, textures); !errors.Is(err, ErrSource) {
		t.Fatal("mismatched normal accepted", err)
	}

	textures[1].Width = 256
	if err := validateVegetationNormalDimensions(document, textures); err != nil {
		t.Fatal(err)
	}

	document.Presets[0].NormalTexture = 0

	textures = textures[:1]
	if err := validateVegetationNormalDimensions(document, textures); err != nil {
		t.Fatal("optional normal required", err)
	}
}
