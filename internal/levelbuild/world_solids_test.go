package levelbuild

import (
	"bytes"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

const levelSolidYAML = `solids:
  - id: pillar
    footprint: [{x: 1, y: 1}, {x: 2, y: 1}, {x: 2, y: 2}, {x: 1, y: 2}]
    bottom: {c: 0}
    top: {c: 3}
    side_material: wall
    top_material: wall
    bottom_material: wall
    collision: true
`

func TestStaticSolidsCapabilityPackagingAndWASM(t *testing.T) {
	t.Parallel()

	selected, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	selected = geometrySDK(selected)

	selected = withoutRuntimeCapabilities(selected, asset.CapabilityWorldStaticSolidsV1)

	root := lightingLevelFixture(t, "version: 6\n"+lightingRoomYAML+levelSolidYAML)

	if _, err := BuildAllWithAssets(t.Context(), root, 4, "", selected); !errors.Is(err, ErrManifest) {
		t.Fatalf("undeclared solids accepted: %v", err)
	}

	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldStaticSolidsV1)

	first, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || len(first) != 1 {
		t.Fatalf("solids build: %v", err)
	}

	second, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || !bytes.Equal(first[0].Bytes, second[0].Bytes) || !slices.Contains(first[0].Features, world.FeatureStaticSolids) {
		t.Fatalf("noncanonical build: %v", err)
	}

	document := executePackagedLightingWorld(t, first[0].Bytes)
	if document.StaticSolids == nil || len(document.StaticSolids.Items) != 1 || !document.StaticSolids.Items[0].Collision ||
		document.StaticSolids.Items[0].TopUV == nil {
		t.Fatalf("WASM lost solid payload: %+v", document)
	}
}

func TestSolidMaterialsStableAtlasEnumeration(t *testing.T) {
	t.Parallel()

	document := world.Document{
		Sectors: []world.Sector{{FloorMaterial: 3, CeilingMaterial: 1, Walls: []world.Wall{{Material: 2}}}},
		StaticSolids: &world.StaticSolids{
			Version: 1,
			Items: []world.Solid{
				{SideMaterial: 4, TopMaterial: 3, BottomMaterial: 5},
				{SideMaterial: 5, TopMaterial: 6, BottomMaterial: 4},
			},
		},
	}
	if ids := assetpipeline.WorldMaterialIDs(document); !reflect.DeepEqual(ids, []uint32{3, 1, 2, 4, 5, 6}) {
		t.Fatalf("atlas solid order changed: %v", ids)
	}
}
