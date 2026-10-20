//nolint:mnd // Fixed RGBA channels, mip halving and two-sided image gutters.
package assetpipeline

import (
	"context"
	"fmt"
	"image"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/textureatlas"
)

// Shelf placement is deterministic and never rotates or rescales images.
// Physical page dimensions are trimmed to used bounds rather than rounded up.
func placeAtlasUnits(units []atlasUnit) [][]atlasUnitPosition {
	type shelf struct{ y, height, x int }

	var (
		pages   [][]atlasUnitPosition
		shelves [][]shelf
	)

	for _, unit := range units {
		if unit.dedicated || unit.width > atlasPageSize || unit.height > atlasPageSize {
			pages = append(pages, []atlasUnitPosition{{unit: unit}})
			shelves = append(shelves, []shelf{{height: unit.height, x: atlasPageSize + 1}})

			continue
		}

		placed := false

		for page := range pages {
			for row := range shelves[page] {
				s := &shelves[page][row]
				if s.height >= unit.height && s.x+unit.width <= atlasPageSize {
					pages[page] = append(pages[page], atlasUnitPosition{unit: unit, x: s.x, y: s.y})
					s.x += unit.width
					placed = true

					break
				}
			}

			if placed {
				break
			}

			last := shelves[page][len(shelves[page])-1]

			y := last.y + last.height
			if y+unit.height <= atlasPageSize && last.x <= atlasPageSize {
				pages[page] = append(pages[page], atlasUnitPosition{unit: unit, y: y})
				shelves[page] = append(shelves[page], shelf{y: y, height: unit.height, x: unit.width})
				placed = true

				break
			}
		}

		if !placed {
			pages = append(pages, []atlasUnitPosition{{unit: unit}})
			shelves = append(shelves, []shelf{{height: unit.height, x: unit.width}})
		}
	}

	return pages
}

func encodeAtlasPage(
	ctx context.Context,
	sources []atlasSource,
	units []atlasUnitPosition,
	result *textureatlas.Bundle,
	regions map[int]textureatlas.Region,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	width, height := 0, 0
	for _, unit := range units {
		width = max(width, unit.x+unit.unit.width)
		height = max(height, unit.y+unit.unit.height)
	}

	pixels := image.NewNRGBA(image.Rect(0, 0, width, height))

	for _, unit := range units {
		for _, rect := range unit.unit.images {
			if err := copyAtlasImage(
				ctx,
				pixels,
				sources[rect.source].levels[rect.mip],
				unit.x+rect.x,
				unit.y+rect.y,
				rect.gutter,
			); err != nil {
				return err
			}
		}
	}

	encoded, _, err := qoi.Encode(pixels, qoi.Options{Channels: qoi.ChannelsRGBA, Colorspace: qoi.ColorspaceSRGB})
	if err != nil {
		return err
	}

	if len(encoded) > textureatlas.MaxPageBytes {
		return splitAtlasPage(ctx, sources, units, result, regions)
	}

	page := len(result.Pages)

	for _, unit := range units {
		for _, rect := range unit.unit.images {
			source := sources[rect.source]
			bounds := source.levels[rect.mip].Bounds()
			region := regions[rect.source]
			region.Page = page
			region.Filter = source.filter

			region.MipCount = len(source.levels) - 1
			if rect.mip == 0 {
				region.X, region.Y = unit.x+rect.x, unit.y+rect.y
				region.Width, region.Height, region.Gutter = bounds.Dx(), bounds.Dy(), rect.gutter
			} else {
				region.Mips[rect.mip-1] = textureatlas.MipRect{
					X:      unit.x + rect.x,
					Y:      unit.y + rect.y,
					Width:  bounds.Dx(),
					Height: bounds.Dy(),
				}
			}

			regions[rect.source] = region
		}
	}

	result.Layout.Pages = append(result.Layout.Pages, textureatlas.Page{Width: width, Height: height})
	result.Pages = append(result.Pages, encoded)

	return nil
}

func copyAtlasImage(ctx context.Context, destination, source *image.NRGBA, left, top, gutter int) error {
	width, height := source.Bounds().Dx(), source.Bounds().Dy()
	for row := -gutter; row < height+gutter; row++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		sy := source.Bounds().Min.Y + max(0, min(height-1, row))
		for column := -gutter; column < width+gutter; column++ {
			sx := source.Bounds().Min.X + max(0, min(width-1, column))
			from, to := source.PixOffset(sx, sy), destination.PixOffset(left+column, top+row)
			copy(destination.Pix[to:to+4], source.Pix[from:from+4])
		}
	}

	return nil
}

func splitAtlasPage(
	ctx context.Context,
	sources []atlasSource,
	units []atlasUnitPosition,
	result *textureatlas.Bundle,
	regions map[int]textureatlas.Region,
) error {
	if len(units) == 1 {
		unit := units[0].unit
		if len(unit.images) != 1 {
			return fmt.Errorf("animation atlas exceeds encoded page bound: %w", textureatlas.ErrAtlas)
		}

		source := unit.images[0].source

		bounds := sources[source].pixels.Bounds()
		if len(sources[source].encoded) > textureatlas.MaxPageBytes {
			return textureatlas.ErrAtlas
		}

		regions[source] = textureatlas.Region{Page: len(result.Pages), Width: bounds.Dx(), Height: bounds.Dy()}
		result.Layout.Pages = append(result.Layout.Pages, textureatlas.Page{Width: bounds.Dx(), Height: bounds.Dy()})
		result.Pages = append(result.Pages, sources[source].encoded)

		return nil
	}
	// Retry bounded groups on smaller pages rather than weakening the public
	// per-entry limit. Connected particle frames remain indivisible.
	middle := len(units) / 2
	for _, part := range [][]atlasUnitPosition{units[:middle], units[middle:]} {
		split := make([]atlasUnit, len(part))
		for i, p := range part {
			split[i] = p.unit
		}

		pages := placeAtlasUnits(split)
		for _, page := range pages {
			if err := encodeAtlasPage(ctx, sources, page, result, regions); err != nil {
				return err
			}
		}
	}

	return nil
}
