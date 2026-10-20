package assetpipeline

import (
	"bytes"
	"image"
	"testing"

	atlasfilter "github.com/karty-game/karty-sdk/bake/textureatlas"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/textureatlas"
)

func TestBuildTimeAtlasPreservesPixelsDimensionsAliasesAndGutters(t *testing.T) {
	t.Parallel()

	inputs := make([]AtlasImage, 0, 4)

	originals := make(map[string]*image.NRGBA)

	for i, name := range []string{"wide", "tall", "alpha"} {
		pixels := image.NewNRGBA(image.Rect(0, 0, 7+i*4, 19-i*5))
		for at := range pixels.Pix {
			pixels.Pix[at] = byte((at*19 + i*23) % 256)
		}

		encoded, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
		if err != nil {
			t.Fatal(err)
		}

		inputs = append(inputs, AtlasImage{Name: name, Bytes: encoded})
		originals[name] = pixels
	}

	inputs = append(inputs, AtlasImage{Name: "wide-alias", Bytes: inputs[0].Bytes})
	originals["wide-alias"] = originals["wide"]

	first, err := PackTextureAtlas(t.Context(), inputs, [][]string{{"wide", "alpha"}})
	if err != nil {
		t.Fatal(err)
	}

	second, err := PackTextureAtlas(t.Context(), inputs, [][]string{{"wide", "alpha"}})
	if err != nil {
		t.Fatal(err)
	}

	if len(first.Pages) >= len(inputs) {
		t.Fatal("images were not combined")
	}

	for index, page := range first.Pages {
		if !bytes.Equal(page, second.Pages[index]) {
			t.Fatal("nondeterministic atlas pixels")
		}

		verifyAtlasPixels(t, first, index, page, originals)
	}
}

func verifyAtlasPixels(t *testing.T, bundle textureatlas.Bundle, index int, page []byte, originals map[string]*image.NRGBA) {
	t.Helper()

	_, pixels, err := qoi.Decode(page)
	if err != nil {
		t.Fatal(err)
	}

	for _, region := range bundle.Layout.Regions {
		if region.Page != index {
			continue
		}

		original := originals[region.Name]
		if region.Width != original.Bounds().Dx() || region.Height != original.Bounds().Dy() {
			t.Fatal("resized source")
		}

		for y := -region.Gutter; y < region.Height+region.Gutter; y++ {
			for x := -region.Gutter; x < region.Width+region.Gutter; x++ {
				want := original.NRGBAAt(max(0, min(region.Width-1, x)), max(0, min(region.Height-1, y)))
				if pixels.NRGBAAt(region.X+x, region.Y+y) != want {
					t.Fatalf("changed %s pixel %d,%d", region.Name, x, y)
				}
			}
		}
	}
}

func TestBuildTimeAtlasCacheIdentityAndDedicatedPage(t *testing.T) {
	t.Parallel()

	pixels := image.NewNRGBA(image.Rect(0, 0, 4094, 1))

	encoded, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		t.Fatal(err)
	}

	inputs := []AtlasImage{{Name: "wide", Bytes: encoded}, {Name: "small", Bytes: inputsForCache(t)}}
	directory := t.TempDir()

	first, reports, err := ProcessTextureAtlas(t.Context(), directory, inputs, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(first.Pages) != 2 || len(reports) != 2 || reports[0].CacheHit {
		t.Fatal("dedicated source must not share a page", reports)
	}

	second, reports, err := ProcessTextureAtlas(t.Context(), directory, inputs, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !reports[0].CacheHit || !bytes.Equal(first.Pages[0], second.Pages[0]) {
		t.Fatal("final atlas cache was not reused")
	}

	_, grouped, err := ProcessTextureAtlas(t.Context(), directory, inputs, [][]string{{"small"}})
	if err != nil {
		t.Fatal(err)
	}

	if grouped[0].CacheHit {
		t.Fatal("changed frame groups reused an incompatible layout")
	}
}

func inputsForCache(t *testing.T) []byte {
	t.Helper()

	encoded, _, err := qoi.Encode(
		image.NewNRGBA(image.Rect(0, 0, 8, 8)),
		qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB},
	)
	if err != nil {
		t.Fatal(err)
	}

	return encoded
}

func TestBuildTimeAtlasFiltersFoliageMipsIntoSharedPages(t *testing.T) {
	t.Parallel()

	pixels := image.NewNRGBA(image.Rect(0, 0, 33, 33))
	for i := range pixels.Pix {
		pixels.Pix[i] = byte(i * 11)
	}

	encoded, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		t.Fatal(err)
	}

	bundle, err := PackTextureAtlas(
		t.Context(),
		[]AtlasImage{
			{Name: "color", Bytes: encoded, Filter: textureatlas.FilterColor},
			{Name: "normal", Bytes: encoded, Filter: textureatlas.FilterNormal},
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(bundle.Pages) != 1 {
		t.Fatal("foliage chains did not share the level page")
	}

	_, page, err := qoi.Decode(bundle.Pages[0])
	if err != nil {
		t.Fatal(err)
	}

	for _, region := range bundle.Layout.Regions {
		if region.MipCount != 2 {
			t.Fatal("missing build-time mip chain", region)
		}

		current := pixels.Pix

		width, height := 33, 33
		for _, rect := range region.Mips[:region.MipCount] {
			current = atlasfilter.Mip(current, width, height, region.Filter == textureatlas.FilterNormal)
			if region.Filter == textureatlas.FilterColor {
				atlasfilter.PreserveCoverage(current, atlasfilter.Coverage(pixels.Pix, 1))
			}

			width, height = width/2, height/2
			for y := range height {
				for x := range width {
					at := page.PixOffset(rect.X+x, rect.Y+y)

					from := (y*width + x) * 4
					if !bytes.Equal(page.Pix[at:at+4], current[from:from+4]) {
						t.Fatal("runtime would receive different filtered pixels")
					}
				}
			}
		}
	}

	layout, err := textureatlas.EncodeLayout(bundle.Layout)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := textureatlas.DecodeLayout(layout)
	if err != nil || decoded.Regions[1].Mips != bundle.Layout.Regions[1].Mips {
		t.Fatal("mip layout round trip", err)
	}
}

func TestBuildTimeAtlasGroupTraversalDoesNotChangeOutput(t *testing.T) {
	t.Parallel()

	inputs := make([]AtlasImage, 0, 4)

	for index, name := range []string{"a", "b", "c", "d"} {
		pixels := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		for offset := range pixels.Pix {
			pixels.Pix[offset] = byte(offset + index*31)
		}

		encoded, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
		if err != nil {
			t.Fatal(err)
		}

		inputs = append(inputs, AtlasImage{Name: name, Bytes: encoded})
	}

	first, err := PackTextureAtlas(t.Context(), inputs, [][]string{{"d", "a"}, {"b", "c"}})
	if err != nil {
		t.Fatal(err)
	}

	second, err := PackTextureAtlas(t.Context(), inputs, [][]string{{"c", "b"}, {"a", "d"}})
	if err != nil {
		t.Fatal(err)
	}

	before, err := first.Entries()
	if err != nil {
		t.Fatal(err)
	}

	after, err := second.Entries()
	if err != nil || len(before) != len(after) {
		t.Fatal("group order changed atlas entries", err)
	}

	for index := range before {
		if before[index].Name != after[index].Name || !bytes.Equal(before[index].Bytes, after[index].Bytes) {
			t.Fatal("equivalent connected groups changed deterministic output")
		}
	}
}
