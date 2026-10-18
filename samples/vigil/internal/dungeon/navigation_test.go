package dungeon

import (
	"math"
	"testing"
)

func TestEngineRouteStartsTowardGoalWithoutRecentering(t *testing.T) {
	var nav Navigation
	p := Spawn
	const step = MoveSpeed / 60
	for update := range 50 {
		if update%3 == 0 {
			via, ok := RouteStart(p)
			if !ok {
				t.Fatal("no grid connector", p)
			}
			nav.Begin(uint32(update+1), 7, via)
			if matched, usable := nav.Accept(uint32(update+1), 7, p, []Point{{12, 8}}); !matched || !usable || nav.Count != 1 {
				t.Fatal("open engine route recentered", nav.Count)
			}
		}
		next := nav.Step(p, step)
		if next.Y <= p.Y || math.Abs(Distance(p, next)-step) > 1e-9 {
			t.Fatal("open route lost movement budget", p, next)
		}
		p = next
	}
}
func TestFollowerCarriesBudgetAcrossShortSegmentsAndTurns(t *testing.T) {
	for _, test := range []struct {
		path [3]Point
		want Point
	}{
		{[3]Point{{12, 4.01}, {12, 4.02}, {12, 4.1}}, Point{12, 4.075}},
		{[3]Point{{12.01, 4}, {12.01, 4.01}, {12.01, 4.1}}, Point{12.01, 4.065}},
	} {
		nav := Navigation{Count: len(test.path)}
		copy(nav.Path[:], test.path[:])
		p := nav.Step(Spawn, .075)
		if Distance(p, test.want) > 1e-9 || nav.Next != 2 {
			t.Fatal("follower discarded movement budget", p, test.want, nav.Next)
		}
	}
}
func TestFollowerAcceptsSafeDetoursAndPreservesSweptClearance(t *testing.T) {
	for _, test := range []struct {
		from Point
		path []Point
	}{
		{Point{8.75, 4.75}, []Point{{10.25, 4.75}, {10.25, 6.75}, {8.75, 6.75}}},
		{Point{5.5, 8.5}, []Point{{6.5, 8.5}, {6.5, 7}}},
		{Spawn, []Point{{12, 8.75}, {2.25, 8.75}}},
	} {
		var nav Navigation
		nav.Begin(1, 7, test.from)
		if matched, usable := nav.Accept(1, 7, test.from, test.path); !matched || !usable {
			t.Fatal("rejected safe engine detour", test)
		}
		p := test.from
		for updates := 0; nav.Next < nav.Count; updates++ {
			if updates >= Cells {
				t.Fatal("follower stalled", p)
			}
			next := nav.Step(p, .075)
			if !Walkable(next) || !ClearSegment(p, next) {
				t.Fatal("combined movement cut a corner", p, next)
			}
			p = next
		}
		if p != test.path[len(test.path)-1] {
			t.Fatal("lost exact destination", p)
		}
	}
}
func TestSweptCapsuleRejectsCornersAndFurniture(t *testing.T) {
	if Walkable(Point{6.19, 8.205}) || !Walkable(Point{6.2, 8.21}) {
		t.Fatal("walking and follower corner clearance differ")
	}
	for _, test := range []struct {
		from, to Point
		want     bool
	}{
		{Spawn, Point{12, 8}, true}, {Point{8, 4}, Point{8, 7}, false},
		{Point{5.5, 8.5}, Point{6.5, 7}, false}, {Point{5.5, 8.5}, Point{6.5, 8.5}, true},
	} {
		if got := ClearSegment(test.from, test.to); got != test.want {
			t.Fatal(test, got)
		}
	}
}
func TestNavigationRejectsStaleFailedAndCancelledResults(t *testing.T) {
	var nav Navigation
	nav.Begin(1, 7, Spawn)
	nav.Begin(2, 7, Spawn)
	if matched, _ := nav.Accept(1, 7, Spawn, []Point{{12, 8}}); matched || nav.Pending != 2 {
		t.Fatal("superseded reply accepted")
	}
	if matched, _ := nav.Accept(2, 8, Spawn, []Point{{12, 8}}); matched || nav.Pending != 2 {
		t.Fatal("wrong mount reply accepted")
	}
	points := []Point{{12, 8}}
	if matched, usable := nav.Accept(2, 7, Spawn, points); !matched || !usable {
		t.Fatal("current reply rejected")
	}
	points[0] = Point{}
	if nav.Path[0] != (Point{12, 8}) {
		t.Fatal("retained borrowed SDK storage")
	}
	nav.Begin(3, 7, Spawn)
	if matched, usable := nav.Accept(3, 7, Spawn, nil); !matched || usable || nav.Count != 0 || nav.Pending != 0 {
		t.Fatal("failure retained a route")
	}
	nav.Begin(4, 7, Spawn)
	nav.Cancel()
	if matched, _ := nav.Accept(4, 7, Spawn, []Point{{12, 8}}); matched || nav.Count != 0 {
		t.Fatal("keyboard/reset cancellation revived route")
	}
	nav.Begin(5, 7, Point{8.75, 4.75})
	if matched, usable := nav.Accept(5, 7, Point{8.75, 4.75}, []Point{{8.75, 6.75}}); !matched || usable {
		t.Fatal("unsafe reply entered furniture")
	}
}
func TestFollowerReusesStorage(t *testing.T) {
	var nav Navigation
	if n := testing.AllocsPerRun(10, func() {
		nav.Begin(1, 7, Spawn)
		nav.Accept(1, 7, Spawn, []Point{{12, 8}})
		p := Spawn
		for nav.Next < nav.Count {
			p = nav.Step(p, .075)
		}
	}); n != 0 {
		t.Fatal("follower allocations", n)
	}
}
