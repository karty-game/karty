package dungeon

import "testing"

func TestMovementCannotTunnelOrLeaveFootprint(t *testing.T) {
	for _, test := range []struct{ from, delta Point }{
		{Point{8, 4}, Point{0, 4}},   // across a pew
		{Point{12, 1}, Point{0, -5}}, // exterior wall
		{Point{5, 9}, Point{3, -3}},  // concave corner
		{Point{2, 12}, Point{0, -4}}, // tomb
	} {
		p := Move(test.from, test.delta)
		if !Walkable(p) {
			t.Fatalf("escaped geometry: %v", p)
		}
		if Distance(p, Point{test.from.X + test.delta.X, test.from.Y + test.delta.Y}) < .1 {
			t.Fatalf("tunneled through obstacle: %v", p)
		}
	}
}

func TestProjectionAndDirections(t *testing.T) {
	focus := Point{12, 12}
	if p := ScreenToGround(640, 360, 14, focus); p != focus {
		t.Fatal(p)
	}
	up := ScreenToGround(640, 260, 14, focus)
	if up.X <= focus.X || up.Y <= focus.Y || Facing(up.X-focus.X, up.Y-focus.Y) != 4 {
		t.Fatal("screen up must face sprite north", up)
	}
	right := ScreenToGround(740, 360, 14, focus)
	if right.X <= focus.X || right.Y >= focus.Y || Facing(right.X-focus.X, right.Y-focus.Y) != 6 {
		t.Fatal("screen right must face sprite east", right)
	}
}

func TestBlockedClickProjectsToWalkableFloor(t *testing.T) {
	for _, target := range []Point{{-100, -100}, {8, 6}, {12, 24}, {100, 100}} {
		if p := ProjectGoal(target); !Walkable(p) {
			t.Fatal("projected target blocked", target, p)
		}
	}
}
func TestEndpointProjectionReusesStorage(t *testing.T) {
	if n := testing.AllocsPerRun(10, func() { ProjectGoal(Shrines[0]); RouteStart(Spawn) }); n != 0 {
		t.Fatal("projection allocations", n)
	}
}
