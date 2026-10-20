package levelbuild

import (
	"bytes"
	"image"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/textureatlas"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/sdk"
)

func TestBuildTimeAtlasPackagesLevelImagesWithoutIndividualPayloads(t *testing.T) {
	t.Parallel()

	pixels := image.NewNRGBA(image.Rect(0, 0, 24, 16))
	for i := range pixels.Pix {
		pixels.Pix[i] = byte(i * 11)
	}

	encoded, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		t.Fatal(err)
	}

	entries := []level.SourceEntry{
		{Name: level.TextureEntryName(7), Kind: level.EntryTexture, Data: encoded},
		{Name: level.TextureEntryName(9), Kind: level.EntryTexture, Data: encoded},
	}
	reports := []assetpipeline.Texture{{Name: "a", Kind: "texture"}, {Name: "b", Kind: "texture"}}
	selected := sdk.Manifest{}
	selected.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityTextureQOIv1, asset.CapabilityTextureAtlasV1}
	assets := &assetBuild{projectRoot: t.TempDir(), manifest: selected}

	first, outputs, packed, err := packageTextureAtlas(t.Context(), entries, reports, assets)
	if err != nil {
		t.Fatal(err)
	}

	if !packed || len(outputs) != 1 || outputs[0].EstimatedDecodedBytes != 28*20*4 {
		t.Fatal("physical page accounting", outputs)
	}

	second, _, _, err := packageTextureAtlas(t.Context(), entries, reports, assets)
	if err != nil || len(first) != len(second) {
		t.Fatal(err)
	}

	reserved := make([]textureatlas.Entry, len(first))
	for i, entry := range first {
		if entry.Kind == level.EntryTexture || !bytes.Equal(entry.Data, second[i].Data) {
			t.Fatal("individual payload or unstable page output")
		}

		reserved[i] = textureatlas.Entry{Name: entry.Name, Bytes: entry.Data}
	}

	bundle, found, err := textureatlas.ReadEntries(reserved)
	if err != nil || !found || len(bundle.Layout.Regions) != 2 {
		t.Fatal("missing logical IDs", err)
	}

	if names := []string{
		bundle.Layout.Regions[0].Name,
		bundle.Layout.Regions[1].Name,
	}; !slices.Equal(
		names,
		[]string{level.TextureEntryName(7), level.TextureEntryName(9)},
	) {
		t.Fatal(names)
	}
}
