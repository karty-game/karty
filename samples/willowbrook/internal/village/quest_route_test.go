package village

import (
	"math"
	"testing"
)

func TestQuestRoutesFollowActualVillageAtWalkingAndRunningSpeed(t *testing.T) {
	for _, speed := range []float64{WalkSpeed() / 60, RunSpeed() / 60} {
		p := Spawn
		up, down, bridge := false, false, false
		for _, target := range QuestStations {
			end, visited := followQuestRoute(t, p, target, speed)
			for j, q := range visited {
				bridge = bridge || InRegion("bridge", q)
				if j == 0 {
					continue
				}
				if InRegion("stair", q) || InRegion("stair", visited[j-1]) {
					up = up || q.Y < visited[j-1].Y
					down = down || q.Y > visited[j-1].Y
				}
			}

			p = end
		}
		if !up || !down || !bridge {
			t.Fatalf("quest skipped a required landmark at speed %g: ascent=%v descent=%v bridge=%v", speed, up, down, bridge)
		}
		// A player may discover/select a visible station without having visited
		// the previous note. Its route must still handle the actual obstacles.
		for _, target := range QuestStations {
			followQuestRoute(t, Spawn, target, speed)
		}
	}
}

func followQuestRoute(t *testing.T, from, target Point, speed float64) (Point, []Point) {
	t.Helper()
	route := QuestRoute(from, target)
	const maxNodes = 32767
	if len(route) == 0 || len(route) > maxNodes {
		t.Fatalf("invalid bounded route from %+v to %+v: %d nodes", from, target, len(route))
	}
	for _, p := range route {
		if !Walkable(p) {
			t.Fatalf("route includes river, collider or map edge: %+v", p)
		}
	}
	p := from
	visited := []Point{p}
	for range 12000 {
		// These are the runtime arrival and waypoint radii. Follow at the
		// real per-frame speed rather than teleporting between valid nodes.
		if CanInteract(p, target) {
			return p, visited
		}
		for len(route) > 0 && math.Hypot(route[0].X-p.X, route[0].Y-p.Y) < 4 {
			route = route[1:]
		}
		if len(route) == 0 {
			t.Fatalf("route ended outside interaction range: %+v, target %+v", p, target)
		}
		dx, dy := route[0].X-p.X, route[0].Y-p.Y
		length := math.Hypot(dx, dy)
		next := Move(p, Point{X: dx / length * speed, Y: dy / length * speed})
		if !Walkable(next) || math.Hypot(next.X-p.X, next.Y-p.Y) < .00001 {
			t.Fatalf("per-frame route blocked at %+v toward %+v, target %+v", p, route[0], target)
		}
		p = next
		visited = append(visited, p)
	}
	t.Fatalf("route exceeded frame bound from %+v toward %+v", from, target)
	return p, visited
}

func TestQuestRouteRejectsInvalidAndUnreachableTargetsWithinSearchBound(t *testing.T) {
	river := Point{}
	for _, r := range Colliders {
		if r.Name == "river-north" {
			river = Point{r.X + r.Width/2, r.Y + r.Height/2}
		}
	}
	for _, test := range []struct{ from, to Point }{
		{Point{-20, -20}, QuestStations[0]},
		{river, QuestStations[0]}, // River start.
		{Spawn, Point{Width() + 80, Height() + 80}},
		{Spawn, river}, // No dry approach at the river's elevation.
	} {
		if route := QuestRoute(test.from, test.to); len(route) != 0 {
			t.Fatalf("unexpected route from %+v to %+v: %d nodes", test.from, test.to, len(route))
		}
	}
}
