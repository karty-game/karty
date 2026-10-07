package levelbuild

import (
	"path/filepath"
	"testing"

	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/testfixture"
)

func TestRomanStaticSolidSamplePackagedWASM(t *testing.T) {
	t.Parallel()

	selected, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	selected = geometrySDK(selected)
	root := testfixture.WorldCameraGeometry(t, filepath.Join("..", "..", "samples", "world-camera"))

	artifacts, err := BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("Roman package: %v", err)
	}

	document := executePackagedLightingWorld(t, artifacts[0].Bytes)
	portals, courtSectors, courtPortals := 0, 0, 0

	for _, sector := range document.Sectors {
		if sector.Instance == "roman-court" {
			courtSectors++
		}

		for _, wall := range sector.Walls {
			if wall.Portal >= 0 {
				portals++

				if sector.Instance == "roman-court" {
					courtPortals++
				}
			}
		}
	}

	if len(document.Sectors) != 69 || portals != 236 || document.StaticSolids == nil ||
		len(document.StaticSolids.Items) != 8 || len(document.Contents) != 12 {
		t.Fatalf("actual WASM lost migrated graph or solids: sectors=%d portals=%d", len(document.Sectors), portals)
	}

	if courtSectors != 61 || courtPortals != 219 {
		t.Fatalf("Roman court graph: sectors=%d directed portals=%d", courtSectors, courtPortals)
	}

	t.Logf("whole sample: %d sectors, %d directed portals; roman-court: %d sectors, %d outgoing directed portals",
		len(document.Sectors), portals, courtSectors, courtPortals)

	for _, solid := range document.StaticSolids.Items {
		if !solid.Collision || len(solid.Footprint) != 10 || solid.Top.C != 12 {
			t.Fatal("actual WASM lost shaft volume")
		}
	}
}
