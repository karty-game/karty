package levelbuild

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/actions"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

func TestBuiltKLDResourcesUseTypedBinaryContracts(t *testing.T) {
	t.Parallel()
	root := lightingLevelFixture(t, "version: 4\n"+chunkRoomsYAML+directLightmapYAML)
	selected := directLightmapSDK(t)
	selected.Assets.Capabilities.Runtime = append(
		selected.Assets.Capabilities.Runtime,
		asset.Capability(worldchunk.Feature),
		asset.CapabilityWorldMaterialAtlasV1,
	)

	enableFixtureLightmap(
		t,
		root,
		"enabled = true\noffline = true\nlights = ['red','blue']\npage_size = 512\ndensity = 1\nbake_samples = 1\nbake_bounces = 0\nbake_denoise = 'off'\n",
	)
	enableFixtureChunks(t, root, "max_sectors = 1\nmax_lightmap_texels = 262144\n")

	if err := os.WriteFile(
		filepath.Join(root, "levels", "lighting", "actions.yaml"),
		[]byte("version: 1\nsequences:\n  - name: hello\n    steps:\n      - waitFrames: 1\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	assets := &assetBuild{
		projectRoot: root,
		manifest:    selected,
		materials: func(_ context.Context, _ string, _ sdk.Manifest, document world.Document, _ map[uint32][]byte, _ ...map[uint32]assetpipeline.MaterializeOptions) (worldmaterial.Pair, assetpipeline.Artifact, error) {
			return fixtureWorldMaterialPair(t, assetpipeline.WorldMaterialIDs(document)), assetpipeline.Artifact{}, nil
		},
	}

	artifacts, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(err)
	}

	envelope := executeLightmapEnvelope(t, artifacts[0].Bytes)
	if !bytes.HasPrefix(envelope.Metadata, []byte("KTYM")) {
		t.Fatal("metadata bypassed binary codec")
	}

	if _, err := level.DecodeMetadata(envelope.Metadata); err != nil {
		t.Fatal(err)
	}

	worldBytes := binaryEntry(t, envelope, world.EntryName, "KWR1")

	document, err := world.Decode(worldBytes)
	if err != nil {
		t.Fatal(err)
	}

	material := binaryEntry(t, envelope, worldmaterial.LayoutEntry, "KMA1")
	if _, err := worldmaterial.DecodeLayout(material); err != nil {
		t.Fatal(err)
	}

	compiledActions := binaryEntry(t, envelope, "karty/actions@1", "KTYA")
	if _, err := actions.Decode(compiledActions); err != nil {
		t.Fatal(err)
	}

	catalogBytes, _, found := envelope.Read(worldchunk.CatalogEntryName, 0, level.MaxEntrySize)
	if !found {
		t.Fatal("missing chunk catalog")
	}

	if _, err := worldchunk.DecodeCatalog(catalogBytes, &document); err != nil {
		t.Fatal(err)
	}

	for _, artifact := range artifacts[0].Chunks {
		child := executeLightmapEnvelope(t, artifact.Bytes)
		if !bytes.HasPrefix(child.Metadata, []byte("KTYM")) {
			t.Fatal("child metadata bypassed binary codec")
		}

		layoutBytes := binaryEntry(t, child, worldlightmap.EntryName, "KLM1")

		layout, err := worldlightmap.Decode(layoutBytes, &document)
		if err != nil {
			t.Fatal(err)
		}

		manifest := binaryEntry(t, child, worldlightmap.PrebakeEntryName, "KPB1")

		image, _, found := child.Read(worldlightmap.PrebakeImageEntryName, 0, level.MaxEntrySize)
		if !found {
			t.Fatal("missing completed image")
		}

		if _, err := worldlightmap.DecodePrebake(manifest, image, layout, &document); err != nil {
			t.Fatal(err)
		}

		mesh, _, found := child.Read(worldchunk.MeshEntryName, 0, level.MaxEntrySize)
		if !found {
			t.Fatal("missing prebuilt mesh")
		}

		if _, err := worldchunk.DecodeMesh(mesh, &document); err != nil {
			t.Fatal(err)
		}
	}
}
func binaryEntry(tb testing.TB, envelope level.Envelope, name, magic string) []byte {
	tb.Helper()

	data, _, found := envelope.Read(name, 0, level.MaxEntrySize)
	if !found || !bytes.HasPrefix(data, []byte(magic)) {
		tb.Fatalf("%s bypassed binary codec", name)
	}

	return data
}
