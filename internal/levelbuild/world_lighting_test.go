package levelbuild

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

const lightingRoomYAML = `rooms:
  - id: room
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 2, y: 0}, material: wall}
      - {id: east, start: {x: 2, y: 0}, end: {x: 2, y: 2}, material: wall}
      - {id: north, start: {x: 2, y: 2}, end: {x: 0, y: 2}, material: wall}
      - {id: west, start: {x: 0, y: 2}, end: {x: 0, y: 0}, material: wall}
    floor: {c: 0}
    ceiling: {c: 4}
    floor_material: wall
    ceiling_material: wall
`

func TestWorldLightingCapabilityGateAndPackagedWASM(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	if manifest.Version != "0.0.7" || slices.Contains(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldLightingV1) {
		t.Fatal("released SDK gained candidate lighting capability")
	}

	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+levelLightingYAML(50))

	if artifacts, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest); !errors.Is(err, ErrManifest) || artifacts != nil {
		t.Fatalf("released SDK accepted candidate lighting: %+v, %v", artifacts, err)
	}
	// The optional lighting capability is sufficient on its own. Materialize
	// generation and atlas capability are independent of static lighting.
	manifest.Version = "0.0.8"
	manifest.Assets.Capabilities.Runtime = append(slices.Clone(manifest.Assets.Capabilities.Runtime), asset.CapabilityWorldLightingV1)

	first, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
	if err != nil {
		t.Fatal(err)
	}

	second, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
	if err != nil {
		t.Fatal(err)
	}

	if len(first) != 1 || len(second) != 1 || !bytes.Equal(first[0].Bytes, second[0].Bytes) ||
		!slices.Equal(first[0].Features, []string{cartridge.FeatureTextureQOIv1, world.FeatureLighting, world.Feature}) {
		t.Fatalf("lighting features or deterministic packaging lost: %+v", first)
	}

	document := executePackagedLightingWorld(t, first[0].Bytes)
	if document.Version != world.Version || document.Lighting == nil ||
		document.Lighting.Version != 1 || document.Lighting.Ambient != (world.Vec3{X: .1, Y: .2, Z: .3}) || len(document.Lighting.Lights) != 50 {
		t.Fatalf("packaged lighting contract changed: %+v", document.Lighting)
	}

	for index, light := range document.Lighting.Lights {
		want := world.PointLight{
			ID: fmt.Sprintf("light-%d", 50-index), Position: world.Vec3{X: -float64(index), Y: 99, Z: 8},
			Color: world.Vec3{X: .5, Y: .25, Z: 1}, Radius: 6,
		}
		if light != want {
			t.Fatalf("global coordinates/order changed at light %d: %+v", index, light)
		}
	}
}

func TestWorldLightingEmptyAndRejectedBatches(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	manifest.Assets.Capabilities.Runtime = append(slices.Clone(manifest.Assets.Capabilities.Runtime), asset.CapabilityWorldLightingV1)

	for name, test := range map[string]struct {
		lighting string
		want     error
	}{
		"ambient only": {levelLightingYAML(0), nil},
		"51":           {levelLightingYAML(51), worldsource.ErrBounds},
		"last invalid": {strings.Replace(levelLightingYAML(50), "id: light-1,", "id: light-1, unexpected: true,", 1), nil},
		"last radius":  {strings.Replace(levelLightingYAML(50), "x: -49, y: 99, z: 8}, color: {x: 0.5, y: 0.25, z: 1}, radius: 6", "x: -49, y: 99, z: 8}, color: {x: 0.5, y: 0.25, z: 1}, radius: 0", 1), worldsource.ErrLighting},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+test.lighting)

			artifacts, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
			if name != "ambient only" {
				if err == nil || artifacts != nil || test.want != nil && !errors.Is(err, test.want) {
					t.Fatalf("malformed lighting produced artifact: %+v, %v", artifacts, err)
				}

				return
			}

			if err != nil || len(artifacts) != 1 || !slices.Contains(artifacts[0].Features, world.FeatureLighting) {
				t.Fatalf("ambient-only payload lost feature: %+v, %v", artifacts, err)
			}

			document := executePackagedLightingWorld(t, artifacts[0].Bytes)
			if document.Lighting == nil || len(document.Lighting.Lights) != 0 {
				t.Fatalf("ambient-only payload lost: %+v", document.Lighting)
			}
		})
	}
}

