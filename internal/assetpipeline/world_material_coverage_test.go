package assetpipeline

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/sdk"
)

func TestMaskedAtlasCacheRestoresCoverageAndSecondaryRoles(t *testing.T) {
	t.Parallel()

	document := fixtureMaterialWorld()
	document.MaterialLayers = &world.MaterialLayers{Version: 1}
	document.Sectors[0].Walls[0].FrameRegions = []world.WallFrameRegion{{Material: 11, Coverage: world.FrameCoverageMasked}}
	document.Sectors[0].FloorSecondary = &world.SurfaceSecondary{Material: 12, Strength: 1}
	textures := fixtureTextures(t)
	textures[11] = fixtureQOI(t, 2, 2, color.NRGBA{R: 180, G: 20, A: 96})
	textures[12] = fixtureQOI(t, 2, 2, color.NRGBA{R: 188, G: 188, B: 188, A: 255})
	manifest := fixtureMaterialManifest()
	manifest.Assets.Capabilities.Runtime = append(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV2)
	calls := 0
	execute := func(_ context.Context, _ sdk.Manifest, directory string, args []string) error {
		calls++

		encoded, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}

		input, err := png.Decode(bytes.NewReader(encoded))
		if err != nil {
			return err
		}

		for y := range input.Bounds().Dy() {
			for x := range input.Bounds().Dx() {
				_, _, _, a := input.At(x, y).RGBA()
				if a != 65535 {
					t.Fatal("Materialize received alpha")
				}
			}
		}

		return fixtureMaterialMaps(directory, input.Bounds())
	}
	root := t.TempDir()

	pair, cold, err := processWorldMaterials(context.Background(), root, manifest, document, textures, execute)
	if err != nil {
		t.Fatal(err)
	}

	_, warm, err := processWorldMaterials(context.Background(), root, manifest, document, textures, execute)
	if err != nil || calls != 1 || cold.CacheHit || !warm.CacheHit {
		t.Fatalf("cache calls=%d err=%v", calls, err)
	}

	_, albedo, err := qoi.Decode(pair.Albedo)
	if err != nil {
		t.Fatal(err)
	}

	_, data, err := qoi.Decode(pair.Data)
	if err != nil {
		t.Fatal(err)
	}

	for _, rect := range pair.Layout.Materials {
		if rect.MaterialID == 11 && albedo.NRGBAAt(rect.X, rect.Y).A != 96 {
			t.Fatal("coverage not restored")
		}

		if rect.MaterialID == 12 && data.NRGBAAt(rect.X, rect.Y) != (color.NRGBA{128, 128, 0, 255}) {
			t.Fatal("secondary data not neutral")
		}
	}

	if pair.Layout.Schema != worldmaterial.SchemaV2 {
		t.Fatal("coverage atlas is not versioned")
	}
}

func TestOpaqueFrameClaimBindsOpaqueAtlasSlot(t *testing.T) {
	t.Parallel()

	document := fixtureMaterialWorld()
	document.MaterialLayers = &world.MaterialLayers{Version: 1}
	document.Sectors[0].Walls[0].FrameRegions = []world.WallFrameRegion{
		{Material: 11, Coverage: world.FrameCoverageMasked},
		{Material: 12, Coverage: world.FrameCoverageMasked},
		{Material: 11, Coverage: world.FrameCoverageOpaque},
	}
	textures := fixtureTextures(t)
	textures[11] = fixtureQOI(t, 2, 2, color.NRGBA{R: 115, G: 70, B: 50, A: 255})
	textures[12] = fixtureQOI(t, 2, 2, color.NRGBA{R: 115, G: 70, B: 50, A: 96})
	manifest := fixtureMaterialManifest()
	manifest.Assets.Capabilities.Runtime = append(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldMaterialAtlasV2)
	calls := 0
	execute := func(_ context.Context, _ sdk.Manifest, directory string, _ []string) error {
		calls++

		layout, err := worldmaterial.NewLayoutV2(world.MaterialIDs(&document))
		if err != nil {
			return err
		}

		return fixtureMaterialMaps(directory, image.Rect(0, 0, layout.Width, layout.Height))
	}
	root := t.TempDir()

	pair, _, err := processWorldMaterials(t.Context(), root, manifest, document, textures, execute)
	if err != nil {
		t.Fatal(err)
	}

	for _, rect := range pair.Layout.Materials {
		if rect.MaterialID == 11 && rect.Coverage != worldmaterial.CoverageOpaque {
			t.Fatal("opaque frame claim packaged a masked atlas slot")
		}

		if rect.MaterialID == 12 && rect.Coverage != worldmaterial.CoverageMasked {
			t.Fatal("masked frame lost its coverage role")
		}
	}

	if err := pair.Validate(); err != nil {
		t.Fatal(err)
	}

	_, warm, err := processWorldMaterials(t.Context(), root, manifest, document, textures, execute)
	if err != nil || !warm.CacheHit || calls != 1 {
		t.Fatalf("opaque/masked atlas cache calls=%d err=%v", calls, err)
	}

	// A forged opaque claim must fail before native generation or cache reuse.
	textures[11] = textures[12]
	if _, _, err := processWorldMaterials(t.Context(), root, manifest, document, textures, execute); err == nil || calls != 1 {
		t.Fatalf("transparent pixels accepted as an opaque frame: calls=%d err=%v", calls, err)
	}
}

