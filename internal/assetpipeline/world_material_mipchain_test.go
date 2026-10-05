package assetpipeline

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/worldmaterial"
)

func TestMaterialMipIndependentGammaAndVectorReferences(t *testing.T) {
	t.Parallel()

	albedo, data := image.NewNRGBA(image.Rect(0, 0, 2, 2)), image.NewNRGBA(image.Rect(0, 0, 2, 2))

	normals := []color.NRGBA{{R: 255, G: 128}, {R: 128, G: 255}, {R: 128, G: 128}, {R: 128, G: 128}}
	for sample, normal := range normals {
		column, row := sample%2, sample/2

		shade := uint8(0)
		if column == 1 {
			shade = 255
		}

		albedo.SetNRGBA(column, row, color.NRGBA{R: shade, G: shade, B: shade, A: 255})
		normal.B, normal.A = uint8(sample*80), uint8(sample*60)
		data.SetNRGBA(column, row, normal)
	}

	filteredAlbedo, filteredData := downsampleMaterial(albedo, data)
	// Independent IEC sRGB reference: the mean of black and white is linear .5.
	expectedSRGB := uint8(math.Round((1.055*math.Pow(0.5, 1/2.4) - 0.055) * 255))
	if got := filteredAlbedo.NRGBAAt(
		0,
		0,
	); got != (color.NRGBA{R: expectedSRGB, G: expectedSRGB, B: expectedSRGB, A: 255}) ||
		expectedSRGB != 188 {
		t.Fatalf("gamma mean: %v, reference %d", got, expectedSRGB)
	}

	// Unit input vectors are +X, +Y, +Z, +Z: normalize their sum (1,1,2).
	expectedNormal := uint8(math.Round(128 + 127/math.Sqrt(6)))
	if got := filteredData.NRGBAAt(
		0,
		0,
	); got != (color.NRGBA{R: expectedNormal, G: expectedNormal, B: 120, A: 90}) ||
		expectedNormal != 180 {
		t.Fatalf("3D normal/linear height-AO mean: %v", got)
	}

	for sample := range 4 {
		column, row := sample%2, sample/2

		red := uint8(1)
		if column == 0 {
			red = 255
		}

		data.SetNRGBA(column, row, color.NRGBA{R: red, G: 128, B: 73, A: 0})
	}

	_, filteredData = downsampleMaterial(albedo, data)
	if got := filteredData.NRGBAAt(0, 0); got != (color.NRGBA{R: 128, G: 128, B: 73, A: 0}) {
		t.Fatalf("opposing horizon normals/AO zero: %v", got)
	}

	for row := range 2 {
		for column := range 2 {
			data.SetNRGBA(column, row, color.NRGBA{R: 128, G: 128, A: 255})
		}
	}

	_, filteredData = downsampleMaterial(albedo, data)
	if got := filteredData.NRGBAAt(0, 0); got != (color.NRGBA{R: 128, G: 128, A: 255}) {
		t.Fatalf("neutral normal drift: %v", got)
	}
}

func TestMaterialMipChainIndependentInteriorsGuttersAndOnePixel(t *testing.T) {
	t.Parallel()

	layout, _ := worldmaterial.NewLayout([]uint32{9, 3})
	layout, _ = worldmaterial.NewMipLayout(layout)
	albedo, data := image.NewNRGBA(
		image.Rect(0, 0, layout.Width, layout.Height),
	), image.NewNRGBA(
		image.Rect(0, 0, layout.Width, layout.Height),
	)
	fillMaterialMipFixture(layout, albedo, data)

	tail, err := buildWorldMaterialMips(t.Context(), layout, albedo, data)
	if err != nil {
		t.Fatal(err)
	}

	checkMaterialMipInteriors(t, layout, tail)

	last, _ := layout.MipRect(8, 0, false)
	if last.Width != 1 || last.Height != 1 || last.Gutter != 1 {
		t.Fatal("complete chain lacks independently extruded one-pixel mip")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := buildWorldMaterialMips(ctx, layout, albedo, data); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled generation allocated/published: %v", err)
	}
}

