//nolint:mnd // Fixed RGBA channels, mip halving and two-sided image gutters.
package assetpipeline

import (
	"context"
	"fmt"
	"image"
	"slices"

	"github.com/karty-game/karty-sdk/format/textureatlas"
)

const atlasPageSize = 4096

type AtlasImage struct {
	Filter int
	Name   string
	Bytes  []byte
}

type atlasSource struct {
	filter  int
	levels  []*image.NRGBA
	name    string
	encoded []byte
	pixels  *image.NRGBA
}
type atlasPlacement struct {
	source       int
	mip          int
	x, y, gutter int
}
type atlasUnit struct {
	dedicated     bool
	width, height int
	images        []atlasPlacement
}
type atlasUnitPosition struct {
	unit atlasUnit
	x, y int
}

// PackTextureAtlas preserves source pixels, dimensions and logical names. Groups
// are connected animation frames that must share one physical page. Identical
// encoded images share one rectangle, including when named by multiple frames.
func PackTextureAtlas(ctx context.Context, inputs []AtlasImage, groups [][]string) (textureatlas.Bundle, error) {
	if len(inputs) == 0 || len(inputs) > textureatlas.MaxRegions {
		return textureatlas.Bundle{}, textureatlas.ErrAtlas
	}

	inputs = slices.Clone(inputs)
	slices.SortFunc(inputs, func(a, b AtlasImage) int { return compareAtlasNames(a.Name, b.Name) })

	sources, aliases, err := decodeAtlasSources(ctx, inputs)
	if err != nil {
		return textureatlas.Bundle{}, err
	}

	units, err := atlasUnits(sources, aliases, groups)
	if err != nil {
		return textureatlas.Bundle{}, err
	}

	pages := placeAtlasUnits(units)
	result := textureatlas.Bundle{}

	regions := make(map[int]textureatlas.Region, len(sources))
	for _, page := range pages {
		if err := encodeAtlasPage(ctx, sources, page, &result, regions); err != nil {
			return textureatlas.Bundle{}, err
		}
	}

	for _, input := range inputs {
		region := regions[aliases[input.Name]]
		region.Name = input.Name
		result.Layout.Regions = append(result.Layout.Regions, region)
	}

	if err := result.Validate(); err != nil {
		return textureatlas.Bundle{}, err
	}

	return result, nil
}

func compareAtlasNames(a, b string) int {
	if a < b {
		return -1
	}

	if a > b {
		return 1
	}

	return 0
}

func atlasUnits(sources []atlasSource, names map[string]int, groups [][]string) ([]atlasUnit, error) {
	parents := make([]int, len(sources))
	for i := range parents {
		parents[i] = i
	}

	root := func(i int) int {
		for parents[i] != i {
			i = parents[i]
		}

		return i
	}

	for _, group := range groups {
		leader := -1

		for _, name := range group {
			index, found := names[name]
			if !found {
				return nil, fmt.Errorf("atlas frame %q: %w", name, textureatlas.ErrAtlas)
			}

			if leader < 0 {
				leader = root(index)
			} else {
				left, right := root(index), root(leader)
				parents[max(left, right)] = min(left, right)
			}
		}
	}

	sets := make(map[int][]int)

	for index := range sources {
		leader := root(index)
		sets[leader] = append(sets[leader], index)
	}

	var units []atlasUnit

	for leader := range sources {
		indices := sets[leader]
		if len(indices) == 0 {
			continue
		}

		unit, err := packAtlasUnit(sources, indices)
		if err != nil {
			return nil, err
		}

		units = append(units, unit)
	}

	slices.SortStableFunc(units, func(a, b atlasUnit) int {
		if a.height != b.height {
			return b.height - a.height
		}

		return b.width - a.width
	})

	return units, nil
}

func packAtlasUnit(sources []atlasSource, indices []int) (atlasUnit, error) {
	var images []atlasPlacement

	for _, index := range indices {
		for mip := range sources[index].levels {
			images = append(images, atlasPlacement{source: index, mip: mip})
		}
	}

	slices.SortStableFunc(images, func(a, b atlasPlacement) int {
		ap, bp := sources[a.source].levels[a.mip].Bounds(), sources[b.source].levels[b.mip].Bounds()
		if ap.Dy() != bp.Dy() {
			return bp.Dy() - ap.Dy()
		}

		return bp.Dx() - ap.Dx()
	})

	unit := atlasUnit{}
	column, rowY, row := 0, 0, 0

	for _, placement := range images {
		bounds := sources[placement.source].levels[placement.mip].Bounds()
		width, height := bounds.Dx(), bounds.Dy()

		gutter := textureatlas.Gutter
		if len(images) == 1 && (width+2*gutter > atlasPageSize || height+2*gutter > atlasPageSize) {
			unit = atlasUnit{width: width, height: height, dedicated: true, images: []atlasPlacement{placement}}

			break
		}

		if column+width+2*gutter > atlasPageSize {
			column, rowY, row = 0, rowY+row, 0
		}

		if width+2*gutter > atlasPageSize || rowY+height+2*gutter > atlasPageSize {
			return atlasUnit{}, fmt.Errorf("connected image/mip group does not fit an atlas page: %width", textureatlas.ErrAtlas)
		}

		placement.x, placement.y, placement.gutter = column+gutter, rowY+gutter, gutter
		unit.images = append(unit.images, placement)
		unit.width, unit.height = max(unit.width, column+width+2*gutter), max(unit.height, rowY+height+2*gutter)
		column += width + 2*gutter
		row = max(row, height+2*gutter)
	}

	return unit, nil
}
