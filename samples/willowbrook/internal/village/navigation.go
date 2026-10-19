package village

import (
	"fmt"
	"math"

	"github.com/karty-game/tiled"
)

// Navigation uses the existing engine solver in meters. The 24-pixel grid fits
// the complete authored walking area within the host's 4096-cell bound.
const NavigationScale = 32.0
const NavigationCell = 24.0

type NavigationGeometry struct {
	Min, Max  Point
	Obstacles [][]Point
}

// NavigationRequest rejects replies superseded by a click, manual input or
// level change. Accepted points are copied before the host event borrow ends.
type NavigationRequest struct {
	Pending, Level uint32
	Start, Goal    Point
}

func (n *NavigationRequest) Cancel() { n.Pending, n.Level = 0, 0 }
func (n *NavigationRequest) Begin(request, level uint32, start, goal Point) {
	n.Pending, n.Level, n.Start, n.Goal = request, level, start, goal
}
func (n *NavigationRequest) Accept(request, level uint32, from Point, points []Point) []Point {
	if request == 0 || request != n.Pending || level != n.Level {
		return nil
	}
	start, goal := n.Start, n.Goal
	n.Cancel()
	if len(points) == 0 || len(points) > 128 {
		return nil
	}
	path := make([]Point, 0, len(points)+2)
	if start != from {
		path = append(path, start)
	}
	path = append(path, points...)
	if goal != path[len(path)-1] {
		path = append(path, goal)
	}
	if !ClearRoute(from, path) {
		return nil
	}
	return path
}

func AdvanceRoute(route []Point, position Point, tolerance float64) []Point {
	for len(route) > 0 && math.Hypot(route[0].X-position.X, route[0].Y-position.Y) <= tolerance {
		route = route[1:]
	}
	return route
}

func NavigationMap() (NavigationGeometry, error) {
	walkingMin, walkingMax := navigationBounds()
	// Extend the search grid outward to align centers with the narrow stair
	// landing and riverbank passage; authored borders still define walking.
	geometry := NavigationGeometry{Min: Point{walkingMin.X - 8, walkingMin.Y - 16}, Max: walkingMax}
	columns := math.Ceil((geometry.Max.X - geometry.Min.X) / NavigationCell)
	rows := math.Ceil((geometry.Max.Y - geometry.Min.Y) / NavigationCell)
	if columns < 1 || rows < 1 || columns*rows > 4096 {
		return geometry, fmt.Errorf("village navigation exceeds the engine grid bound")
	}
	for _, region := range Colliders {
		polygon := make([]Point, 0, len(region.Polygon))
		for _, point := range region.Polygon {
			polygon = append(polygon, Point{region.X + point.X, region.Y + point.Y})
		}
		if len(polygon) == 0 {
			x, y := navigationPadding(region, walkingMin, walkingMax)
			polygon = []Point{{region.X - x, region.Y - y}, {region.X + region.Width + x, region.Y - y},
				{region.X + region.Width + x, region.Y + region.Height + y}, {region.X - x, region.Y + region.Height + y}}
		}
		polygon = clipNavigationPolygon(polygon, geometry.Min, geometry.Max)
		pieces, err := splitNavigationPolygon(polygon)
		if err != nil {
			return geometry, fmt.Errorf("navigation collider %q: %w", region.Name, err)
		}
		geometry.Obstacles = append(geometry.Obstacles, pieces...)
		if len(geometry.Obstacles) > 128 {
			return geometry, fmt.Errorf("village navigation exceeds the engine obstacle bound")
		}
	}
	return geometry, nil
}

func navigationBounds() (Point, Point) {
	for _, region := range Level.Objects {
		if region.Class == "navigation" {
			return Point{region.X, region.Y}, Point{region.X + region.Width, region.Y + region.Height}
		}
	}
	return Point{}, Point{Width(), Height()}
}

func navigationPadding(region tiled.Region, lower, upper Point) (float64, float64) {
	// Border-wall corners lie outside walking. Interior padding keeps the
	// solver's rounded ellipse clear of walking's rectangular solid corners.
	if region.X+region.Width <= lower.X || region.X >= upper.X || region.Y+region.Height <= lower.Y || region.Y >= upper.Y {
		return 0, 0
	}
	return Radius*(1-1/math.Sqrt2) + .25, RadiusY*(1-1/math.Sqrt2) + .25
}

func navigationEndpointClear(point, lower, upper Point) bool {
	if !Walkable(point) {
		return false
	}
	for _, region := range Colliders {
		if len(region.Polygon) != 0 {
			continue
		}
		x, y := navigationPadding(region, lower, upper)
		nearest := Point{max(region.X-x, min(point.X, region.X+region.Width+x)), max(region.Y-y, min(point.Y, region.Y+region.Height+y))}
		dx, dy := (point.X-nearest.X)/Radius, (point.Y-nearest.Y)/RadiusY
		if dx*dx+dy*dy < 1+1e-7 {
			return false
		}
	}
	return true
}

// NavigationEndpoint connects a physical wall contact to the conservatively
// padded search surface. The short connector is walked normally, never applied
// as a teleport; the final path retains the exact clicked destination as well.
func NavigationEndpoint(point Point) (Point, bool) {
	lower, upper := navigationBounds()
	if navigationEndpointClear(point, lower, upper) {
		return point, true
	}
	if !Walkable(point) {
		return Point{}, false
	}
	best, distance := Point{}, math.Inf(1)
	for ring := 1; ring <= 16 && float64(ring)/2 <= distance; ring++ {
		for dx := -ring; dx <= ring; dx++ {
			for dy := -ring; dy <= ring; dy++ {
				if dx != -ring && dx != ring && dy != -ring && dy != ring {
					continue
				}
				candidate := Point{point.X + float64(dx)/2, point.Y + float64(dy)/2}
				length := math.Hypot(candidate.X-point.X, candidate.Y-point.Y)
				if length < distance && length <= 8 && navigationEndpointClear(candidate, lower, upper) &&
					ClearRoute(point, []Point{candidate}) {
					best, distance = candidate, length
				}
			}
		}
	}
	return best, !math.IsInf(distance, 1)
}

// ClearRoute checks borrowed host results against the game's actual movement
// rules before retaining them. Movement still applies collision every frame.
func ClearRoute(from Point, route []Point) bool {
	if !Walkable(from) {
		return false
	}
	for _, target := range route {
		if !Walkable(target) {
			return false
		}
		actual := Move(from, Point{target.X - from.X, target.Y - from.Y})
		if math.Hypot(actual.X-target.X, actual.Y-target.Y) > .001 {
			return false
		}
		steps := max(1, int(math.Ceil(math.Hypot(target.X-from.X, target.Y-from.Y)/(min(Radius, RadiusY)/3))))
		for index := 1; index < steps; index++ {
			fraction := float64(index) / float64(steps)
			if !Walkable(Point{from.X + (target.X-from.X)*fraction, from.Y + (target.Y-from.Y)*fraction}) {
				return false
			}
		}
		from = target
	}
	return len(route) > 0
}
