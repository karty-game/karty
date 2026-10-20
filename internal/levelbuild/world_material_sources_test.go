package levelbuild

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

type atlasSourceCase struct {
	name, source            string
	atlas, chunks, ui, keep bool
}

func TestWorldAtlasOmitsOnlyMaterialSourcePayloads(t *testing.T) {
	t.Parallel()

	for _, test := range []atlasSourceCase{
		{name: "legacy", keep: true},
		{name: "atlas", atlas: true},
		{name: "chunked atlas", atlas: true, chunks: true},
		{name: "shared sprite", atlas: true, keep: true, source: `contents:
  - id: shared
    kind: actor
    position: {x: 1, y: 1, z: 1}
    actor:
      sprite: {texture: wall, facing: fixed, alpha: cutout, width: 1, height: 1}
`},
		{name: "shared particle", atlas: true, keep: true, source: levelParticleYAML},
		{name: "shared vegetation", atlas: true, keep: true, source: levelVegetationYAML},
		{name: "shared UI", atlas: true, ui: true, keep: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			checkAtlasSourcePackaging(t, test)
		})
	}
}

func checkAtlasSourcePackaging(t *testing.T, test atlasSourceCase) {
	t.Helper()

	rooms := chunkRoomsYAML
	if test.name == "shared vegetation" {
		rooms = strings.Replace(
			rooms,
			"    floor:",
			"    vegetation_layers: [{preset: grass, density: 4, seed: 42, surface: floor}]\n    floor:",
			1,
		)
	}

	root := lightingLevelFixture(t, "version: 20\n"+rooms+test.source)

	selected, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	selected = geometrySDK(selected)

	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime,
		asset.Capability(worldparticles.Feature), asset.Capability(worldvegetation.Feature))
	if test.atlas {
		selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV1)
	}

	if test.chunks {
		selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, "world/chunks@1")

		enableFixtureChunks(t, root, "max_sectors = 1\n")
	}

	if test.ui {
		addAtlasSourceUI(t, root)
	}

	var generated worldmaterial.Pair

	assets := &assetBuild{projectRoot: root, manifest: selected, materials: func(_ context.Context, _ string, _ sdk.Manifest,
		document world.Document, _ map[uint32][]byte, _ ...map[uint32]assetpipeline.MaterializeOptions) (worldmaterial.Pair, assetpipeline.Artifact, error) {
		generated = fixtureWorldMaterialPair(t, assetpipeline.WorldMaterialIDs(document))

		return generated, assetpipeline.Artifact{CacheKey: "fixture"}, nil
	}}

	first, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil {
		t.Fatal(err)
	}

	second, err := buildAll(t.Context(), root, 4, "", assets)
	if err != nil || !bytes.Equal(first[0].Bytes, second[0].Bytes) {
		t.Fatalf("non-deterministic packaging: %v", err)
	}

	envelope, err := level.DecodePassiveModule(first[0].Bytes)
	if err != nil {
		t.Fatal(err)
	}

	_, _, found := envelope.Read(level.TextureEntryName(1), 0, level.MaxEntrySize)
	if found != test.keep {
		t.Fatalf("standalone material texture present=%v, want %v", found, test.keep)
	}

	wantReports := 0
	if test.atlas {
		wantReports = 2
	}

	if test.keep {
		wantReports++
	}

	if len(first[0].Textures) != wantReports {
		t.Fatalf("reported %d textures, want %d packaged textures", len(first[0].Textures), wantReports)
	}

	if test.atlas {
		checkWorldMaterialModule(t, first[0].Bytes, generated)
	}
}

func addAtlasSourceUI(t *testing.T, root string) {
	t.Helper()

	directory := filepath.Join(root, "levels", "lighting")
	path := filepath.Join(directory, "level.toml")

	manifest, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	manifest = append(manifest, []byte("\n[theme]\nsource = \"theme.toml\"\n[[ui]]\nname = \"hud\"\nsource = \"hud.kui\"\n")...)
	for name, data := range map[string][]byte{
		"level.toml": manifest,
		"theme.toml": []byte("extends = \"dark\"\n[images.wall]\nasset = \"wall\"\nslice = [0, 0, 0, 0]\n"),
		"hud.kui":    []byte("<template><panel class=\"frame\"><label>HUD</label></panel></template>\n<style>\n.frame\n  background-image: theme.images.wall\n</style>"),
	} {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
