package ssbump

import "math"

const frameOrthonormalEpsilon = 1e-8

// Reproject transfers authored coefficients between tangent frames sharing the
// same surface normal. Axes are destination XY coordinates of source X and Y;
// they must be orthonormal and may include a mirrored UV axis. Each source lobe
// distributes its energy across destination lobes; occlusion is never removed
// by normalizing the resulting coefficient vector.
func Reproject(weights [3]float64, axes [2][2]float64) ([3]float64, error) {
	for _, w := range weights {
		if math.IsNaN(w) || math.IsInf(w, 0) || w < 0 {
			return [3]float64{}, ErrInput
		}
	}

	for _, axis := range axes {
		if math.IsNaN(axis[0]) || math.IsNaN(axis[1]) || math.IsInf(axis[0], 0) || math.IsInf(axis[1], 0) ||
			math.Abs(axis[0]*axis[0]+axis[1]*axis[1]-1) > frameOrthonormalEpsilon {
			return [3]float64{}, ErrInput
		}
	}

	if math.Abs(axes[0][0]*axes[1][0]+axes[0][1]*axes[1][1]) > frameOrthonormalEpsilon {
		return [3]float64{}, ErrInput
	}

	var result [3]float64

	for source, b := range rnmBasis() {
		rotated := vector{axes[0][0]*b.x + axes[1][0]*b.y, axes[0][1]*b.x + axes[1][1]*b.y, b.z}

		var (
			projection [3]float64
			total      float64
		)

		for destination, d := range rnmBasis() {
			p := max(0, dot(rotated, d))
			projection[destination] = p * p
			total += p * p
		}

		for destination := range result {
			result[destination] += weights[source] * projection[destination] / total
		}
	}

	return result, nil
}