func TestMaskedBandCannotMakeSharedMainOrSecondaryTransparent(t *testing.T) {
	t.Parallel()

	for _, role := range []string{"main", "secondary", "solid"} {
		document := fixtureMaterialWorld()
		document.MaterialLayers = &world.MaterialLayers{Version: 1}
		document.Sectors[0].Walls[0].FrameRegions = []world.WallFrameRegion{{Material: 11, Coverage: world.FrameCoverageMasked}}

		switch role {
		case "main":
			document.Sectors[0].Walls[0].Material = 11
		case "secondary":
			document.Sectors[0].FloorSecondary = &world.SurfaceSecondary{Material: 11}
		case "solid":
			document.StaticSolids = &world.StaticSolids{Items: []world.Solid{{SideMaterial: 11}}}
		}

		if isMaskedFrameMaterial(document, 11) {
			t.Fatalf("%s texture was classified as masked", role)
		}

		layout, err := worldmaterial.NewLayoutV2(world.MaterialIDs(&document))
		if err != nil {
			t.Fatal(err)
		}

		textures := fixtureTextures(t)

		textures[11] = fixtureQOI(t, 2, 2, color.NRGBA{R: 100, A: 128})
		if _, err := packWorldAlbedo(t.Context(), layout, textures); err == nil {
			t.Fatalf("%s accepted transparent shared band texture", role)
		}
	}
}

func TestCoveredMaterialGenerationAndMipFiltering(t *testing.T) {
	t.Parallel()

	layout, err := worldmaterial.NewLayoutV2([]uint32{1})
	if err != nil {
		t.Fatal(err)
	}

	layout.Materials[0].Coverage = worldmaterial.CoverageMasked
	rect := layout.Materials[0]
	albedo := image.NewNRGBA(image.Rect(0, 0, layout.Width, layout.Height))
	// Uncovered blue must never bleed into covered red or generated height.
	for y := range rect.Height {
		for x := range rect.Width {
			albedo.SetNRGBA(rect.X+x, rect.Y+y, color.NRGBA{0, 0, 255, 0})
		}
	}

	albedo.SetNRGBA(rect.X, rect.Y, color.NRGBA{255, 0, 0, 255})

	input, err := materialGenerationInput(context.Background(), layout, albedo)
	if err != nil {
		t.Fatal(err)
	}

	for y := -rect.Gutter; y < rect.Height+rect.Gutter; y++ {
		for x := -rect.Gutter; x < rect.Width+rect.Gutter; x++ {
			if got := input.NRGBAAt(rect.X+x, rect.Y+y); got != (color.NRGBA{255, 0, 0, 255}) {
				t.Fatalf("generation(%d,%d)=%v", x, y, got)
			}
		}
	}

	if got := albedo.NRGBAAt(rect.X+1, rect.Y); got.A != 0 || got.B != 255 {
		t.Fatalf("authored alpha changed: %v", got)
	}

	colors := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	data := image.NewNRGBA(colors.Bounds())

	for y := range 2 {
		for x := range 2 {
			colors.SetNRGBA(x, y, color.NRGBA{0, 0, 255, 0})
			data.SetNRGBA(x, y, color.NRGBA{255, 0, 255, 0})
		}
	}

	colors.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	data.SetNRGBA(0, 0, color.NRGBA{128, 128, 32, 200})

	mipColor, mipData := downsampleMaterial(colors, data)
	if got := mipColor.NRGBAAt(0, 0); got != (color.NRGBA{255, 0, 0, 64}) {
		t.Fatalf("coverage color mip=%v", got)
	}

	if got := mipData.NRGBAAt(0, 0); got != (color.NRGBA{128, 128, 32, 200}) {
		t.Fatalf("coverage data mip=%v", got)
	}

	colors.SetNRGBA(0, 0, color.NRGBA{})

	mipColor, mipData = downsampleMaterial(colors, data)
	if mipColor.NRGBAAt(0, 0) != (color.NRGBA{}) || mipData.NRGBAAt(0, 0) != (color.NRGBA{128, 128, 0, 255}) {
		t.Fatal("empty mip must use neutral data")
	}
}
