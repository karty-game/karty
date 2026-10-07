//nolint:gocognit,varnamelen // Bounded tile filling keeps queue traversal and image coordinates together.
package assetpipeline

import (
	"context"
	"image"
	"image/color"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
)

// An opaque fast-path claim requires an opaque atlas slot, even when other
// regions use the same tile with masked sampling because their UVs leave its
// domain. Packing then validates every source pixel against this claim.
func isMaskedFrameMaterial(document world.Document, id uint32) bool {
	if requiresOpaqueMaterial(document, id) {
		return false
	}

	masked := false

	for _, sector := range document.Sectors {
		for _, wall := range sector.Walls {
			for _, region := range wall.FrameRegions {
				if region.Coverage != world.FrameCoverageMain && region.Material == id {
					if region.Coverage == world.FrameCoverageOpaque {
						return false
					}

					masked = true
				}
			}
		}
	}

	return masked
}

func secondaryOnlyMaterials(document world.Document) map[uint32]bool {
	result := make(map[uint32]bool)

	full := make(map[uint32]bool)
	for _, sector := range document.Sectors {
		full[sector.FloorMaterial], full[sector.CeilingMaterial] = true, true
		for _, layer := range []*world.SurfaceSecondary{sector.FloorSecondary, sector.CeilingSecondary} {
			if layer != nil {
				result[layer.Material] = true
			}
		}

		for _, wall := range sector.Walls {
			full[wall.Material] = true
			if wall.Secondary != nil {
				result[wall.Secondary.Material] = true
			}

			for _, region := range wall.FrameRegions {
				full[region.Material] = true
			}
		}
	}

	if document.StaticSolids != nil {
		for _, solid := range document.StaticSolids.Items {
			full[solid.SideMaterial], full[solid.TopMaterial], full[solid.BottomMaterial] = true, true, true
		}
	}

	for id := range full {
		delete(result, id)
	}

	return result
}

func neutralizeSecondaryTiles(layout worldmaterial.Layout, pixels *image.NRGBA, ids map[uint32]bool, value color.NRGBA) {
	for _, rect := range layout.Materials {
		if !ids[rect.MaterialID] {
			continue
		}

		for y := -rect.Gutter; y < rect.Height+rect.Gutter; y++ {
			for x := -rect.Gutter; x < rect.Width+rect.Gutter; x++ {
				pixels.SetNRGBA(rect.X+x, rect.Y+y, value)
			}
		}
	}
}

// Materialize receives opaque colors, never coverage silhouettes. A bounded
// multi-source breadth-first fill extends covered colors within each tile.
// The original straight color/coverage image remains the packaged albedo.
func materialGenerationInput(ctx context.Context, layout worldmaterial.Layout, albedo *image.NRGBA) (*image.NRGBA, error) {
	result := image.NewNRGBA(albedo.Bounds())
	copy(result.Pix, albedo.Pix)

	for _, rect := range layout.Materials {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if rect.Coverage != worldmaterial.CoverageMasked {
			continue
		}

		seen := make([]bool, rect.Width*rect.Height)
		queue := make([]int, 0, len(seen))

		for y := range rect.Height {
			for x := range rect.Width {
				if result.NRGBAAt(rect.X+x, rect.Y+y).A != 0 {
					index := y*rect.Width + x
					seen[index] = true
					queue = append(queue, index)
				}
			}
		}

		if len(queue) == 0 {
			for y := range rect.Height {
				for x := range rect.Width {
					result.SetNRGBA(rect.X+x, rect.Y+y, color.NRGBA{128, 128, 128, 255})
				}
			}
		}

		for head := 0; head < len(queue); head++ {
			index := queue[head]
			x, y := index%rect.Width, index/rect.Width

			pixel := result.NRGBAAt(rect.X+x, rect.Y+y)
			for _, step := range [4]image.Point{{X: -1}, {X: 1}, {Y: -1}, {Y: 1}} {
				nx, ny := x+step.X, y+step.Y
				if nx < 0 || ny < 0 || nx >= rect.Width || ny >= rect.Height {
					continue
				}

				next := ny*rect.Width + nx
				if seen[next] {
					continue
				}

				seen[next] = true

				result.SetNRGBA(rect.X+nx, rect.Y+ny, pixel)

				queue = append(queue, next)
			}
		}

		for y := -rect.Gutter; y < rect.Height+rect.Gutter; y++ {
			for x := -rect.Gutter; x < rect.Width+rect.Gutter; x++ {
				pixel := result.NRGBAAt(rect.X+min(max(x, 0), rect.Width-1), rect.Y+min(max(y, 0), rect.Height-1))
				pixel.A = 255
				result.SetNRGBA(rect.X+x, rect.Y+y, pixel)
			}
		}
	}

	return result, nil
}

// A shared main/secondary/band texture must satisfy its opaque consumer too.
func requiresOpaqueMaterial(document world.Document, materialID uint32) bool {
	secondary := func(layer *world.SurfaceSecondary) bool { return layer != nil && layer.Material == materialID }
	for _, sector := range document.Sectors {
		if sector.FloorMaterial == materialID || sector.CeilingMaterial == materialID || secondary(sector.FloorSecondary) ||
			secondary(sector.CeilingSecondary) {
			return true
		}

		for _, wall := range sector.Walls {
			if wall.Material == materialID || secondary(wall.Secondary) {
				return true
			}
		}
	}

	if document.StaticSolids != nil {
		for _, solid := range document.StaticSolids.Items {
			if solid.SideMaterial == materialID || solid.TopMaterial == materialID || solid.BottomMaterial == materialID {
				return true
			}
		}
	}

	return false
}