func TestWorldLightingAndMaterialAtlasPackageTogether(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	manifest.Version = "0.0.8"
	manifest.Assets.Capabilities.Runtime = append(slices.Clone(manifest.Assets.Capabilities.Runtime),
		asset.CapabilityWorldLightingV1, asset.CapabilityWorldMaterialAtlasV1)
	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+levelLightingYAML(2))
	calls := 0

	var generated worldmaterial.Pair

	assets := &assetBuild{projectRoot: root, manifest: manifest,
		materials: func(_ context.Context, _ string, _ sdk.Manifest, document world.Document,
			_ map[uint32][]byte) (worldmaterial.Pair, assetpipeline.Artifact, error) {
			calls++

			if document.Lighting == nil || len(document.Lighting.Lights) != 2 {
				t.Fatal("atlas generation lost the world lighting payload")
			}

			generated = fixtureWorldMaterialPair(t, assetpipeline.WorldMaterialIDs(document))

			return generated, assetpipeline.Artifact{CacheKey: "fixture"}, nil
		},
	}

	artifacts, err := buildAll(context.Background(), root, 4, "", assets)
	if err != nil || len(artifacts) != 1 || calls != 1 ||
		!slices.Contains(artifacts[0].Features, world.FeatureLighting) || !slices.Contains(artifacts[0].Features, worldmaterial.Feature) {
		t.Fatalf("lighting and atlas did not package together: %+v, calls=%d, err=%v", artifacts, calls, err)
	}

	document := executePackagedLightingWorld(t, artifacts[0].Bytes)
	if document.Lighting == nil || len(document.Lighting.Lights) != 2 {
		t.Fatal("atlas packaging removed compiled lighting")
	}

	checkWorldMaterialModule(t, artifacts[0].Bytes, generated)
}

func TestAbsentLightingKeepsReleasedSDKPackagingAcrossSourceVersions(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	var original Artifact

	for version := 1; version <= 4; version++ {
		root := lightingLevelFixture(t, fmt.Sprintf("version: %d\n%s", version, lightingRoomYAML))

		legacy, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
		if err != nil || len(legacy) != 1 {
			t.Fatalf("released build v%d failed: %+v, %v", version, legacy, err)
		}

		if version == 1 {
			original = legacy[0]
		}

		candidate := manifest
		candidate.Assets.Capabilities.Runtime = append(slices.Clone(manifest.Assets.Capabilities.Runtime), asset.CapabilityWorldLightingV1)

		unlit, err := BuildAllWithAssets(context.Background(), root, 4, "", candidate)
		if err != nil || len(unlit) != 1 || !bytes.Equal(unlit[0].Bytes, original.Bytes) ||
			!reflect.DeepEqual(unlit[0].Features, original.Features) || slices.Contains(legacy[0].Features, world.FeatureLighting) {
			t.Fatalf("absent lighting changed released packaging v%d: %+v, %v", version, unlit, err)
		}
	}
}

func lightingLevelFixture(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()

	directory := filepath.Join(root, "levels", "lighting")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}

	const manifest = `[level]
name = "levels.lighting"
kind = "world"
[world]
source = "world.yaml"
[[textures]]
name = "wall"
source = "wall.png"
`

	imageData := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	imageData.SetNRGBA(0, 0, color.NRGBA{R: 180, A: 255})

	var texture bytes.Buffer
	if err := png.Encode(&texture, imageData); err != nil {
		t.Fatal(err)
	}

	for name, data := range map[string][]byte{"level.toml": []byte(manifest), "world.yaml": []byte(source), "wall.png": texture.Bytes()} {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

func levelLightingYAML(count int) string {
	var lighting strings.Builder
	lighting.WriteString("lighting:\n  version: 1\n  ambient: {x: 0.1, y: 0.2, z: 0.3}\n  lights:\n")

	for index := range count {
		fmt.Fprintf(
			&lighting,
			"    - {id: light-%d, position: {x: %d, y: 99, z: 8}, color: {x: 0.5, y: 0.25, z: 1}, radius: 6}\n",
			count-index,
			-index,
		)
	}

	return lighting.String()
}

func executePackagedLightingWorld(t *testing.T, module []byte) world.Document {
	t.Helper()

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("passive WASM execution needs Node (provided by root mise tasks)")
	}

	path := filepath.Join(t.TempDir(), "lighting.kld")
	if err := os.WriteFile(path, module, 0o600); err != nil {
		t.Fatal(err)
	}

	const script = `import fs from 'node:fs';
const {instance} = await WebAssembly.instantiate(fs.readFileSync(process.argv[1]));
const e = instance.exports;
if (e.karty_level_abi_version() !== 1) throw new Error('level ABI');
process.stdout.write(Buffer.from(e.memory.buffer, e.karty_level_data_pointer(), e.karty_level_data_length()));`

	encoded, err := exec.CommandContext(t.Context(), node, "--input-type=module", "-e", script, path).Output()
	if err != nil {
		t.Fatal(err)
	}

	envelope, err := level.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	encoded, _, found := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)
	if !found {
		t.Fatal("packaged world entry missing")
	}

	document, err := world.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	return document
}
