package levelbuild

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty/internal/assetpipeline"
)

func processedFixedAlbedo(t *testing.T) []byte {
	t.Helper()

	var source bytes.Buffer

	pixels := image.NewNRGBA(image.Rect(0, 0, 8, 4))

	for y := range 4 {
		for x := range 8 {
			pixels.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 30), G: uint8(y * 60), A: 255})
		}
	}

	if err := png.Encode(&source, pixels); err != nil {
		t.Fatal(err)
	}

	processed, err := assetpipeline.ProcessImage(source.Bytes(), asset.ImageRecipe{
		MaxWidth: 2, MaxHeight: 1, Filter: asset.ImageFilterNearest, BitDepth: 8,
	})
	if err != nil {
		t.Fatal(err)
	}

	return processed.EncodedQOI
}

// A valid maximum-size opaque QOI stream needs no large image allocation.
func maximumFixedAlbedo() []byte {
	const pixels = 4096 * 4096

	encoded := make([]byte, 14, 14+(pixels+61)/62+8)
	copy(encoded, "qoif")
	binary.BigEndian.PutUint32(encoded[4:8], 4096)
	binary.BigEndian.PutUint32(encoded[8:12], 4096)
	encoded[12] = qoi.ChannelsRGBA

	for remaining := pixels; remaining > 0; {
		count := min(62, remaining)
		encoded = append(encoded, 0xc0|byte(count-1))
		remaining -= count
	}

	return append(encoded, 0, 0, 0, 0, 0, 0, 0, 1)
}

func TestFixedBakeProcessedAlbedoPreflight(t *testing.T) {
	t.Parallel()
	document, _, _ := incrementalBakeFixture(t)
	document.Sectors[2].FloorMaterial = 2

	small, maximum := processedFixedAlbedo(t), maximumFixedAlbedo()
	if _, err := qoi.Validate(maximum); err != nil {
		t.Fatal("maximum fixture must be a valid bounded stream", err)
	}

	for _, name := range []string{"missing", "malformed", "wrong kind", "duplicate", "duplicate empty first", "aggregate pixels"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sources := []level.SourceEntry{
				{Name: level.TextureEntryName(1), Kind: level.EntryTexture, Data: small},
				{Name: level.TextureEntryName(2), Kind: level.EntryTexture, Data: small},
			}

			switch name {
			case "missing":
				sources = sources[:1]
			case "malformed":
				sources[1].Data = []byte("invalid QOI")
			case "wrong kind":
				sources[1].Kind = level.EntryData
			case "duplicate":
				sources = append(sources, sources[0])
			case "duplicate empty first":
				sources = append([]level.SourceEntry{{Name: sources[0].Name, Kind: level.EntryTexture}}, sources...)
			case "aggregate pixels":
				sources[0].Data = maximum
			}

			if _, err := loadFixedBakeMaterials(&document, sources); !errors.Is(err, ErrSource) ||
				name == "aggregate pixels" && !strings.Contains(err.Error(), "exceed") {
				t.Fatal("invalid required albedo escaped aggregate preflight", name, err)
			}
		})
	}
}
