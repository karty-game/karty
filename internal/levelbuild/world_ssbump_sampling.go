package levelbuild

import (
	"fmt"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/ssbump"
)

// Return premultiplied coefficient energy and coverage. UV projection blending
// and masked bands preserve directional occlusion in the destination frame.
func (b *webGPUChartBaker) sampleSSBump(materialID uint32, mapping *world.SurfaceUV, point, normal chartVector,
	frame chartFrame, clampBand *world.WallFrameRegion,
) (chartVector, float64, error) {
	var (
		result   chartVector
		coverage float64
	)

	rect, ok := b.materials[materialID]
	if !ok || mapping == nil || b.ssbump.tiles[materialID] == nil {
		return result, 0, fmt.Errorf("SSBump material %d unavailable: %w", materialID, ErrSource)
	}

	for index, weight := range mapping.Weights {
		if weight <= 0 {
			continue
		}

		projection := mapping.Projections[index]
		coordinates := [2]float64{chartPlane(projection.U, point), chartPlane(projection.V, point)}

		if clampBand != nil {
			if !clampBand.RepeatU {
				coordinates[0] = min(1-.5/webGPUChartTileSize, max(.5/webGPUChartTileSize, coordinates[0]))
			}

			if !clampBand.RepeatV {
				coordinates[1] = min(1-.5/webGPUChartTileSize, max(.5/webGPUChartTileSize, coordinates[1]))
			}
		}

		coeff := chartTexel(b.ssbump.tiles[materialID], 0, 0, coordinates)
		albedo := chartTexel(b.source.Albedo(), rect.X, rect.Y, coordinates)
		alpha := float64(albedo[3]) / webGPUChartByteMax

		axisU, axisV, valid := chartAxes(projection, normal)
		if !valid {
			return result, 0, fmt.Errorf("SSBump material %d has degenerate tangent frame: %w", materialID, ErrSource)
		}

		tangent := chartUnit(axisU)
		bitangent := chartUnit(chartAdd(axisV, chartScale(tangent, -chartDot(axisV, tangent))))
		axes := [2][2]float64{{chartDot(tangent, frame.tangent), chartDot(tangent, frame.bitangent)},
			{chartDot(bitangent, frame.tangent), chartDot(bitangent, frame.bitangent)}}

		rotated, err := ssbump.Reproject(
			[3]float64{
				float64(coeff[0]) / webGPUChartByteMax,
				float64(coeff[1]) / webGPUChartByteMax,
				float64(coeff[2]) / webGPUChartByteMax,
			},
			axes,
		)
		if err != nil {
			return result, 0, err
		}

		for channel := range result {
			result[channel] += rotated[channel] * weight * alpha
		}

		coverage += alpha * weight
	}

	return result, coverage, nil
}

func (b *webGPUChartBaker) sampleChartSSBump(chart webGPUChart, point chartVector, frame chartFrame) (chartVector, error) {
	result, alpha, err := b.sampleSSBump(chart.primary, chart.mapping, point, chart.normal, frame, nil)
	if err != nil {
		return result, err
	}

	for channel := range result {
		result[channel] /= max(alpha, webGPUChartEpsilon)
	}

	if chart.band == nil {
		return result, nil
	}

	projection := chart.band.UV.Projections[0]
	u, v := chartPlane(projection.U, point), chartPlane(projection.V, point)

	inside := (chart.band.RepeatU || u >= 0 && u <= 1) && (chart.band.RepeatV || v >= 0 && v <= 1)
	if !inside {
		return result, nil
	}

	band, coverage, err := b.sampleSSBump(chart.band.Material, chart.band.UV, point, chart.normal, frame, chart.band)
	if err != nil {
		return result, err
	}

	return chartAdd(chartScale(result, 1-coverage), band), nil
}
