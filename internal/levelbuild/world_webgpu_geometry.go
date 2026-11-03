package levelbuild

import (
	"context"
	"fmt"

	"github.com/karty-game/karty-sdk/format/world"
)

func needsWebGPUCharts(document *world.Document) bool {
	for _, sector := range document.Sectors {
		if webGPUMixedMapping(sector.FloorUV) ||
			webGPUMixedMapping(sector.CeilingUV) {
			return true
		}

		for _, wall := range sector.Walls {
			if webGPUMixedMapping(wall.UV) {
				return true
			}

			for _, region := range wall.FrameRegions {
				if region.Coverage == world.FrameCoverageMasked || region.UV != nil && len(region.UV.Projections) > 1 {
					return true
				}
			}
		}
	}

	return webGPUSolidsNeedCharts(document)
}

func webGPUMixedMapping(mapping *world.SurfaceUV) bool {
	return mapping != nil && len(mapping.Projections) > 1
}

func (b *webGPUChartBaker) convert(ctx context.Context) error {
	if err := validateWebGPUSingleLayer(b.document); err != nil {
		return err
	}

	for index := range b.document.Sectors {
		sector := &b.document.Sectors[index]

		floor, ceiling := horizontalChartPoints(*sector, sector.Floor), horizontalChartPoints(*sector, sector.Ceiling)
		if webGPUMixedMapping(sector.FloorUV) {
			materialID, mapping, err := b.bake(
				ctx,
				webGPUChart{primary: sector.FloorMaterial, mapping: sector.FloorUV,
					points: floor, normal: chartUnit(chartVector{-sector.Floor.A, -sector.Floor.B, 1})},
			)
			if err != nil {
				return fmt.Errorf("sector %d floor: %w", index, err)
			}

			sector.FloorMaterial, sector.FloorUV = materialID, mapping
		}

		if webGPUMixedMapping(sector.CeilingUV) {
			materialID, mapping, err := b.bake(ctx,
				webGPUChart{primary: sector.CeilingMaterial, mapping: sector.CeilingUV,
					points: ceiling, normal: chartUnit(chartVector{sector.Ceiling.A, sector.Ceiling.B, -1})},
			)
			if err != nil {
				return fmt.Errorf("sector %d ceiling: %w", index, err)
			}

			sector.CeilingMaterial, sector.CeilingUV = materialID, mapping
		}

		for edge := range sector.Walls {
			if err := b.convertWall(ctx, sector, edge); err != nil {
				return fmt.Errorf("sector %d wall %d: %w", index, edge, err)
			}
		}
	}

	if err := b.convertSolids(ctx); err != nil {
		return err
	}

	return world.Validate(b.document)
}

func horizontalChartPoints(sector world.Sector, plane world.Plane) []chartVector {
	points := make([]chartVector, 0, len(sector.Walls))
	for _, wall := range sector.Walls {
		points = append(points, chartVector{wall.Start.X, wall.Start.Y, plane.A*wall.Start.X + plane.B*wall.Start.Y + plane.C})
	}

	return points
}

func (b *webGPUChartBaker) convertWall(ctx context.Context, sector *world.Sector, edge int) error {
	wall := &sector.Walls[edge]
	original := *wall
	normal := chartUnit(chartVector{wall.Start.Y - wall.End.Y, wall.End.X - wall.Start.X, 0})

	points := make([]chartVector, 0, 4)
	for _, point := range []world.Vec2{wall.Start, wall.End} {
		points = append(points, chartVector{point.X, point.Y, sector.Floor.A*point.X + sector.Floor.B*point.Y + sector.Floor.C},
			chartVector{point.X, point.Y, sector.Ceiling.A*point.X + sector.Ceiling.B*point.Y + sector.Ceiling.C})
	}

	if webGPUMixedMapping(wall.UV) {
		materialID, mapping, err := b.bake(ctx,
			webGPUChart{primary: wall.Material, mapping: wall.UV, points: points, normal: normal},
		)
		if err != nil {
			return err
		}

		wall.Material, wall.UV = materialID, mapping
	}

	for index := range wall.FrameRegions {
		region := &wall.FrameRegions[index]
		if region.Coverage == world.FrameCoverageMain {
			region.Material = wall.Material

			continue
		}

		if region.Coverage != world.FrameCoverageMasked && (region.UV == nil || len(region.UV.Projections) <= 1) {
			continue
		}

		regionPoints := make([]chartVector, 0, len(region.Vertices))
		for _, point := range region.Vertices {
			regionPoints = append(regionPoints, chartVector{wall.Start.X + (wall.End.X-wall.Start.X)*point.X,
				wall.Start.Y + (wall.End.Y-wall.Start.Y)*point.X, point.Y})
		}

		chart := webGPUChart{primary: original.Material, mapping: original.UV,
			points: regionPoints, normal: normal, band: region}
		if region.Coverage == world.FrameCoverageOpaque {
			chart.primary, chart.mapping, chart.band = region.Material, region.UV, nil
		}

		materialID, mapping, err := b.bake(ctx, chart)
		if err != nil {
			return err
		}

		region.Material, region.UV, region.Coverage = materialID, mapping, world.FrameCoverageOpaque
		region.RepeatU, region.RepeatV = false, false
	}

	return nil
}
