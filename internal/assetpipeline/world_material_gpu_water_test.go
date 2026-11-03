package assetpipeline

import (
	"bytes"
	"math"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldmaterial"
)

func TestGPUWaterNormalsPreserveLegacyFieldBeforeSemanticMips(t *testing.T) {
	t.Parallel()
	base, tiles := gpuMaterialFixture(t)

	original, err := PrepareWorldMaterialGPU(t.Context(), base, tiles)
	if err != nil {
		t.Fatal(err)
	}

	water, err := PrepareWorldMaterialGPU(t.Context(), base, tiles, map[uint32]float64{7: .065})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(original[0].Levels[0], water[0].Levels[0]) || !bytes.Equal(original[1].Levels[0], water[1].Levels[0]) {
		t.Fatal("water normal changed albedo/SSBump")
	}

	for _, point := range []struct {
		x, y       int
		red, green byte
	}{{0, 0, 132, 137}, {3, 3, 132, 137}, {64, 0, 137, 123}, {128, 0, 123, 137}, {192, 0, 118, 113}, {0, 64, 128, 123}} {
		offset := (point.y*worldmaterial.TileSize + point.x) * 4

		pixel := water[2].Levels[0][offset : offset+4]
		if !bytes.Equal(pixel, []byte{point.red, point.green, 42, 173}) {
			t.Fatal("legacy64px procedural field or reflectedchannels changed", point, pixel)
		}
	}

	for level := range water[2].Levels {
		size := max(worldmaterial.TileSize>>level, 1)

		layerBytes := size * size * 4
		if !bytes.Equal(original[2].Levels[level][layerBytes:], water[2].Levels[level][layerBytes:]) {
			t.Fatal("water override touched another material", level)
		}

		for offset := 0; offset < layerBytes; offset += 4 {
			if water[2].Levels[level][offset+2] != 42 || water[2].Levels[level][offset+3] != 173 {
				t.Fatal("water mips lost metallic/smoothness", level)
			}
		}
	}
}

func TestGPUWaterNormalOverridesRejectInvalidProfiles(t *testing.T) {
	t.Parallel()

	base, tiles := gpuMaterialFixture(t)
	for _, overrides := range []map[uint32]float64{{7: math.NaN()}, {7: math.Inf(1)}, {7: -1}, {7: 2}, {999: .1}} {
		if _, err := PrepareWorldMaterialGPU(t.Context(), base, tiles, overrides); err == nil {
			t.Fatal("invalid water override accepted")
		}
	}
}