func TestMaterialMipRecipeStrengthIndependenceAndRevision(t *testing.T) {
	t.Parallel()

	layout, _ := worldmaterial.NewLayout([]uint32{0})

	base, err := worldMaterialRecipe(fixtureMaterialManifest(), layout, nil, "linux-amd64", strings.Repeat("a", 64), nil)
	if err != nil {
		t.Fatal(err)
	}

	layout, _ = worldmaterial.NewMipLayout(layout)

	mips, err := worldMaterialRecipe(fixtureMaterialManifest(), layout, nil, "linux-amd64", strings.Repeat("a", 64), nil)
	if err != nil || base.Digest() == mips.Digest() {
		t.Fatal("full mip chain did not invalidate old L0 recipe")
	}

	layout.Materials[0].Strengths = &worldmaterial.Strengths{Normal: 4, Height: 0, AO: 0.75, Rim: 2}

	strengths, err := worldMaterialRecipe(fixtureMaterialManifest(), layout, nil, "linux-amd64", strings.Repeat("a", 64), nil)
	if err != nil || strengths.Digest() != mips.Digest() {
		t.Fatal("artistic strengths would rerun Materialize")
	}
}

func TestMaterialMipPaddedBudgetPrecedesSourceDecode(t *testing.T) {
	t.Parallel()

	ids := make([]uint32, worldmaterial.MaxMaterials)
	for index := range ids {
		ids[index] = uint32(index)
	}

	layout, _ := worldmaterial.NewLayout(ids)
	layout, _ = worldmaterial.NewMipLayout(layout)
	large := fixtureQOI(t, 1, 1, color.NRGBA{A: 255})
	binary.BigEndian.PutUint32(large[4:8], 4096)
	binary.BigEndian.PutUint32(large[8:12], 4096)
	// Two 64MiB source headers plus the 128MiB pair fit exactly before the
	// padded tail is charged. Their streams intentionally cannot decode.
	if _, err := packWorldAlbedo(
		t.Context(),
		layout,
		map[uint32][]byte{1: large, 2: bytes.Clone(large)},
	); !errors.Is(
		err,
		ErrAssetResourceLimits,
	) {
		t.Fatalf("tail budget was not preflighted before source decode: %v", err)
	}
}

func fillMaterialMipFixture(layout worldmaterial.Layout, albedo, data *image.NRGBA) {
	for row := range layout.Height {
		for column := range layout.Width {
			albedo.SetNRGBA(column, row, color.NRGBA{R: 255, A: 255})
			data.SetNRGBA(column, row, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}

	for slot, rect := range layout.Materials {
		for row := range rect.Height {
			for column := range rect.Width {
				pixel := color.NRGBA{G: 255, A: 255}

				if slot == 0 {
					shade := uint8(((column + row) % 2) * 255)
					pixel = color.NRGBA{R: shade, G: shade, B: shade, A: 255}
				}

				albedo.SetNRGBA(rect.X+column, rect.Y+row, pixel)
				data.SetNRGBA(rect.X+column, rect.Y+row, color.NRGBA{R: 128, G: 128, B: uint8(slot * 255), A: uint8(slot * 255)})
			}
		}
	}
}

func checkMaterialMipInteriors(t *testing.T, layout worldmaterial.Layout, tail *image.NRGBA) {
	t.Helper()

	for level := 1; level <= worldmaterial.MipLevels; level++ {
		for slot := range layout.Materials {
			for _, raw := range []bool{false, true} {
				rect, err := layout.MipRect(level, slot, raw)
				if err != nil {
					t.Fatal(err)
				}

				want := color.NRGBA{G: 255, A: 255}
				if slot == 0 {
					want = color.NRGBA{R: 188, G: 188, B: 188, A: 255}
				}

				if raw {
					want = color.NRGBA{R: 128, G: 128, B: uint8(slot * 255), A: uint8(slot * 255)}
				}

				checkMaterialMipRectangle(t, tail, rect, want)
			}
		}
	}
}

func checkMaterialMipRectangle(t *testing.T, tail *image.NRGBA, rect worldmaterial.Rect, want color.NRGBA) {
	t.Helper()

	for row := rect.Y - rect.Gutter; row < rect.Y+rect.Height+rect.Gutter; row++ {
		for column := rect.X - rect.Gutter; column < rect.X+rect.Width+rect.Gutter; column++ {
			if got := tail.NRGBAAt(column, row); got != want {
				t.Fatalf(
					"material%d size%d (%d,%d) cross-material/gutter bleed: %v want%v",
					rect.MaterialID,
					rect.Width,
					column,
					row,
					got,
					want,
				)
			}
		}
	}
}
