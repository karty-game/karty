package levelbuild

import (
	"image"
	"image/color"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldvegetation"
)

func TestFixedVegetationBakeUsesProcessedCutoutAndIgnoresNormalSources(t *testing.T) {
	t.Parallel()

	document := worldvegetation.Document{
		Version: worldvegetation.NormalVersion,
		Presets: []worldvegetation.Preset{
			{
				Name:          "grass",
				Texture:       17,
				NormalTexture: 18,
				Width:         worldvegetation.Range{Min: .2, Max: .4},
				Height:        worldvegetation.Range{Min: .3, Max: .8},
				DrawDistance:  32,
			},
		},
		Layers: []worldvegetation.Layer{{Preset: "grass", Density: 1, Seed: 42, Surface: worldvegetation.SurfaceFloor}},
	}

	encoded, err := worldvegetation.Encode(document)
	if err != nil {
		t.Fatal(err)
	}

	pixels := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	pixels.SetNRGBA(0, 0, color.NRGBA{G: 210, A: 127})
	pixels.SetNRGBA(1, 0, color.NRGBA{G: 160, A: 255})

	texture, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		t.Fatal(err)
	}

	result := worldMaterialBuild{data: []level.SourceEntry{{Name: worldvegetation.EntryName, Kind: level.EntryData, Data: encoded}},
		sources: []level.SourceEntry{{Name: level.TextureEntryName(17), Kind: level.EntryTexture, Data: texture},
			{Name: level.TextureEntryName(18), Kind: level.EntryTexture, Data: []byte("unrelated normal data")}}}

	input, err := prepareFixedVegetationBake(result)
	if err != nil || input == nil || len(input.Materials) != 1 || input.Document.Layers[0].Seed != 42 {
		t.Fatal("processed vegetation input missing", err)
	}

	if input.Materials[0].ID != 17 || input.Materials[0].Albedo.Bounds() != pixels.Bounds() ||
		input.Materials[0].Albedo.At(0, 0) != pixels.At(0, 0) {
		t.Fatal("cutout pixels/dimensions changed before baking")
	}

	result.sources = result.sources[1:]
	if _, err := prepareFixedVegetationBake(result); err == nil {
		t.Fatal("missing cutout accepted")
	}
}

func TestFixedVegetationBakeAbsentPreservesLegacyInputs(t *testing.T) {
	t.Parallel()

	input, err := prepareFixedVegetationBake(worldMaterialBuild{})
	if err != nil || input != nil {
		t.Fatal("absent vegetation allocated a bake input", err)
	}
}
