package levelbuild

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldgpuchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldmaterialgpu"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

func TestGPUCellActualBuildLiveAndFixedProfiles(t *testing.T) {
	encoder := os.Getenv("KARTY_TEST_KTX")
	if encoder == "" {
		t.Skip("KARTY_TEST_KTX not selected")
	}

	t.Setenv("KARTY_KTX", encoder)

	for _, profile := range []string{"live", "fixed", "prebake", "prebake-whole"} {
		testGPUCellActualProfile(t, profile)
	}
}

func testGPUCellActualProfile(t *testing.T, profile string) {
	t.Helper()
	root, assets := gpuActualProfileFixture(t, profile)

	artifacts, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(profile, err)
	}

	envelope := decodePassiveEnvelope(t, artifacts[0].Bytes)
	catalogBytes, _, found := envelope.Read(worldgpuchunk.CatalogEntry, 0, level.MaxEntrySize)

	catalog, err := worldgpuchunk.DecodeCatalog(catalogBytes)
	if !found || err != nil || len(catalog.Cells) == 0 || !slices.Contains(artifacts[0].Features, worldgpuchunk.Feature) {
		t.Fatal("missing GPUcell root", profile, err)
	}

	if _, _, found := envelope.Read(worldlightmap.FixedImageEntryName, 0, level.MaxEntrySize); found {
		t.Fatal("root retained global RNM image")
	}

	if profile == "live" && len(catalog.BakedLights) != 0 || profile != "live" && !slices.Equal(catalog.BakedLights, []uint32{0, 1}) {
		t.Fatal("baked/live light partition changed", profile, catalog.BakedLights)
	}

	validateGPUProducerCells(t, artifacts[0], catalog, profile == "live")

	warm, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil || !bytes.Equal(artifacts[0].Bytes, warm[0].Bytes) {
		t.Fatal("warm GPUroot changed", profile, err)
	}

	assets.development = true

	development, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil || !bytes.Equal(artifacts[0].Bytes, development[0].Bytes) {
		t.Fatal("development lost complete cached GPUcells", profile, err)
	}

	if profile != "live" {
		if err := os.RemoveAll(filepath.Join(root, ".karty", "bakes", "lighting")); err != nil {
			t.Fatal(err)
		}

		if _, err := buildAll(t.Context(), root, 4, "", assets); !errors.Is(err, ErrManifest) {
			t.Fatal("development traced new rays instead of rejecting missing bake", profile, err)
		}
	}
}

func gpuActualProfileFixture(t *testing.T, profile string) (string, *assetBuild) {
	t.Helper()
	root := lightingLevelFixture(t, "version: 8\n"+chunkRoomsYAML+directLightmapYAML)

	switch profile {
	case "fixed":
		enableFixtureLightmap(t, root, fixedBakeSettings)
	case "prebake", "prebake-whole":
		enableFixtureLightmap(t, root, automaticBakeSettings)
	}

	if profile == "fixed" || profile == "prebake" {
		enableFixtureChunks(t, root, "max_sectors = 1\n[chunks.gpu]\ncell_size = 2\n")
	}

	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(
		selected.Assets.Capabilities.Runtime,
		asset.CapabilityWorldMaterialAtlasV1,
		worldmaterialgpu.Feature,
		worldgpuchunk.Feature,
		asset.Capability(worldchunk.ReadyFeature),
		asset.Capability(worldlightmap.FixedFeature),
	)
	assets := &assetBuild{
		projectRoot: root,
		manifest:    selected,
		materials: func(_ context.Context, _ string, _ sdk.Manifest, document world.Document, _ map[uint32][]byte, _ ...map[uint32]assetpipeline.MaterializeOptions) (worldmaterial.Pair, assetpipeline.Artifact, error) {
			return fixtureWorldMaterialPair(t, world.MaterialIDs(&document)), assetpipeline.Artifact{}, nil
		},
	}

	return root, assets
}

func validateGPUProducerCells(t *testing.T, artifact Artifact, catalog worldgpuchunk.Catalog, live bool) {
	t.Helper()

	for _, cell := range catalog.Cells {
		var module []byte

		for _, artifact := range artifact.Chunks {
			if artifact.ContentSHA256 == hex.EncodeToString(cell.ContentSHA256[:]) {
				module = artifact.Bytes

				break
			}
		}

		child := decodePassiveEnvelope(t, module)
		data, _, _ := child.Read(worldgpuchunk.PayloadEntry, 0, level.MaxEntrySize)

		payload, err := worldgpuchunk.DecodePayload(data)
		if err != nil {
			t.Fatal(err)
		}

		rnm, _, _ := child.Read(worldgpuchunk.RNMEntry, 0, level.MaxEntrySize)
		if err := catalog.ValidateCell(cell.ID, payload, rnm); err != nil {
			t.Fatal(err)
		}

		if live && (cell.RNMBytes != 12 || len(cell.PhysicalIDs) != 0) {
			t.Fatal("live cell gained global lighting/physics ownership")
		}

		if _, _, found := child.Read(worldchunk.ReadyEntryName, 0, level.MaxEntrySize); found {
			t.Fatal("GPUcell retained sourceReady")
		}

		if _, _, found := child.Read(worldlightmap.FixedImageEntryName, 0, level.MaxEntrySize); found {
			t.Fatal("GPUcell retained QOI lighting")
		}

		for _, name := range []string{worldlightmap.EntryName, worldlightmap.FixedEntryName,
			worldlightmap.PrebakeEntryName, worldlightmap.PrebakeImageEntryName} {
			if _, _, found := child.Read(name, 0, level.MaxEntrySize); found {
				t.Fatal("GPU render cell retained CPU lighting provenance", name)
			}
		}
	}
}
