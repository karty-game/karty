package ssbump

import "math"

type vector struct{ x, y, z float64 }

func rnmBasis() [3]vector {
	return [3]vector{
		{0.816496580927726, 0, 0.577350269189626},
		{-0.408248290463863, 0.707106781186548, 0.577350269189626},
		{-0.408248290463863, -0.707106781186548, 0.577350269189626},
	}
}

const (
	half             = 0.5
	verticalEpsilon  = 1e-9
	heightOriginBias = 1e-4
	goldenAngle      = 2.399963229728653
)

func dot(left, right vector) float64 {
	return left.x*right.x + left.y*right.y + left.z*right.z
}

func normalized(value vector) vector {
	length := math.Sqrt(dot(value, value))

	return vector{value.x / length, value.y / length, value.z / length}
}

func cross(left, right vector) vector {
	return vector{left.y*right.z - left.z*right.y, left.z*right.x - left.x*right.z, left.x*right.y - left.y*right.x}
}

// Hemisphere uses a deterministic low-discrepancy cosine-weighted disk sample.
// Directions are rotated to each heightfield texel's local normal before tracing.
func hemisphere(count int) []vector {
	result := make([]vector, count)
	for index := range result {
		radius := math.Sqrt((float64(index) + half) / float64(count))
		angle := float64(index) * goldenAngle
		result[index] = vector{radius * math.Cos(angle), radius * math.Sin(angle), math.Sqrt(1 - radius*radius)}
	}

	return result
}

func (field heightfield) normal(column, row int) vector {
	dx := (field.at(column+1, row) - field.at(column-1, row)) * half
	dy := (field.at(column, row+1) - field.at(column, row-1)) * half

	return normalized(vector{-dx, -dy, 1})
}

func (field heightfield) weightsForNormal(column, row int, options Options, rays []vector,
	normal vector, occlusionStrength float64,
) [3]float64 {
	var (
		result [3]float64
		sum    float64
	)

	for index, direction := range rnmBasis() {
		projection := max(0, dot(normal, direction))
		result[index] = projection * projection
		sum += result[index]
	}
	// Z is positive, so at least one positive basis projection always exists.
	for index := range result {
		result[index] /= sum
	}

	if options.Depth == 0 || occlusionStrength == 0 {
		return result
	}

	tangent := normalized(vector{1, 0, -normal.x / max(normal.z, verticalEpsilon)})
	bitangent := cross(normal, tangent)

	var visible, total [3]float64

	for _, ray := range rays {
		direction := vector{
			tangent.x*ray.x + bitangent.x*ray.y + normal.x*ray.z,
			tangent.y*ray.x + bitangent.y*ray.y + normal.y*ray.z,
			tangent.z*ray.x + bitangent.z*ray.y + normal.z*ray.z,
		}

		open := field.visible(column, row, direction, options)
		for index, lobe := range rnmBasis() {
			weight := max(0, dot(direction, lobe))

			total[index] += weight
			if open {
				visible[index] += weight
			}
		}
	}

	for index := range result {
		if total[index] > 0 {
			visibility := visible[index] / total[index]
			result[index] *= max(0, 1-occlusionStrength*(1-visibility))
		}
	}

	return result
}

func (field heightfield) visible(column, row int, direction vector, options Options) bool {
	horizontal := math.Hypot(direction.x, direction.y)
	if horizontal < verticalEpsilon {
		return direction.z > 0
	}

	stepX, stepY, slope := direction.x/horizontal, direction.y/horizontal, direction.z/horizontal
	// The bias is measured in heightfield texels, independent of height range.
	origin := field.at(column, row) + heightOriginBias

	for step := 1; step <= options.Steps; step++ {
		distance := options.Radius * float64(step) / float64(options.Steps)
		if field.sample(float64(column)+stepX*distance, float64(row)+stepY*distance) > origin+slope*distance {
			return false
		}
	}

	return true
}
