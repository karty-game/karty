package levelbuild

import (
	"bytes"
	"testing"
)

func TestGPUCellLightingCompactsDistinctUniformLobes(t *testing.T) {
	t.Parallel()

	colors := [][4]byte{{12, 24, 36, 48}, {8, 16, 32, 64}, {17, 31, 63, 255}}

	pixels := make([]byte, 0, 48)

	for _, color := range colors {
		for range 6 {
			pixels = append(pixels, color[:]...)
		}
	}

	original := gpuCellLighting{width: 2, height: 3, rangeValue: 8, pixels: pixels}

	compact := compactGPUCellLighting(original)
	if compact.width != 1 || compact.height != 1 || len(compact.pixels) != 12 || compact.rangeValue != 8 {
		t.Fatal("uniform lighting was not packed", compact)
	}

	for lobe, color := range colors {
		if !bytes.Equal(compact.pixels[lobe*4:(lobe+1)*4], color[:]) {
			t.Fatal("directional lobes were mixed", lobe)
		}
	}

	if original.width != 2 || original.height != 3 || len(original.pixels) != 72 {
		t.Fatal("source lighting was mutated")
	}
}

func TestGPUCellLightingCompactsCoveredBlackAndEmptyGutters(t *testing.T) {
	t.Parallel()

	pixels := make([]byte, 0, 48)
	for range 3 {
		pixels = append(pixels, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 255, 255, 64, 32, 0)
	}

	compact := compactGPUCellLighting(gpuCellLighting{width: 2, height: 2, pixels: pixels})
	if compact.width != 1 || compact.height != 1 || len(compact.pixels) != 12 {
		t.Fatal("zero-radiance atlas was expanded", compact)
	}

	for lobe := range 3 {
		if !bytes.Equal(compact.pixels[lobe*4:(lobe+1)*4], []byte{0, 0, 0, 1}) {
			t.Fatal("black encoded incorrectly")
		}
	}
}

func TestGPUCellLightingRetainsNonuniformAndUncoveredConstantCharts(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		pixel  [4]byte
		offset int
	}{
		{"one differing radiance texel in last lobe", [4]byte{4, 8, 16, 64}, 44},
		{"uncovered gutter beside nonzero constant", [4]byte{}, 4},
		{"different nonzero RGBM multiplier", [4]byte{4, 8, 16, 32}, 12},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			pixels := bytes.Repeat([]byte{4, 8, 16, 255}, 12)
			copy(pixels[test.offset:], test.pixel[:])
			original := gpuCellLighting{width: 2, height: 2, pixels: pixels}

			result := compactGPUCellLighting(original)
			if result.width != 2 || result.height != 2 || len(result.pixels) != len(pixels) || &result.pixels[0] != &pixels[0] {
				t.Fatal("nonuniform sampled lighting was changed")
			}
		})
	}
}

func TestGPUCellLightingCompactionLeavesInvalidInputForEncoderAdmission(t *testing.T) {
	t.Parallel()

	for _, lighting := range []gpuCellLighting{{}, {width: 1, height: 2, pixels: []byte{0, 0, 0, 1}}, {width: 2049, height: 1, pixels: make([]byte, 2049*12)}} {
		result := compactGPUCellLighting(lighting)
		if result.width != lighting.width || result.height != lighting.height || !bytes.Equal(result.pixels, lighting.pixels) {
			t.Fatal("invalid lighting was silently repaired")
		}
	}
}
