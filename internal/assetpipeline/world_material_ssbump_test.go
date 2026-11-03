package assetpipeline

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/ssbump"
)

func TestWorldSSBumpCacheBindsHeightAndControls(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	height := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for o := 3; o < len(height.Pix); o += 4 {
		height.Pix[o] = 255
	}

	options := ssbump.Defaults()

	a, first, err := cachedWorldSSBump(t.Context(), root, height, options)
	if err != nil {
		t.Fatal(err)
	}

	if a.NRGBAAt(0, 0) != (color.NRGBA{85, 85, 85, 255}) || first.CacheHit {
		t.Fatal("cold flat output", first)
	}

	_, warm, err := cachedWorldSSBump(t.Context(), root, height, options)
	if err != nil || !warm.CacheHit || warm.CacheKey != first.CacheKey {
		t.Fatal("warm cache", err)
	}

	height.SetNRGBA(1, 0, color.NRGBA{255, 255, 255, 255})

	_, changed, err := cachedWorldSSBump(t.Context(), root, height, options)
	if err != nil || changed.CacheKey == first.CacheKey {
		t.Fatal("height cache identity", err)
	}

	options.Depth = 0

	_, disabled, err := cachedWorldSSBump(t.Context(), root, height, options)
	if err != nil || disabled.CacheKey == changed.CacheKey {
		t.Fatal("depth cache identity", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if p, _, err := cachedWorldSSBump(ctx, root, height, options); err == nil || p != nil {
		t.Fatal("canceled cache generation published")
	}
}

func TestWorldSSBumpPackingPreservesLinearEnergyAndIsolation(t *testing.T) {
	t.Parallel()

	layout, err := worldmaterial.NewLayout([]uint32{7, 9})
	if err != nil {
		t.Fatal(err)
	}

	layout, err = worldmaterial.NewMipLayout(layout)
	if err != nil {
		t.Fatal(err)
	}

	first, second := flatSSBump(256, 256), flatSSBump(256, 256)

	for y := range 256 {
		for x := range 256 {
			first.SetNRGBA(x, y, color.NRGBA{0, 40, 80, 255})
			second.SetNRGBA(x, y, color.NRGBA{200, 0, 0, 255})
		}
	}

	p, err := PackWorldMaterialSSBump(t.Context(), layout, map[uint32]*image.NRGBA{7: first, 9: second})
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := p.Decode(layout)
	if err != nil {
		t.Fatal(err)
	}

	for slot, want := range []color.NRGBA{{0, 40, 80, 255}, {200, 0, 0, 255}} {
		r := layout.Materials[slot]
		if decoded.Atlas.NRGBAAt(r.X-r.Gutter, r.Y) != want {
			t.Fatal("neighbor leaked into gutter")
		}

		for level := 1; level <= 8; level++ {
			m, _ := layout.MipRect(level, slot, true)
			if decoded.MipTail.NRGBAAt(m.X, m.Y) != want {
				t.Fatal("linear energy normalized", level)
			}
		}
	}
	// Opposing coefficient samples average arithmetically, not in sRGB or as a normal.
	checker := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	checker.SetNRGBA(0, 0, color.NRGBA{100, 0, 0, 255})
	checker.SetNRGBA(1, 0, color.NRGBA{0, 100, 0, 255})
	checker.SetNRGBA(0, 1, color.NRGBA{100, 0, 0, 255})
	checker.SetNRGBA(1, 1, color.NRGBA{0, 100, 0, 255})

	if got := downsampleSSBump(checker).NRGBAAt(0, 0); got != (color.NRGBA{50, 50, 0, 255}) {
		t.Fatal(got)
	}

	if _, _, err := GenerateWorldMaterialSSBump(t.Context(), t.TempDir(), sdk.Manifest{}, worldmaterial.Pair{}, []uint32{7}); err == nil {
		t.Fatal("missing capability accepted")
	}
}
