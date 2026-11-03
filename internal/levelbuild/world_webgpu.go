package levelbuild

import (
	"fmt"

	"github.com/karty-game/karty-sdk/format/world"
)

// Private build profile revision. Legacy cache keys deliberately remain exact.
const webGPUChartRevision = "karty-cli-webgpu-charts@3"

// A one-hot triplanar mapping is already one affine projection. Retain that
// exact projection, including offsets and orientation; blended mappings need
// spatial material baking rather than dropping their other contributions.
func reduceWebGPUProjection(mapping **world.SurfaceUV) {
	value := *mapping
	if value == nil || len(value.Projections) != 3 || len(value.Weights) != 3 {
		return
	}

	selected := -1

	for index, weight := range value.Weights {
		if weight == 1 {
			if selected >= 0 {
				return
			}

			selected = index
		} else if weight != 0 {
			return
		}
	}

	if selected >= 0 {
		*mapping = &world.SurfaceUV{Projections: []world.UVProjection{value.Projections[selected]}, Weights: []float64{1}}
	}
}

func reduceWebGPUProjections(document *world.Document) {
	for index := range document.Sectors {
		sector := &document.Sectors[index]
		reduceWebGPUProjection(&sector.FloorUV)
		reduceWebGPUProjection(&sector.CeilingUV)

		for edge := range sector.Walls {
			wall := &sector.Walls[edge]
			reduceWebGPUProjection(&wall.UV)

			for region := range wall.FrameRegions {
				reduceWebGPUProjection(&wall.FrameRegions[region].UV)
			}
		}
	}

	if document.StaticSolids != nil {
		for index := range document.StaticSolids.Items {
			reduceWebGPUSolid(&document.StaticSolids.Items[index])
		}
	}

	if document.SolidRenderers != nil {
		for index := range document.SolidRenderers.Prefabs {
			for solid := range document.SolidRenderers.Prefabs[index].Solids {
				reduceWebGPUSolid(&document.SolidRenderers.Prefabs[index].Solids[solid])
			}
		}
	}
}

func reduceWebGPUSolid(solid *world.Solid) {
	reduceWebGPUProjection(&solid.SideUV)
	reduceWebGPUProjection(&solid.TopUV)
	reduceWebGPUProjection(&solid.BottomUV)
}

// WebGPU V1 renders one authored material layer. Keep ordinary primary tiles
// and their affine repeat UVs; omitted secondary modulation must never force
// an entire room into a finite-resolution composite chart. The compiled
// document is owned by this build; authoring inputs and legacy builds are intact.
func prepareWebGPUSingleLayer(document *world.Document) {
	reduceWebGPUProjections(document)

	frames := false

	for index := range document.Sectors {
		sector := &document.Sectors[index]
		sector.FloorSecondary = nil
		sector.CeilingSecondary = nil

		for edge := range sector.Walls {
			sector.Walls[edge].Secondary = nil
			frames = frames || len(sector.Walls[edge].FrameRegions) != 0
		}
	}

	if !frames {
		document.MaterialLayers = nil
	}
}

// Normal production builds omit these authored layers before reaching the
// private baker. Reject bypasses rather than silently resurrecting compositing.
func validateWebGPUSingleLayer(document *world.Document) error {
	for index, sector := range document.Sectors {
		if sector.FloorSecondary != nil || sector.CeilingSecondary != nil {
			return fmt.Errorf("WebGPU V1 sector %d has unsupported secondary material layers: %w", index, ErrManifest)
		}

		for edge, wall := range sector.Walls {
			if wall.Secondary != nil {
				return fmt.Errorf("WebGPU V1 sector %d wall %d has unsupported secondary material layers: %w", index, edge, ErrManifest)
			}
		}
	}

	return nil
}
