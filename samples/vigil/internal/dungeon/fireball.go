package dungeon

import "math"

// FacingDirection inverts the fixed dimetric projection used by Facing. Equal
// screen angles need unequal ground angles because forward is foreshortened.
func FacingDirection(facing int) Point {
	a := float64(facing%WalkDirections) * math.Pi / 4
	s, c := math.Sincos(a)
	x, y := -2*c-s, -2*c+s
	length := math.Hypot(x, y)
	return Point{x / length, y / length}
}

// FireballStep stops at the first obstruction instead of sliding or tunnelling.
// Reusing player clearance conservatively keeps the light out of solid props.
func FireballStep(from, direction Point) (Point, bool) {
	to := Point{from.X + direction.X*.1, from.Y + direction.Y*.1}
	if !ClearSegment(from, to) {
		return from, false
	}
	return to, true
}
