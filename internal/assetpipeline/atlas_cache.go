//nolint:mnd // Fixed RGBA channels, mip halving and two-sided image gutters.
package assetpipeline

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-sdk/format/textureatlas"
)

// ProcessTextureAtlas caches the final build product, including its layout.
// The key covers logical names, processed source bytes, connected frame groups
// and packing revisions. Runtime consumers never need the cached source images.
func ProcessTextureAtlas(
	ctx context.Context,
	projectRoot string,
	inputs []AtlasImage,
	groups [][]string,
) (textureatlas.Bundle, []Texture, error) {
	inputs = slices.Clone(inputs)
	slices.SortFunc(inputs, func(a, b AtlasImage) int { return strings.Compare(a.Name, b.Name) })

	sourceAssets := make([]cartridge.Asset, len(inputs))
	for i, input := range inputs {
		sourceAssets[i] = cartridge.Asset{Name: input.Name, Bytes: input.Bytes}
	}

	source, err := cartridge.EncodeAssets(sourceAssets)
	if err != nil {
		return textureatlas.Bundle{}, nil, err
	}

	for _, input := range inputs {
		source = binary.LittleEndian.AppendUint32(source, uint32(input.Filter))
	}
	// Groups affect placement. Sort members and groups so equivalent declarations
	// keep the same cache identity regardless of source traversal order.
	canonical := make([]string, len(groups))
	for i, group := range groups {
		members := slices.Clone(group)
		slices.Sort(members)
		canonical[i] = strings.Join(slices.Compact(members), "\x00")
	}

	slices.Sort(canonical)

	for _, group := range canonical {
		source = binary.LittleEndian.AppendUint32(source, uint32(len(group)))
		source = append(source, group...)
	}

	snapshot, err := SnapshotBytes(source)
	if err != nil {
		return textureatlas.Bundle{}, nil, err
	}

	recipe, err := NewRecipe(
		"texture-atlas",
		"texture-atlas@1",
		[]Revision{{Name: "packing", Version: "shelf-dedup-gutter-mips@2"}, {Name: "codec", Version: "qoi@1"}},
		map[string]int{"pageSize": atlasPageSize, "gutter": textureatlas.Gutter},
	)
	if err != nil {
		return textureatlas.Bundle{}, nil, err
	}

	artifact, err := NewProjectCache(
		projectRoot,
	).Resolve(ctx, snapshot, recipe, func(ctx context.Context, _ SourceSnapshot, _ Recipe) (Processed, error) {
		bundle, err := PackTextureAtlas(ctx, inputs, groups)
		if err != nil {
			return Processed{}, err
		}

		entries, err := bundle.Entries()
		if err != nil {
			return Processed{}, err
		}

		assets := make([]cartridge.Asset, len(entries))
		for i, entry := range entries {
			assets[i] = cartridge.Asset{Name: entry.Name, Bytes: entry.Bytes}
		}

		payload, err := cartridge.EncodeAssets(assets)

		return Processed{Encoding: "texture-atlas@1", Payload: payload}, err
	})
	if err != nil {
		return textureatlas.Bundle{}, nil, err
	}

	encoded, err := os.ReadFile(artifact.PayloadPath)
	if err != nil {
		return textureatlas.Bundle{}, nil, err
	}

	if digest := fmt.Sprintf("%x", sha256.Sum256(encoded)); digest != artifact.OutputDigest || int64(len(encoded)) != artifact.OutputBytes {
		return textureatlas.Bundle{}, nil, ErrCacheOutput
	}

	assets, err := cartridge.DecodeAssets(encoded)
	if err != nil {
		return textureatlas.Bundle{}, nil, err
	}

	entries := make([]textureatlas.Entry, len(assets))
	for i, entry := range assets {
		entries[i] = textureatlas.Entry{Name: entry.Name, Bytes: entry.Bytes}
	}

	bundle, found, err := textureatlas.ReadEntries(entries)
	if err != nil {
		return textureatlas.Bundle{}, nil, err
	}

	if !found {
		return textureatlas.Bundle{}, nil, textureatlas.ErrAtlas
	}

	reports := make([]Texture, len(bundle.Pages))
	for index, page := range bundle.Pages {
		size := bundle.Layout.Pages[index]
		reports[index] = Texture{
			Name:                  textureatlas.PageEntryName(index),
			Kind:                  "texture-atlas",
			Processor:             "texture-atlas@1",
			Encoding:              "qoi@1",
			Width:                 size.Width,
			Height:                size.Height,
			Output:                artifact.OutputDigest + ".atlas#" + textureatlas.PageEntryName(index),
			OutputSHA256:          fmt.Sprintf("%x", sha256.Sum256(page)),
			OutputBytes:           int64(len(page)),
			EstimatedDecodedBytes: int64(size.Width) * int64(size.Height) * 4,
			CacheKey:              artifact.CacheKey,
			CacheHit:              artifact.CacheHit,
			SourcePath:            artifact.PayloadPath,
		}
	}

	return bundle, reports, nil
}
