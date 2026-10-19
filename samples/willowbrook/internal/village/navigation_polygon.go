package village

import (
	"fmt"
	"math"
)

func navigationCross(a, b, c Point) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

func clipNavigationPolygon(polygon []Point, min, max Point) []Point {
	for _, edge := range [][3]float64{{1, 0, min.X}, {-1, 0, -max.X}, {0, 1, min.Y}, {0, -1, -max.Y}} {
		var output []Point
		for index, end := range polygon {
			start := polygon[(index+len(polygon)-1)%len(polygon)]
			a, b := start.X*edge[0]+start.Y*edge[1]-edge[2], end.X*edge[0]+end.Y*edge[1]-edge[2]
			if (a >= 0) != (b >= 0) {
				u := a / (a - b)
				intersection := Point{start.X + (end.X-start.X)*u, start.Y + (end.Y-start.Y)*u}
				if edge[0] != 0 {
					intersection.X = edge[2] / edge[0]
				} else {
					intersection.Y = edge[2] / edge[1]
				}
				output = append(output, intersection)
			}
			if b >= 0 {
				output = append(output, end)
			}
		}
		polygon = output
	}
	for changed := true; changed && len(polygon) >= 3; {
		changed = false
		for index, point := range polygon {
			if math.Abs(navigationCross(polygon[(index+len(polygon)-1)%len(polygon)], point, polygon[(index+1)%len(polygon)])) < 1e-8 {
				polygon = append(polygon[:index], polygon[index+1:]...)
				changed = true
				break
			}
		}
	}
	return polygon
}

func splitNavigationPolygon(polygon []Point) ([][]Point, error) {
	area := 0.0
	for index, point := range polygon {
		next := polygon[(index+1)%len(polygon)]
		area += point.X*next.Y - next.X*point.Y
	}
	if len(polygon) < 3 || math.Abs(area) < 1e-8 {
		return nil, nil
	}
	if area < 0 {
		for left, right := 0, len(polygon)-1; left < right; left, right = left+1, right-1 {
			polygon[left], polygon[right] = polygon[right], polygon[left]
		}
	}
	var pieces [][]Point
	for len(polygon) > 16 {
		found := false
		for index, b := range polygon {
			a, c := polygon[(index+len(polygon)-1)%len(polygon)], polygon[(index+1)%len(polygon)]
			if !navigationEar(polygon, a, b, c) {
				continue
			}
			pieces = append(pieces, []Point{a, b, c})
			polygon = append(polygon[:index], polygon[index+1:]...)
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("cannot partition authored polygon")
		}
	}
	return mergeNavigationPieces(append(pieces, polygon)), nil
}

func mergeNavigationPieces(pieces [][]Point) [][]Point {
	for changed := true; changed; {
		changed = false
		for a := 0; a < len(pieces) && !changed; a++ {
			for b := a + 1; b < len(pieces); b++ {
				merged := mergeNavigationPair(pieces[a], pieces[b])
				if merged == nil {
					continue
				}
				pieces[a] = merged
				pieces = append(pieces[:b], pieces[b+1:]...)
				changed = true
				break
			}
		}
	}
	return pieces
}

func mergeNavigationPair(a, b []Point) []Point {
	if len(a)+len(b)-2 > 16 {
		return nil
	}
	for i, start := range a {
		for j, end := range b {
			if start != b[(j+1)%len(b)] || a[(i+1)%len(a)] != end {
				continue
			}
			merged := make([]Point, 0, len(a)+len(b)-2)
			for offset := range a {
				merged = append(merged, a[(i+1+offset)%len(a)])
			}
			for offset := 1; offset < len(b)-1; offset++ {
				merged = append(merged, b[(j+1+offset)%len(b)])
			}
			return merged
		}
	}
	return nil
}

func navigationEar(polygon []Point, a, b, c Point) bool {
	if navigationCross(a, b, c) <= 0 {
		return false
	}
	for _, point := range polygon {
		if point == a || point == b || point == c {
			continue
		}
		if navigationCross(a, b, point) >= 0 && navigationCross(b, c, point) >= 0 && navigationCross(c, a, point) >= 0 {
			return false
		}
	}
	return true
}
