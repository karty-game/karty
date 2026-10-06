package levelbuild

import (
	"bytes"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/sdk"
)

func TestMaterialMappingCapabilityGateAndPackagedWASM(t *testing.T) {
	t.Parallel()

	selected, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	if selected.Version != "0.0.7" || slices.Contains(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialMappingV1) {
		t.Fatal("released SDK gained optional material mapping")
	}

	root := lightingLevelFixture(t, "version: 5\n"+lightingRoomYAML)

	if _, err := BuildAllWithAssets(t.Context(), root, 4, "", selected); !errors.Is(err, ErrManifest) {
		t.Fatalf("released SDK accepted source v5 mapping: %v", err)
	}
	// Projection controls work independently of atlas generation and lighting.
	selected.Version = "0.0.8"
	selected.Assets.Capabilities.Runtime = append(selected.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialMappingV1)

	first, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || len(first) != 1 {
		t.Fatalf("candidate mapping-only build: %v", err)
	}

	second, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || len(second) != 1 || !bytes.Equal(first[0].Bytes, second[0].Bytes) ||
		!slices.Equal(first[0].Features, []string{cartridge.FeatureTextureQOIv1, world.FeatureMaterialMapping, world.Feature}) {
		t.Fatalf("mapping feature/deterministic packaging changed: %+v %v", first, err)
	}

	document := executePackagedLightingWorld(t, first[0].Bytes)
	if document.Version != 3 || document.MaterialMapping == nil || document.MaterialMapping.Version != 1 || document.Lighting != nil {
		t.Fatalf("mapping marker or independent lighting contract lost: %+v", document)
	}

	for _, sector := range document.Sectors {
		if world.ValidateSurfaceUV(sector.FloorUV) != nil || world.ValidateSurfaceUV(sector.CeilingUV) != nil {
			t.Fatal("packaged WASM omitted complete horizontal mapping")
		}

		for _, wall := range sector.Walls {
			if world.ValidateSurfaceUV(wall.UV) != nil || len(wall.UV.Projections) != 3 {
				t.Fatal("packaged WASM omitted complete default triplanar wall mapping")
			}
		}
	}
}

func TestMaterialMappingGateAbsentRemainsLegacy(t *testing.T) {
	t.Parallel()

	selected, err := sdk.Resolve("0.0.7")
	if err != nil {
		t.Fatal(err)
	}

	for version := 1; version <= 4; version++ {
		root := lightingLevelFixture(t, "version: "+strconv.Itoa(version)+"\n"+lightingRoomYAML)

		legacy, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
		if err != nil || len(legacy) != 1 || slices.Contains(legacy[0].Features, world.FeatureMaterialMapping) {
			t.Fatalf("legacy source v%d packaging changed: %v", version, err)
		}

		document := executePackagedLightingWorld(t, legacy[0].Bytes)
		if document.MaterialMapping != nil || document.Sectors[0].FloorUV != nil || document.Sectors[0].Walls[0].UV != nil {
			t.Fatal("legacy source gained mapping records")
		}
	}
}
