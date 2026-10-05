package levelbuild

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/testfixture"
)

func enableFixtureLightmap(t *testing.T, root, settings string) {
	t.Helper()

	path := filepath.Join(root, "levels", "lighting", "level.toml")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G703 -- root is an isolated test fixture from t.TempDir.
	if err := os.WriteFile(path, append(data, []byte("\n[lightmap]\n"+settings)...), 0o600); err != nil {
		t.Fatal(err)
	}
}

func executeLightmapEnvelope(t *testing.T, module []byte) level.Envelope {
	t.Helper()

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("actual WASM execution needs pinned Node")
	}

	path := filepath.Join(t.TempDir(), "lightmap.kld")
	if err := os.WriteFile(path, module, 0o600); err != nil {
		t.Fatal(err)
	}

	const script = `import fs from 'node:fs'; const {instance}=await WebAssembly.instantiate(fs.readFileSync(process.argv[1])); const e=instance.exports; if(e.karty_level_abi_version()!==1)throw new Error('ABI');process.stdout.write(Buffer.from(e.memory.buffer,e.karty_level_data_pointer(),e.karty_level_data_length()));`

	raw, err := exec.CommandContext(t.Context(), node, "--input-type=module", "-e", script, path).Output()
	if err != nil {
		t.Fatal(err)
	}

	envelope, err := level.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	return envelope
}

func TestWorldLightmapCapabilityGateAndPackagedWASM(t *testing.T) {
	t.Parallel()

	selected, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	root := lightingLevelFixture(
		t,
		"version: 4\n"+lightingRoomYAML+strings.Replace(levelLightingYAML(1), "x: 0, y: 99, z: 8", "x: 1, y: 1, z: 2", 1),
	)
	enableFixtureLightmap(t, root, "enabled = true\nlight = \"light-1\"\nshadow_size = 256\n")

	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldLightingV1)
	if artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected); !errors.Is(err, ErrManifest) || artifacts != nil {
		t.Fatalf("SDK without capability accepted lightmaps: %v", err)
	}

	selected.Version = "0.0.8"
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldLightmapsV1)

	first, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}

	second, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || len(first) != 1 || len(second) != 1 || !bytes.Equal(first[0].Bytes, second[0].Bytes) {
		t.Fatalf("nondeterministic packaging: %v", err)
	}

	if !slices.Contains(first[0].Features, worldlightmap.Feature) {
		t.Fatal("missing cartridge capability")
	}

	envelope := executeLightmapEnvelope(t, first[0].Bytes)

	worldData, _, found := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)
	if !found {
		t.Fatal("missing world")
	}

	document, err := world.Decode(worldData)
	if err != nil {
		t.Fatal(err)
	}

	encoded, _, found := envelope.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)
	if !found {
		t.Fatal("missing lightmap layout")
	}

	layout, err := worldlightmap.Decode(encoded, &document)
	if err != nil {
		t.Fatal(err)
	}

	if layout.RuntimeBake == nil || layout.RuntimeBake.LightID != "light-1" || layout.RuntimeBake.ShadowSize != 256 ||
		layout.TexelsPerUnit != 16 ||
		len(layout.Charts) != 6 {
		t.Fatalf("packaged recipe: %+v", layout.RuntimeBake)
	}

	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(
		envelope.Metadata,
		&metadata,
	); err != nil ||
		string(metadata[worldlightmap.MetadataKey]) != `"karty.world-lightmaps@1"` {
		t.Fatal("missing metadata marker")
	}
}

