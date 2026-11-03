package levelbuild

import (
	"errors"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
)

func TestSpatialCombinedGeometryBudgetDoesNotWidenLegacyPolicy(t *testing.T) {
	t.Parallel()

	definition := manifest{Chunks: &chunkBuildSettings{
		Enabled:      true,
		MaxSpan:      new(float64(256)),
		MaxMeshBytes: new(uint64(maxSpatialGeometryBytes)),
		Spatial:      spatialTestSettings(),
	}, Lightmap: &lightmapBuildSettings{Enabled: true, Fixed: true, Offline: true, Density: new(float64(4))}}

	options, err := chunkOptions(definition)
	if err != nil || options.MaxMeshBytes != maxSpatialGeometryBytes {
		t.Fatal("separate spatial geometry resources could not share their combined decoded budget", err)
	}

	definition.Chunks.MaxMeshBytes = new(uint64(maxSpatialGeometryBytes + 1))
	if _, err := chunkOptions(definition); !errors.Is(err, ErrManifest) {
		t.Fatal("spatial combined geometry exceeded its bounded budget", err)
	}

	definition.Chunks.MaxMeshBytes = new(uint64(maxSpatialGeometryBytes))

	definition.Chunks.Spatial = nil
	if _, err := chunkOptions(definition); !errors.Is(err, ErrManifest) {
		t.Fatal("spatial combined budget widened legacy chunk policy", err)
	}
}

func TestSpatialAnimatedWaterRequiresSharedEdgesBeforeBaking(t *testing.T) {
	t.Parallel()

	document, _, _ := incrementalBakeFixture(t)
	document.Version = world.Version
	material := document.Sectors[0].FloorMaterial
	document.StaticSolids = &world.StaticSolids{Version: world.StaticSolidsVersion, Items: []world.Solid{{ID: "ocean",
		Footprint: []world.Vec2{{X: .25, Y: .25}, {X: .75, Y: .25}, {X: .75, Y: .75}, {X: .25, Y: .75}}, Top: world.Plane{C: .25},
		TopMaterial: material, SideMaterial: material, BottomMaterial: material}}}
	document.Animations = &world.Animations{
		Version:   world.AnimationVersion,
		Presets:   []world.AnimationPreset{{Name: "waves", Kind: "liquid", SurfaceAmplitude: .1, Frequency: 1}},
		Materials: []world.MaterialAnimation{{Material: material, Preset: "waves"}},
	}
	definition := manifest{Chunks: &chunkBuildSettings{Enabled: true}}

	definition.Chunks.Spatial = spatialTestSettings()
	if _, err := prepareSpatialWorldChunks(definition, &document, nil); !errors.Is(err, ErrManifest) {
		t.Fatal("animated water reached ray tracing without common boundary geometry", err)
	}

	definition.Chunks.Spatial.LiquidEdgeStep = 2
	if _, err := prepareSpatialWorldChunks(definition, &document, nil); err != nil {
		t.Fatal("valid common water boundary grid rejected", err)
	}
}
