package village

import (
	"math"
	"testing"
)

func TestEngineNavigationGeometryFitsPublishedBounds(t *testing.T) {
	geometry, err := NavigationMap()
	if err != nil {
		t.Fatal(err)
	}
	if geometry.Min != (Point{1016, 624}) || geometry.Max != (Point{2320, 1920}) || len(geometry.Obstacles) < 50 ||
		len(geometry.Obstacles) > 128 {
		t.Fatal("missing or unbounded authored navigation geometry", geometry.Min, geometry.Max, len(geometry.Obstacles))
	}
	for _, obstacle := range geometry.Obstacles {
		if len(obstacle) < 3 || len(obstacle) > 16 || navigationArea(obstacle) <= 0 {
			t.Fatal("invalid host obstacle", obstacle)
		}
		for _, point := range obstacle {
			if point.X < geometry.Min.X || point.Y < geometry.Min.Y || point.X > geometry.Max.X || point.Y > geometry.Max.Y {
				t.Fatal("unclipped obstacle", point)
			}
		}
	}
	t.Logf("engine navigation: %d obstacles, %.0f cells", len(geometry.Obstacles),
		math.Ceil((geometry.Max.X-geometry.Min.X)/NavigationCell)*math.Ceil((geometry.Max.Y-geometry.Min.Y)/NavigationCell))
}

func TestNavigationPartitionsLargeConcaveOutlineWithoutLosingArea(t *testing.T) {
	for _, region := range Colliders {
		if len(region.Polygon) <= 16 {
			continue
		}
		polygon := append([]Point(nil), region.Polygon...)
		want := math.Abs(navigationArea(polygon))
		pieces, err := splitNavigationPolygon(polygon)
		if err != nil {
			t.Fatal(region.Name, err)
		}
		area := 0.0
		for _, piece := range pieces {
			if len(piece) > 16 {
				t.Fatal("oversized partition", region.Name)
			}
			area += navigationArea(piece)
		}
		if math.Abs(area-want) > 1e-6 {
			t.Fatal("partition changed authored obstacle coverage", region.Name, area, want)
		}
	}
}

func TestNavigationRequestCopiesOnlyCurrentCollisionSafeResult(t *testing.T) {
	var request NavigationRequest
	points := []Point{{Spawn.X + 2, Spawn.Y}}
	request.Begin(1, 3, Spawn, points[0])
	request.Begin(2, 3, Spawn, points[0]) // A second click supersedes the first.
	if got := request.Accept(1, 3, Spawn, points); got != nil || request.Pending != 2 {
		t.Fatal("accepted superseded click", got, request)
	}
	if got := request.Accept(2, 4, Spawn, points); got != nil || request.Pending != 2 {
		t.Fatal("accepted another level's result", got, request)
	}
	accepted := request.Accept(2, 3, Spawn, points)
	if len(accepted) != 1 || accepted[0] != points[0] || request.Pending != 0 {
		t.Fatal("current clear route was not retained", accepted, request)
	}
	points[0] = Point{}
	if accepted[0] == points[0] {
		t.Fatal("route retained borrowed host storage")
	}
	request.Begin(3, 3, Spawn, accepted[0])
	request.Cancel() // Keyboard, focus loss and reset cancel pending replies.
	if got := request.Accept(3, 3, Spawn, accepted); got != nil {
		t.Fatal("manual input allowed a late route to resume", got)
	}
	request.Begin(4, 3, Spawn, Point{2112, 1376})
	if got := request.Accept(4, 3, Spawn, []Point{{2112, 1376}}); got != nil || request.Pending != 0 {
		t.Fatal("blocked water target retained a route", got, request)
	}
	if ClearRoute(Point{math.NaN(), Spawn.Y}, accepted) || ClearRoute(Spawn, nil) {
		t.Fatal("invalid start or failed host search accepted")
	}
}

func navigationArea(points []Point) float64 {
	area := 0.0
	for index, point := range points {
		next := points[(index+1)%len(points)]
		area += point.X*next.Y - next.X*point.Y
	}
	return area / 2
}

func TestNavigationConnectsInteriorWallContactsWithoutChangingTheGoal(t *testing.T) {
	from, goal := Point{1840, 1226}, Point{1936, 1226}
	start, startOK := NavigationEndpoint(from)
	end, endOK := NavigationEndpoint(goal)
	if !startOK || !endOK || start == from || end == goal ||
		math.Hypot(start.X-from.X, start.Y-from.Y) > 8 || math.Hypot(end.X-goal.X, end.Y-goal.Y) > 8 {
		t.Fatal("physical wall contact did not get a short safe connector", from, start, goal, end)
	}
	var request NavigationRequest
	request.Begin(1, 3, start, goal)
	route := request.Accept(1, 3, from, []Point{end})
	if len(route) != 3 || route[0] != start || route[len(route)-1] != goal {
		t.Fatal("connector teleported or discarded the exact clicked goal", route)
	}
	for _, speed := range []float64{WalkSpeed(), RunSpeed()} {
		followEngineRoute(t, "wall connectors", from, route, speed)
	}
	boundary := Point{2305.8, 1286.1904}
	if point, ok := NavigationEndpoint(boundary); !ok || point != boundary {
		t.Fatal("outer boundary contact unnecessarily moved", point, ok)
	}
	if _, ok := NavigationEndpoint(Point{2112, 1376}); ok {
		t.Fatal("water click was projected onto dry ground")
	}
	t.Logf("interior wall connectors: start %+v -> %+v, goal %+v -> %+v (exact goal retained)", from, start, end, goal)
}