func TestWorldLightmapLayoutOnlyAndInvalidControls(t *testing.T) {
	t.Parallel()

	selected, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldLightmapsV1)

	for name, settings := range map[string]string{"layout only": "enabled = true\n", "zero density": "enabled = true\ndensity = 0\n", "large page": "enabled = true\npage_size = 2048\n", "two pages": "enabled = true\nmax_pages = 2\n", "small budget": "enabled = true\nmax_texels = 10\n", "unknown light": "enabled = true\nlight = \"missing\"\n", "large shadow": "enabled = true\nshadow_size = 513\n"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			root := lightingLevelFixture(t, "version: 1\n"+lightingRoomYAML)
			enableFixtureLightmap(t, root, settings)

			artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
			if name != "layout only" {
				if err == nil || artifacts != nil {
					t.Fatal("accepted invalid settings")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			envelope := executeLightmapEnvelope(t, artifacts[0].Bytes)
			worldData, _, _ := envelope.Read(world.EntryName, 0, world.MaxEncodedSize)

			document, err := world.Decode(worldData)
			if err != nil {
				t.Fatal(err)
			}

			encoded, _, _ := envelope.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)

			layout, err := worldlightmap.Decode(encoded, &document)
			if err != nil || layout.RuntimeBake != nil {
				t.Fatalf("layout only implicitly selected light: %v", err)
			}
		})
	}
}

func TestRomanBuildLightmapSemanticLayout(t *testing.T) {
	t.Parallel()

	selected, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	selected.Version = "0.0.8"
	selected.Assets.Capabilities.Runtime = append(
		selected.Assets.Capabilities.Runtime,
		asset.CapabilityWorldMaterialMappingV1,
		asset.CapabilityWorldStaticSolidsV1,
		asset.CapabilityWorldLightmapsV1,
	)

	root := testfixture.WorldCameraGeometry(t, filepath.Join("..", "..", "samples", "world-camera"))

	path := filepath.Join(root, "levels", "showcase", "level.toml")

	definition, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G703 -- this path is constructed beneath the local t.TempDir fixture.
	if err := os.WriteFile(path, append(definition, []byte("\n[lightmap]\nenabled = true\n")...), 0o600); err != nil {
		t.Fatal(err)
	}

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

	encoded, _, _ := envelope.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize)

	layout, err := worldlightmap.Decode(encoded, &document)
	if err != nil {
		t.Fatal(err)
	}

	sides := 0

	for _, b := range layout.Bindings {
		if b.Kind == "solid-side" {
			sides++
		}
	}

	if sides != 80 || len(layout.Pages) != 1 || layout.Pages[0].Width != 1024 {
		t.Fatalf("Roman per-edge layout: sides=%d pages=%v", sides, layout.Pages)
	}

	t.Logf(
		"actual KLD WASM: %d semantic bindings, %d charts, %d solid sides, %d layout bytes",
		len(layout.Bindings),
		len(layout.Charts),
		sides,
		len(encoded),
	)
}

func TestWorldLightmapAndMaterialAtlasMarkersCoexist(t *testing.T) {
	t.Parallel()

	selected, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime,
		asset.CapabilityWorldLightmapsV1, asset.CapabilityWorldMaterialAtlasV1)
	root := lightingLevelFixture(t, "version: 1\n"+lightingRoomYAML)
	enableFixtureLightmap(t, root, "enabled = true\n")
	assets := &assetBuild{projectRoot: root, manifest: selected,
		materials: func(_ context.Context, _ string, _ sdk.Manifest, document world.Document,
			_ map[uint32][]byte) (worldmaterial.Pair, assetpipeline.Artifact, error) {
			return fixtureWorldMaterialPair(t, assetpipeline.WorldMaterialIDs(document)), assetpipeline.Artifact{}, nil
		},
	}

	artifacts, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(err)
	}

	envelope := executeLightmapEnvelope(t, artifacts[0].Bytes)

	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Metadata, &metadata); err != nil {
		t.Fatal(err)
	}

	if _, ok := metadata[worldlightmap.MetadataKey]; !ok {
		t.Fatal("atlas packaging removed lightmap marker")
	}

	if _, ok := metadata[worldmaterial.MetadataKey]; !ok {
		t.Fatal("lightmap packaging removed atlas marker")
	}

	if _, _, found := envelope.Read(worldlightmap.EntryName, 0, worldlightmap.MaxEncodedSize); !found {
		t.Fatal("atlas packaging removed lightmap layout")
	}
}
