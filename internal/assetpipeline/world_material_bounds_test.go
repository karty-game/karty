package assetpipeline

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/karty-game/karty/internal/sdk"
)

func TestWorldMaterialMaximumRecipeFitsCache(t *testing.T) {
	t.Parallel()

	ids := make([]uint32, 196)
	textures := make(map[uint32][]byte, len(ids))
	encoded := fixtureQOI(t, 1, 1, color.NRGBA{A: 255})

	for index := range ids {
		ids[index] = uint32(index + 1)
		textures[ids[index]] = encoded
	}

	layout, err := worldmaterial.NewLayout(ids)
	if err != nil {
		t.Fatal(err)
	}

	recipe, err := worldMaterialRecipe(fixtureMaterialManifest(), layout, textures,
		"linux-amd64", strings.Repeat("a", 64), materializeArguments("atlas.png", "."))
	if err != nil || len(recipe.OptionsJSON()) > MaxCacheOptionsBytes {
		t.Fatalf("maximum grid recipe: %v", err)
	}

	metadata, err := worldmaterial.EncodeLayout(layout)
	if err != nil || len(metadata) > MaxCacheMetadataBytes/2 {
		t.Fatalf("maximum grid cache metadata: %d bytes, %v", len(metadata), err)
	}
}

func TestWorldMaterialCacheFramingRejectsMalformedLengths(t *testing.T) {
	t.Parallel()

	layout, err := worldmaterial.NewLayout([]uint32{0})
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := SnapshotBytes([]byte("fixture"))
	if err != nil {
		t.Fatal(err)
	}

	recipe, err := worldMaterialRecipe(fixtureMaterialManifest(), layout, nil,
		"linux-amd64", strings.Repeat("a", 64), materializeArguments("atlas.png", "."))
	if err != nil {
		t.Fatal(err)
	}

	for _, length := range []uint32{0, 1, 0xffffffff} {
		t.Run(strconv.FormatUint(uint64(length), 10), func(t *testing.T) {
			t.Parallel()

			payload := make([]byte, materialPayloadHeader+1)
			binary.BigEndian.PutUint32(payload[:4], length)
			binary.BigEndian.PutUint32(payload[4:8], length)
			binary.BigEndian.PutUint32(payload[8:materialPayloadHeader], length)

			cached, err := NewProjectCache(t.TempDir()).Resolve(t.Context(), snapshot, recipe,
				func(context.Context, SourceSnapshot, Recipe) (Processed, error) {
					return Processed{Encoding: worldMaterialProcessor, Payload: payload, Metadata: layout}, nil
				})
			if err != nil {
				t.Fatal(err)
			}

			if _, err := loadWorldMaterialPair(t.Context(), cached, layout); !errors.Is(err, ErrCacheOutput) {
				t.Fatalf("malformed length accepted: %v", err)
			}
		})
	}
}

func TestWorldMaterialCancellationAfterNativeNeverPublishes(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	root := t.TempDir()
	calls := 0

	var temporary string

	execute := func(ctx context.Context, _ sdk.Manifest, directory string, _ []string) error {
		calls++
		temporary = directory

		deadline, found := ctx.Deadline()
		if !found || time.Until(deadline) > worldMaterialTimeout {
			t.Fatal("native processor is not deadline bounded")
		}

		cancel()

		return nil
	}

	_, _, err := processWorldMaterials(ctx, root, fixtureMaterialManifest(), fixtureMaterialWorld(), fixtureTextures(t), execute)
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("canceled native result: calls=%d err=%v", calls, err)
	}

	if _, err := os.Stat(temporary); !os.IsNotExist(err) {
		t.Fatal("temporary native outputs survived cancellation")
	}

	payloads, err := filepath.Glob(filepath.Join(root, ".karty", "cache", "assets", "v2", "*", "payload"))
	if err != nil || len(payloads) != 0 {
		t.Fatalf("canceled result published: %v, %v", payloads, err)
	}
}

func TestWorldMaterialNearestPaletteAndGutters(t *testing.T) {
	t.Parallel()

	layout, err := worldmaterial.NewLayout([]uint32{1})
	if err != nil {
		t.Fatal(err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))

	for row := range 2 {
		for column := range 2 {
			img.SetNRGBA(column, row, color.NRGBA{
				R: uint8(column * 200), G: uint8(row * 120), B: uint8(column * row * 180), A: 255,
			})
		}
	}

	encoded, _, err := qoi.Encode(img, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		t.Fatal(err)
	}

	atlas, err := packWorldAlbedo(t.Context(), layout, map[uint32][]byte{1: encoded})
	if err != nil {
		t.Fatal(err)
	}

	rect := layout.Materials[0]
	// Every texel must retain an authored palette color, including gutters.

	for row := -rect.Gutter; row < rect.Height+rect.Gutter; row++ {
		for column := -rect.Gutter; column < rect.Width+rect.Gutter; column++ {
			want := color.NRGBA{A: 255}
			if column >= rect.Width/2 {
				want.R = 200
			}

			if row >= rect.Height/2 {
				want.G = 120
			}

			if want.R != 0 && want.G != 0 {
				want.B = 180
			}

			if got := atlas.NRGBAAt(rect.X+column, rect.Y+row); got != want {
				t.Fatalf("nearest palette/extruded texel at (%d,%d): %v != %v", column, row, got, want)
			}
		}
	}
}

