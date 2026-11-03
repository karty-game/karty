package levelbuild

import (
	"fmt"
	"image"
	"math"

	"github.com/karty-game/karty-sdk/format/world"
)

type chartFrame struct {
	projection                world.UVProjection
	origin, u, v              chartVector
	tangent, bitangent        chartVector
	lowU, lowV, width, height float64
}

type chartSample struct {
	color, normal     chartVector
	alpha, height, ao float64
	reflectance       [2]float64
}

func chartDot(a, b chartVector) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func chartCross(a, b chartVector) chartVector {
	return chartVector{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
func chartScale(a chartVector, scale float64) chartVector {
	return chartVector{a[0] * scale, a[1] * scale, a[2] * scale}
}
func chartAdd(a, b chartVector) chartVector {
	return chartVector{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}
func chartUnit(vector chartVector) chartVector {
	length := math.Sqrt(chartDot(vector, vector))
	if length < webGPUChartEpsilon {
		return chartVector{0, 0, 1}
	}

	return chartScale(vector, 1/length)
}
func chartPlane(plane world.UVPlane, point chartVector) float64 {
	return plane.X*point[0] + plane.Y*point[1] + plane.Z*point[2] + plane.Offset
}
func chartAxes(projection world.UVProjection, normal chartVector) (chartVector, chartVector, bool) {
	axisU, axisV := chartVector{projection.U.X, projection.U.Y, projection.U.Z}, chartVector{projection.V.X, projection.V.Y, projection.V.Z}

	determinant := chartDot(axisU, chartCross(axisV, normal))
	if math.Abs(determinant) < webGPUChartEpsilon {
		return chartVector{}, chartVector{}, false
	}

	return chartScale(chartCross(axisV, normal), 1/determinant), chartScale(chartCross(normal, axisU), 1/determinant), true
}
func (chart webGPUChart) frame() (chartFrame, error) {
	if chart.mapping == nil || len(chart.points) < 3 {
		return chartFrame{}, fmt.Errorf("incomplete material chart: %w", ErrSource)
	}

	selected := -1

	for index, weight := range chart.mapping.Weights {
		if weight > 0 {
			if _, _, valid := chartAxes(
				chart.mapping.Projections[index],
				chart.normal,
			); valid &&
				(selected < 0 || weight > chart.mapping.Weights[selected]) {
				selected = index
			}
		}
	}

	if selected < 0 {
		return chartFrame{}, fmt.Errorf("degenerate material chart mapping: %w", ErrSource)
	}

	projection := chart.mapping.Projections[selected]
	axisU, axisV, _ := chartAxes(projection, chart.normal)
	frame := chartFrame{projection: projection, u: axisU, v: axisV, lowU: math.Inf(1), lowV: math.Inf(1)}
	highU, highV := math.Inf(-1), math.Inf(-1)

	for _, point := range chart.points {
		pu, pv := chartPlane(projection.U, point), chartPlane(projection.V, point)
		frame.lowU = min(frame.lowU, pu)
		frame.lowV = min(frame.lowV, pv)
		highU = max(highU, pu)
		highV = max(highV, pv)
	}
	// Integer-repeat bounds keep the same phase for neighboring decomposed
	// pieces and make identical chart pixels share one material-cell identity.
	frame.lowU, frame.lowV = math.Floor(frame.lowU), math.Floor(frame.lowV)
	frame.width, frame.height = max(1, math.Ceil(highU)-frame.lowU), max(1, math.Ceil(highV)-frame.lowV)
	point := chart.points[0]
	frame.origin = chartAdd(
		point,
		chartAdd(
			chartScale(axisU, frame.lowU-chartPlane(projection.U, point)),
			chartScale(axisV, frame.lowV-chartPlane(projection.V, point)),
		),
	)
	frame.tangent = chartUnit(axisU)
	frame.bitangent = chartUnit(chartAdd(axisV, chartScale(frame.tangent, -chartDot(axisV, frame.tangent))))

	return frame, nil
}
func (frame chartFrame) point(u, v float64) chartVector {
	return chartAdd(frame.origin, chartAdd(chartScale(frame.u, u*frame.width), chartScale(frame.v, v*frame.height)))
}
func (frame chartFrame) mapping(cell, grid int) *world.SurfaceUV {
	cellSize := webGPUChartTileSize / grid
	scale := float64(cellSize-2*webGPUChartGutter-1) / webGPUChartTileSize
	projection := frame.projection
	projection.U = chartUVPlane(
		projection.U,
		scale/frame.width,
		(float64(cell%grid*cellSize+webGPUChartGutter)+webGPUChartTexelCenter)/webGPUChartTileSize-frame.lowU*scale/frame.width,
	)
	projection.V = chartUVPlane(
		projection.V,
		scale/frame.height,
		(float64(cell/grid*cellSize+webGPUChartGutter)+webGPUChartTexelCenter)/webGPUChartTileSize-frame.lowV*scale/frame.height,
	)

	return &world.SurfaceUV{Projections: []world.UVProjection{projection}, Weights: []float64{1}}
}
func chartUVPlane(plane world.UVPlane, scale, offset float64) world.UVPlane {
	return world.UVPlane{X: plane.X * scale, Y: plane.Y * scale, Z: plane.Z * scale, Offset: plane.Offset*scale + offset}
}
func chartLinear(value byte) float64 {
	x := float64(value) / webGPUChartByteMax
	if x <= chartSRGBEncodedThreshold {
		return x / chartSRGBLinearSlope
	}

	return math.Pow((x+chartSRGBOffset)/chartSRGBScale, chartSRGBExponent)
}
func chartSRGB(value float64) byte {
	value = min(1, max(0, value))
	if value <= chartSRGBLinearThreshold {
		return chartByte(value * chartSRGBLinearSlope)
	}

	return chartByte(chartSRGBScale*math.Pow(value, 1/chartSRGBExponent) - chartSRGBOffset)
}
func chartByte(value float64) byte {
	return byte(math.Round(min(1, max(0, value)) * webGPUChartByteMax))
}
func chartNormalByte(value float64) byte {
	return byte(math.Round(webGPUChartNormalMidpoint + min(1, max(-1, value))*webGPUChartNormalScale))
}

func chartTexel(pixels *image.NRGBA, rectX, rectY int, coordinates [2]float64) [4]byte {
	if pixels == nil {
		return [4]byte{0, 0, 0, webGPUChartByteMax}
	}

	x, y := int(
		math.Floor((coordinates[0]-math.Floor(coordinates[0]))*webGPUChartTileSize),
	), int(
		math.Floor((coordinates[1]-math.Floor(coordinates[1]))*webGPUChartTileSize),
	)
	pixel := pixels.NRGBAAt(rectX+x, rectY+y)

	return [4]byte{pixel.R, pixel.G, pixel.B, pixel.A}
}

func (b *webGPUChartBaker) sample(
	materialID uint32,
	mapping *world.SurfaceUV,
	point, normal chartVector,
	clampBand *world.WallFrameRegion,
) (chartSample, error) {
	rect, exists := b.materials[materialID]
	if !exists || mapping == nil {
		return chartSample{}, fmt.Errorf("material %d unavailable for chart: %w", materialID, ErrSource)
	}

	strengthNormal, strengthAO, strengthHeight := 1.0, 1.0, 1.0
	if rect.Strengths != nil {
		strengthNormal, strengthAO, strengthHeight = rect.Strengths.Normal, rect.Strengths.AO, rect.Strengths.Height
	}

	var result chartSample

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

		colorValue, data, reflectance := chartTexel(
			b.source.Albedo(),
			rect.X,
			rect.Y,
			coordinates,
		), chartTexel(
			b.source.Data(),
			rect.X,
			rect.Y,
			coordinates,
		), chartTexel(
			b.source.Reflectance(),
			rect.X,
			rect.Y,
			coordinates,
		)

		alpha := float64(colorValue[3]) / webGPUChartByteMax
		for channel := range 3 {
			result.color[channel] += chartLinear(colorValue[channel]) * alpha * weight
		}

		result.alpha += alpha * weight

		axisU, axisV, valid := chartAxes(projection, normal)
		if !valid {
			return chartSample{}, fmt.Errorf("material %d projection has no tangent basis: %w", materialID, ErrSource)
		}

		tangent := chartUnit(axisU)
		bitangent := chartUnit(chartAdd(axisV, chartScale(tangent, -chartDot(axisV, tangent))))
		normalX, normalY := min(
			1,
			max(-1, float64(int(data[0])-webGPUChartNormalMidpoint)/webGPUChartNormalScale*strengthNormal),
		), min(
			1,
			max(-1, float64(int(data[1])-webGPUChartNormalMidpoint)/webGPUChartNormalScale*strengthNormal),
		)
		mapped := chartUnit(
			chartAdd(
				chartAdd(chartScale(tangent, normalX), chartScale(bitangent, normalY)),
				chartScale(normal, math.Sqrt(max(0, 1-normalX*normalX-normalY*normalY))),
			),
		)
		result.normal = chartAdd(result.normal, chartScale(mapped, weight))
		result.height += float64(data[2]) / webGPUChartByteMax * strengthHeight * weight

		result.ao += min(1, max(0, 1-strengthAO*(1-float64(data[3])/webGPUChartByteMax))) * weight
		for channel := range 2 {
			result.reflectance[channel] += float64(reflectance[channel]) / webGPUChartByteMax * weight
		}
	}

	for channel := range 3 {
		result.color[channel] /= max(result.alpha, webGPUChartEpsilon)
	}

	result.normal = chartUnit(result.normal)

	return result, nil
}
func (b *webGPUChartBaker) sampleChart(chart webGPUChart, point chartVector) (chartSample, error) {
	result, err := b.sample(chart.primary, chart.mapping, point, chart.normal, nil)
	if err != nil {
		return result, err
	}

	if chart.band != nil {
		coordinates := chart.band.UV.Projections[0]
		u, v := chartPlane(coordinates.U, point), chartPlane(coordinates.V, point)

		inside := (chart.band.RepeatU || u >= 0 && u <= 1) && (chart.band.RepeatV || v >= 0 && v <= 1)
		if inside {
			band, err := b.sample(chart.band.Material, chart.band.UV, point, chart.normal, chart.band)
			if err != nil {
				return result, err
			}

			coverage := band.alpha
			result.color = chartAdd(chartScale(result.color, 1-coverage), chartScale(band.color, coverage))
			result.normal = chartUnit(chartAdd(chartScale(result.normal, 1-coverage), chartScale(band.normal, coverage)))
			result.alpha = result.alpha*(1-coverage) + coverage
			result.height = result.height*(1-coverage) + band.height*coverage

			result.ao = result.ao*(1-coverage) + band.ao*coverage
			for channel := range 2 {
				result.reflectance[channel] = result.reflectance[channel]*(1-coverage) + band.reflectance[channel]*coverage
			}
		}
	}

	return result, nil
}
