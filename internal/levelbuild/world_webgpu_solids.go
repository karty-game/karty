package levelbuild

import (
	"context"
	"fmt"

	"github.com/karty-game/karty-sdk/format/world"
)

func visitWebGPUSolids(document *world.Document, visit func(*world.Solid) error) error {
	if document.StaticSolids != nil {
		for i := range document.StaticSolids.Items {
			if err := visit(&document.StaticSolids.Items[i]); err != nil {
				return err
			}
		}
	}

	if document.SolidRenderers != nil {
		for i := range document.SolidRenderers.Prefabs {
			for j := range document.SolidRenderers.Prefabs[i].Solids {
				if err := visit(&document.SolidRenderers.Prefabs[i].Solids[j]); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func webGPUSolidsNeedCharts(document *world.Document) bool {
	needed := false
	_ = visitWebGPUSolids(document, func(solid *world.Solid) error {
		needed = needed || webGPUMixedMapping(solid.TopUV) || webGPUMixedMapping(solid.BottomUV) ||
			webGPUMixedMapping(solid.SideUV)

		return nil
	})

	return needed
}

func (b *webGPUChartBaker) convertSolids(ctx context.Context) error {
	return visitWebGPUSolids(b.document, func(solid *world.Solid) error {
		// One side mapping serves several differently oriented polygon edges.
		// Current authored solids permit planar side UVs only; never silently
		// discard a genuinely blended imported mapping on this shared surface.
		if webGPUMixedMapping(solid.SideUV) {
			return fmt.Errorf("solid %q blended side UVs need per-edge geometry charts: %w", solid.ID, ErrManifest)
		}

		for face := range 2 {
			plane, material, mapping := solid.Bottom, &solid.BottomMaterial, &solid.BottomUV

			normal := chartUnit(chartVector{plane.A, plane.B, -1})
			if face == 1 {
				plane, material, mapping = solid.Top, &solid.TopMaterial, &solid.TopUV
				normal = chartUnit(chartVector{-plane.A, -plane.B, 1})
			}

			if !webGPUMixedMapping(*mapping) {
				continue
			}

			points := make([]chartVector, 0, len(solid.Footprint))
			for _, p := range solid.Footprint {
				points = append(points, chartVector{p.X, p.Y, plane.A*p.X + plane.B*p.Y + plane.C})
			}

			id, uv, err := b.bake(ctx, webGPUChart{primary: *material, mapping: *mapping, points: points, normal: normal})
			if err != nil {
				return fmt.Errorf("solid %q face %d: %w", solid.ID, face, err)
			}

			*material, *mapping = id, uv
		}

		return nil
	})
}