func TestWorldMaterialNearestPackingInvalidatesSmoothCache(t *testing.T) {
	t.Parallel()

	layout, err := worldmaterial.NewLayout([]uint32{9, 3, 0, 7})
	if err != nil {
		t.Fatal(err)
	}

	current, err := worldMaterialRecipe(fixtureMaterialManifest(), layout, fixtureTextures(t),
		"linux-amd64", strings.Repeat("a", 64), materializeArguments("atlas.png", "."))
	if err != nil {
		t.Fatal(err)
	}

	revisions := append([]Revision(nil), current.revisions...)
	found := false

	for index := range revisions {
		if revisions[index].Name == "packer" {
			if revisions[index].Version != "5" {
				t.Fatal("nearest packer revision missing")
			}

			revisions[index].Version = "3"
			found = true
		}
	}

	previous, err := NewRecipe(current.kind, current.processor, revisions, json.RawMessage(current.options))
	if err != nil || !found || previous.Digest() == current.Digest() {
		t.Fatalf("previous smooth atlas can be reused: %v", err)
	}
}

func TestWorldMaterialNativeFixtureFormats(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	bounds := image.Rect(0, 0, 2, 2)

	if err := fixtureMaterialMaps(directory, bounds); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"normal", "height", "ao"} {
		encoded, err := os.ReadFile(filepath.Join(directory, "atlas_"+name+".png"))
		if err != nil {
			t.Fatal(err)
		}

		config, err := png.DecodeConfig(bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}

		if encoded[24] != 8 || (name == "normal" && config.ColorModel != color.RGBAModel) ||
			(name != "normal" && config.ColorModel != color.GrayModel) {
			t.Fatalf("fixture does not match pinned native format: %s", name)
		}
	}

	// Public layout remains JSON, despite private binary cache framing.
	layout, err := worldmaterial.NewLayout([]uint32{0})
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := worldmaterial.EncodeLayout(layout)
	if err != nil || !json.Valid(metadata) {
		t.Fatalf("layout JSON: %v", err)
	}
}

func TestWorldMaterialMergeRetainsStraightDataChannels(t *testing.T) {
	t.Parallel()

	bounds := image.Rect(0, 0, 2, 2)
	directory := t.TempDir()
	normal := image.NewRGBA(bounds)
	height, occlusion := image.NewGray(bounds), image.NewGray(bounds)
	want := []color.NRGBA{
		{R: 0, G: 255, B: 255, A: 0},
		{R: 128, G: 128, B: 0, A: 1},
		{R: 255, G: 0, B: 37, A: 83},
		{R: 128, G: 128, B: 128, A: 255},
	}

	for index, pixel := range want {
		column, row := index%bounds.Dx(), index/bounds.Dx()

		normalPixel := color.RGBA{R: pixel.R, G: pixel.G, B: 29, A: 255}
		if index == 1 {
			normalPixel.R, normalPixel.G = 127, 127
		}

		normal.SetRGBA(column, row, normalPixel)
		height.SetGray(column, row, color.Gray{Y: pixel.B})
		occlusion.SetGray(column, row, color.Gray{Y: pixel.A})
	}

	data := image.NewNRGBA(bounds)

	for _, output := range []struct {
		name  string
		image image.Image
	}{{"normal", normal}, {"height", height}, {"ao", occlusion}} {
		if err := writeMaterialPNG(filepath.Join(directory, "atlas_"+output.name+".png"), output.image); err != nil {
			t.Fatal(err)
		}

		if err := mergeMaterialMap(t.Context(), directory, output.name, data); err != nil {
			t.Fatal(err)
		}
	}

	encoded, metadata, err := qoi.Encode(data, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceLinear})
	if err != nil || metadata.Colorspace != qoi.ColorspaceLinear || metadata.Channels != qoi.ChannelsRGBA {
		t.Fatalf("material data QOI header: %+v, %v", metadata, err)
	}

	_, decoded, err := qoi.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	for index, pixel := range want {
		if got := decoded.NRGBAAt(index%bounds.Dx(), index/bounds.Dx()); got != pixel {
			t.Fatalf("material data texel %d: %v != %v (AO is not opacity)", index, got, pixel)
		}
	}
}

func TestWorldMaterialHeightAndAOFailuresNeverPublish(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"height", "ao"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			calls := 0
			execute := func(_ context.Context, _ sdk.Manifest, directory string, args []string) error {
				calls++

				input, err := os.ReadFile(args[0])
				if err != nil {
					return err
				}

				config, err := png.DecodeConfig(bytes.NewReader(input))
				if err != nil {
					return err
				}

				bounds := image.Rect(0, 0, config.Width, config.Height)
				if err := fixtureMaterialMaps(directory, bounds); err != nil {
					return err
				}

				// Fail after the earlier generated channels have already merged.
				transparent := image.NewNRGBA(bounds)
				draw.Draw(transparent, bounds, image.NewUniform(color.NRGBA{R: 61, A: 254}), image.Point{}, draw.Src)

				return writeMaterialPNG(filepath.Join(directory, "atlas_"+name+".png"), transparent)
			}

			for range 2 {
				_, _, err := processWorldMaterials(t.Context(), root, fixtureMaterialManifest(),
					fixtureMaterialWorld(), fixtureTextures(t), execute)
				if !errors.Is(err, ErrCacheOutput) {
					t.Fatalf("transparent %s accepted: %v", name, err)
				}
			}

			if calls != 2 {
				t.Fatal("partial merged pair was published")
			}
		})
	}
}
