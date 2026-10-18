// Package dungeon shares the authored footprint and collision geometry between
// the asset author and the cartridge. No engine or private modules are needed.
package dungeon

import "math"

type Point struct{ X, Y float64 }
type Box struct{ X, Y, W, D float64 }

const Radius = .28
const Cell = .5
const Columns, Rows = 48, 50
const Cells = Columns * Rows

var Spawn = Point{12, 4}
var Shrines = [...]Point{{2.8, 12.5}, {21.2, 12.5}, {12, 22.7}}
var Outline = [...]Point{{6, 0}, {18, 0}, {18, 8}, {24, 8}, {24, 17}, {18, 17}, {18, 25}, {6, 25}, {6, 17}, {0, 17}, {0, 8}, {6, 8}}

// Every blocking decoration is authored from this same list. The narrow runner
// stays open; transept tombs require a deliberate route around their ends.
var Obstacles = [...]Box{
	{7.1, 5.5, 2.8, .75}, {14.1, 5.5, 2.8, .75},
	{7.1, 7.5, 2.8, .75}, {14.1, 7.5, 2.8, .75},
	{6.7, 9.5, .85, .85}, {16.45, 9.5, .85, .85},
	{6.7, 15.4, .85, .85}, {16.45, 15.4, .85, .85},
	{1.2, 9.5, 3.2, 1.35}, {1.2, 14.3, 3.2, 1.35},
	{19.6, 9.5, 3.2, 1.35}, {19.6, 14.3, 3.2, 1.35},
	{7, 18.5, 2.8, 1.35}, {14.2, 18.5, 2.8, 1.35},
	{10.6, 23.4, 2.8, 1.1},
}

func inside(x, y float64) bool {
	return x > 6 && x < 18 && y > 0 && y < 25 || x > 0 && x < 24 && y > 8 && y < 17
}

func Walkable(p Point) bool {
	// Include the disc envelope, including the four concave corners.
	if !inside(p.X, p.Y) {
		return false
	}
	for i, a := range Outline {
		if pointSegmentDistanceSquared(p, a, Outline[(i+1)%len(Outline)]) < Radius*Radius {
			return false
		}
	}
	for _, b := range Obstacles {
		dx := p.X - math.Max(b.X, math.Min(p.X, b.X+b.W))
		dy := p.Y - math.Max(b.Y, math.Min(p.Y, b.Y+b.D))
		if dx*dx+dy*dy < Radius*Radius {
			return false
		}
	}
	return true
}

func Distance(a, b Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// Move tests the full segment in bounded substeps and slides along obstacles.
// Keyboard and route following use this identical capsule clearance.
func Move(p, delta Point) Point {
	n := max(1, int(math.Ceil(math.Hypot(delta.X, delta.Y)/.08)))
	step := Point{delta.X / float64(n), delta.Y / float64(n)}
	for range n {
		x := Point{p.X + step.X, p.Y}
		if Walkable(x) {
			p = x
		}
		y := Point{p.X, p.Y + step.Y}
		if Walkable(y) {
			p = y
		}
	}
	return p
}

func center(i int) Point {
	return Point{(float64(i%Columns) + .5) * Cell, (float64(i/Columns) + .5) * Cell}
}

// ProjectGoal retains Vigil's forgiving blocked-click behavior. It selects an
// endpoint only; the engine owns route search and smoothing.
func ProjectGoal(p Point) Point {
	index, best := -1, math.MaxFloat64
	for i := range Cells {
		q := center(i)
		if d := (p.X-q.X)*(p.X-q.X) + (p.Y-q.Y)*(p.Y-q.Y); d < best && Walkable(q) {
			index, best = i, d
		}
	}
	if index < 0 {
		return Spawn
	}
	return center(index)
}

// RouteStart connects an off-centre accepted pose to a safe grid centre. This
// also handles a pose whose containing cell centre lies inside an obstacle.
func RouteStart(from Point) (Point, bool) {
	best := math.MaxFloat64
	start := Point{}
	found := false
	for i := range Cells {
		q := center(i)
		if d := (q.X-from.X)*(q.X-from.X) + (q.Y-from.Y)*(q.Y-from.Y); d < best && ClearSegment(from, q) {
			start, best, found = q, d, true
		}
	}
	return start, found
}

// ScreenToGround inverts the fixed yaw=45°, pitch=-30° projection at z=0.
// It is also the keyboard basis, so W always travels toward screen-up.
func ScreenToGround(x, y, height float64, focus Point) Point {
	s := height / 720
	right, forward := (x-640)*s, -(y-360)*s/.5
	return Point{focus.X + (right+forward)/math.Sqrt2, focus.Y + (-right+forward)/math.Sqrt2}
}

func Facing(dx, dy float64) int {
	// Rows are S, SW, W, NW, N, NE, E, SE in screen space.
	sx, sy := (dx-dy)/math.Sqrt2, -(dx+dy)*.5/math.Sqrt2
	a := math.Atan2(-sx, sy)
	return (int(math.Round(a/(math.Pi/4))) + 8) % 8
}
