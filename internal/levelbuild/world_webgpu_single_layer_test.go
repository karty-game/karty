package levelbuild

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

func TestWebGPUSingleLayerPreservesPrimaryTilesAndRepeatingUVs(t *testing.T) {
	t.Parallel()

	primary := &world.SurfaceUV{
		Projections: []world.UVProjection{{U: world.UVPlane{X: 1.0 / 2.4}, V: world.UVPlane{Y: 1.0 / 1.2}}},
		Weights:     []float64{1},
	}
	secondary := &world.SurfaceSecondary{Material: 2, Strength: .38, UV: webGPUChartTestMapping()}
	source := world.Sector{FloorMaterial: 1, CeilingMaterial: 1, FloorUV: primary, CeilingUV: primary,
		FloorSecondary: secondary, CeilingSecondary: secondary, Walls: []world.Wall{{Material: 1, UV: primary, Secondary: secondary,
			Start: world.Vec2{X: 5.5, Y: -.5}, End: world.Vec2{X: 18.5, Y: -.5}, Portal: 2, PortalWall: 3}}}
	document := world.Document{Sectors: []world.Sector{source}, MaterialLayers: &world.MaterialLayers{Version: 1}}
	// Production compilation owns its sectors/walls. Retain independent source
	// containers here to assert that the authoring-side layers remain available.
	document.Sectors[0].Walls = append([]world.Wall(nil), source.Walls...)
	prepareWebGPUSingleLayer(&document)

	converted := document.Sectors[0]
	if converted.FloorSecondary != nil || converted.CeilingSecondary != nil || converted.Walls[0].Secondary != nil ||
		document.MaterialLayers != nil {
		t.Fatal("V1 retained secondary material modulation")
	}

	if source.FloorSecondary != secondary || source.Walls[0].Secondary != secondary || secondary.Strength != .38 {
		t.Fatal("WebGPU policy mutated the authored secondary layer")
	}

	if !reflect.DeepEqual(converted.FloorUV, primary) || !reflect.DeepEqual(converted.Walls[0].UV, primary) ||
		converted.FloorMaterial != source.FloorMaterial || converted.Walls[0].Material != source.Walls[0].Material ||
		converted.Walls[0].Start != source.Walls[0].Start || converted.Walls[0].End != source.Walls[0].End ||
		converted.Walls[0].Portal != 2 || converted.Walls[0].PortalWall != 3 {
		t.Fatal("single-layer policy changed primary projection, material or portal geometry")
	}

	if needsWebGPUCharts(&document) {
		t.Fatal("ordinary primary tile was replaced by a whole-surface chart")
	}

	checkSingleLayerTileBytes(t, document)
}

func checkSingleLayerTileBytes(t *testing.T, document world.Document) {
	t.Helper()

	pair := fixtureWorldMaterialPair(t, []uint32{1})
	calls := 0
	assets := &assetBuild{materials: func(context.Context, string, sdk.Manifest, world.Document, map[uint32][]byte,
		...map[uint32]assetpipeline.MaterializeOptions,
	) (worldmaterial.Pair, assetpipeline.Artifact, error) {
		calls++

		return pair, assetpipeline.Artifact{CacheKey: "ordinary-tile", CacheHit: true}, nil
	}}
	assets.manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityWorldMaterialAtlasV1}

	metadata, err := level.EncodeMetadata(level.Metadata{})
	if err != nil {
		t.Fatal(err)
	}

	owned, _, metadata, err := prepareWebGPUCharts(t.Context(), assets, &document, nil, metadata, nil, nil)
	if err != nil || owned != assets || calls != 0 {
		t.Fatal("secondary omission unnecessarily baked/resampled ordinary primary material", err, calls)
	}

	result, err := buildWorldMaterials(t.Context(), owned, document, nil, nil, metadata, nil)
	if err != nil {
		t.Fatal(err)
	}

	if calls != 1 {
		t.Fatal("ordinary material generation repeated", calls)
	}

	matched := 0

	for _, entry := range result.data {
		switch entry.Name {
		case worldmaterial.AlbedoEntry:
			matched++

			if !bytes.Equal(entry.Data, pair.Albedo) {
				t.Fatal("primary albedo pixels were resampled")
			}
		case worldmaterial.DataEntry:
			matched++

			if !bytes.Equal(entry.Data, pair.Data) {
				t.Fatal("primary normal/height/AO bytes changed")
			}
		}
	}

	if matched != 2 {
		t.Fatal("ordinary material pair was not completely packaged", matched)
	}
}

func TestWebGPUChartBakerRejectsSecondaryLayersAtOwnedBoundary(t *testing.T) {
	t.Parallel()

	for _, surface := range []string{"floor", "ceiling", "wall"} {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()

			layer := &world.SurfaceSecondary{Material: 2, Strength: .5, UV: webGPUChartTestMapping()}
			document := world.Document{Sectors: []world.Sector{{Walls: []world.Wall{{}}}}}

			switch surface {
			case "floor":
				document.Sectors[0].FloorSecondary = layer
			case "ceiling":
				document.Sectors[0].CeilingSecondary = layer
			case "wall":
				document.Sectors[0].Walls[0].Secondary = layer
			}

			if _, err := newWebGPUChartBaker(&document, worldmaterial.Pair{}, 4); !errors.Is(err, ErrManifest) {
				t.Fatal("private chart baker accepted a secondary layer", err)
			}

			if _, _, _, err := prepareWebGPUCharts(
				t.Context(),
				&assetBuild{},
				&document,
				nil,
				nil,
				nil,
				nil,
			); !errors.Is(
				err,
				ErrManifest,
			) {
				t.Fatal("private chart preparation accepted a secondary layer", err)
			}

			baker := &webGPUChartBaker{document: &document}
			if err := baker.convert(t.Context()); !errors.Is(err, ErrManifest) {
				t.Fatal("mutable chart conversion bypassed single-layer validation", err)
			}
		})
	}
}
