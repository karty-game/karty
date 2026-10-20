//nolint:mnd // Fixed RGBA channels, mip halving and two-sided image gutters.
package assetpipeline

import (
	"context"
	"crypto/sha256"
	"image"

	atlasfilter "github.com/karty-game/karty-sdk/bake/textureatlas"
	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/textureatlas"
)

func decodeAtlasSources(ctx context.Context, inputs []AtlasImage) ([]atlasSource, map[string]int, error) {
	sources := make([]atlasSource, 0, len(inputs))
	aliases := make(map[string]int, len(inputs))
	digests := make(map[[33]byte]int, len(inputs))

	var decodedBytes uint64

	previous := ""

	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		if input.Name <= previous || textureatlas.IsEntry(input.Name) {
			return nil, nil, textureatlas.ErrAtlas
		}

		previous = input.Name
		if input.Filter < textureatlas.FilterNearest || input.Filter > textureatlas.FilterNormal {
			return nil, nil, textureatlas.ErrAtlas
		}

		digest := [33]byte{}
		hash := sha256.Sum256(input.Bytes)
		copy(digest[:32], hash[:])

		digest[32] = byte(input.Filter)
		if index, found := digests[digest]; found {
			aliases[input.Name] = index

			continue
		}

		metadata, err := qoi.Inspect(input.Bytes)
		if err != nil || metadata.Channels != qoi.ChannelsRGBA || metadata.Colorspace != qoi.ColorspaceSRGB ||
			metadata.DecodedBytes > asset.MaxDecodedTextures-decodedBytes {
			return nil, nil, textureatlas.ErrAtlas
		}

		decodedBytes += metadata.DecodedBytes

		_, pixels, err := qoi.Decode(input.Bytes)
		if err != nil {
			return nil, nil, err
		}

		index := len(sources)

		source := atlasSource{name: input.Name, encoded: input.Bytes, pixels: pixels, filter: input.Filter, levels: []*image.NRGBA{pixels}}
		if input.Filter != textureatlas.FilterNearest {
			coverage := atlasfilter.Coverage(pixels.Pix, 1)
			for current := pixels; min(current.Rect.Dx()/2, current.Rect.Dy()/2) >= 8; {
				width, height := current.Rect.Dx(), current.Rect.Dy()

				filtered := atlasfilter.Mip(current.Pix, width, height, input.Filter == textureatlas.FilterNormal)
				if input.Filter == textureatlas.FilterColor {
					atlasfilter.PreserveCoverage(filtered, coverage)
				}

				current = &image.NRGBA{Pix: filtered, Stride: width / 2 * 4, Rect: image.Rect(0, 0, width/2, height/2)}
				source.levels = append(source.levels, current)
			}
		}

		sources = append(sources, source)
		aliases[input.Name], digests[digest] = index, index
	}

	return sources, aliases, nil
}
