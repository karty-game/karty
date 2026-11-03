package assetpipeline

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/karty-game/karty/internal/ssbump"
)

func TestMaterialSSBumpCacheBindsNormalsAndIndependentStrengths(t *testing.T) {
	t.Parallel()

	height := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	normals := image.NewNRGBA(height.Bounds())

	for y := range 8 {
		for x := range 8 {
			height.SetNRGBA(x, y, color.NRGBA{A: 255})
			normals.SetNRGBA(x, y, color.NRGBA{R: 128, G: 128, A: 255})
		}
	}

	root := t.TempDir()
	input := worldSSBumpSurface{height: height, normals: normals, options: ssbump.Defaults(), normalStrength: 1, occlusionStrength: 1}

	base, first, err := cachedWorldMaterialSSBump(t.Context(), root, input)
	if err != nil || first.CacheHit {
		t.Fatal("cold material SSBump", err)
	}

	_, warm, err := cachedWorldMaterialSSBump(t.Context(), root, input)
	if err != nil || !warm.CacheHit || warm.CacheKey != first.CacheKey {
		t.Fatal("warm material SSBump", err)
	}

	normals.SetNRGBA(0, 0, color.NRGBA{R: 192, G: 128, A: 255})

	changed, normalKey, err := cachedWorldMaterialSSBump(t.Context(), root, input)
	if err != nil || normalKey.CacheHit || normalKey.CacheKey == first.CacheKey || bytes.Equal(changed.Pix, base.Pix) {
		t.Fatal("normal input did not affect the cache and RNM response", err)
	}

	input.normalStrength = 0

	disabled, strengthKey, err := cachedWorldMaterialSSBump(t.Context(), root, input)
	if err != nil || strengthKey.CacheKey == normalKey.CacheKey || !bytes.Equal(disabled.Pix, base.Pix) {
		t.Fatal("normal strength was ignored or applied twice", err)
	}

	input.occlusionStrength = 0

	_, aoKey, err := cachedWorldMaterialSSBump(t.Context(), root, input)
	if err != nil || aoKey.CacheKey == strengthKey.CacheKey {
		t.Fatal("occlusion strength absent from cache identity", err)
	}
}
